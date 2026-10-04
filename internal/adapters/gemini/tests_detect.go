package gemini

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrNotTestCommand         = errors.New("command is not a recognized direct test runner")
	ErrCompoundOrOperator     = errors.New("compound command, pipeline, or shell operator not supported")
	ErrUnsupportedOutputMode  = errors.New("unsupported test runner output mode")
	ErrLeadingEnvNotSupported = errors.New("leading environment variable assignments not supported")
	ErrBackgroundExecution    = errors.New("background execution cannot be classified as tests_passed")
)

// ShellToolInput represents the parameters received in AfterTool for run_shell_command.
type ShellToolInput struct {
	Command      string `json:"command"`
	IsBackground bool   `json:"is_background"`
	DirPath      string `json:"dir_path,omitempty"`
}

// tokenizeCommand parses a command string into discrete arguments while strictly
// rejecting shell operators, expansions, pipelines, redirections, and multi-line strings.
func tokenizeCommand(cmdStr string) ([]string, error) {
	trimmed := strings.TrimSpace(cmdStr)
	if trimmed == "" {
		return nil, ErrNotTestCommand
	}

	// Reject forbidden shell operators, redirections, and expansions
	forbidden := []string{
		"&&", "||", ";", "|", "&", "`", "$(", "${", ">", "<", "\n", "\r",
	}
	for _, f := range forbidden {
		if strings.Contains(trimmed, f) {
			return nil, ErrCompoundOrOperator
		}
	}

	// Simple lexical tokenizer for single and double quoted arguments
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	runes := []rune(trimmed)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' && !inSingle {
			escaped = true
			continue
		}

		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}

		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}

		if (r == ' ' || r == '\t') && !inSingle && !inDouble {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if inSingle || inDouble || escaped {
		return nil, ErrCompoundOrOperator // Unclosed quote or escape
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	if len(tokens) == 0 {
		return nil, ErrNotTestCommand
	}

	return tokens, nil
}

// recognizeTestCommand validates that tokens represent a direct, supported test command.
// Supported:
// - "go test [flags] [pkg...]" (excluding -json)
// - "pytest [flags] [args...]"
// - "python -m pytest [flags] [args...]"
// - "python3 -m pytest [flags] [args...]"
func recognizeTestCommand(tokens []string) (string, error) {
	if len(tokens) == 0 {
		return "", ErrNotTestCommand
	}

	first := tokens[0]

	// Reject leading environment assignments (e.g. KEY=VAL go test)
	if strings.Contains(first, "=") {
		return "", ErrLeadingEnvNotSupported
	}

	// Check Go test
	if first == "go" {
		if len(tokens) < 2 || tokens[1] != "test" {
			return "", ErrNotTestCommand
		}
		// Explicitly reject unsupported output modes such as -json
		for _, arg := range tokens[2:] {
			if arg == "-json" || arg == "--json" || strings.HasPrefix(arg, "-json=") || strings.HasPrefix(arg, "--json=") {
				return "", ErrUnsupportedOutputMode
			}
		}
		return "go_test", nil
	}

	// Check pytest
	if first == "pytest" {
		return "pytest", nil
	}

	if (first == "python" || first == "python3") && len(tokens) >= 3 {
		if tokens[1] == "-m" && tokens[2] == "pytest" {
			return "pytest", nil
		}
	}

	return "", ErrNotTestCommand
}

// extractCleanShellOutput verifies that llmContent represents a completed, synchronous,
// successful command execution in Gemini CLI 0.62.0 and extracts the command output text.
// Returns (output, true) only if the response proves complete execution without error, signal,
// cancellation, timeout, or backgrounding.
func extractCleanShellOutput(llmContentRaw json.RawMessage) (string, bool) {
	if len(llmContentRaw) == 0 {
		return "", false
	}

	var content string
	if err := json.Unmarshal(llmContentRaw, &content); err != nil {
		content = string(llmContentRaw)
	}

	// Strip <untrusted_context> wrapper tags
	content = strings.TrimPrefix(content, "<untrusted_context>")
	content = strings.TrimSuffix(content, "</untrusted_context>")
	content = strings.TrimSpace(content)

	// Incomplete / background / aborted indicators
	if strings.HasPrefix(content, "Command moved to background") ||
		strings.Contains(content, "Command cancelled by user") ||
		strings.Contains(content, "Command timed out") ||
		strings.Contains(content, "There was no output before it was cancelled") {
		return "", false
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", false
	}

	// Verify it starts with "Output: "
	firstLine := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(firstLine, "Output: ") {
		return "", false
	}

	// Verify trailing metadata from bottom up
	hasPGID := false
	outputEndIdx := len(lines)

	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Process Group PGID: ") {
			hasPGID = true
			outputEndIdx = i
			continue
		}

		if strings.HasPrefix(line, "Background PIDs: ") {
			outputEndIdx = i
			continue
		}

		if strings.HasPrefix(line, "Signal: ") {
			return "", false // Terminated by signal
		}

		if strings.HasPrefix(line, "Exit Code: ") {
			return "", false // Non-zero exit code present in metadata
		}

		if strings.HasPrefix(line, "Error: ") {
			return "", false // Error metadata present
		}

		// Reached the end of trailing metadata
		break
	}

	// A completed synchronous command in Gemini CLI 0.62.0 always records Process Group PGID
	if !hasPGID {
		return "", false
	}

	// Reassemble the output portion
	outputLines := lines[:outputEndIdx]
	if len(outputLines) == 0 {
		return "", false
	}

	// Remove "Output: " prefix from line 0
	outputLines[0] = strings.TrimPrefix(strings.TrimSpace(outputLines[0]), "Output: ")
	rawOutput := strings.Join(outputLines, "\n")

	// If output was "(empty)", return empty string
	if strings.TrimSpace(rawOutput) == "(empty)" {
		return "", false
	}

	return rawOutput, true
}

var (
	goPkgOkRegex   = regexp.MustCompile(`^ok\s+(\S+)\s+(.+)$`)
	pytestSumRegex = regexp.MustCompile(`^=+\s*(.*?)\s*in\s+[\d\.]+s\s*=+$`)
	pytestNumRegex = regexp.MustCompile(`(\d+)\s+([a-zA-Z]+)`)
)

// evaluateGoTestOutput inspects Go test output and verifies:
// 1. Zero package failures, compilation failures, panics, or test failures.
// 2. At least one qualifying passed package (excluding [no test files] and [no tests to run]).
// Returns (passed, reasonCode).
func evaluateGoTestOutput(output string) (bool, string) {
	lines := strings.Split(output, "\n")

	qualifyingCount := 0
	cachedCount := 0

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		// Absolute failure indicators
		if strings.HasPrefix(line, "FAIL\t") || line == "FAIL" ||
			strings.HasPrefix(line, "--- FAIL:") ||
			strings.HasPrefix(line, "panic:") ||
			strings.Contains(line, "[build failed]") ||
			strings.Contains(line, "[setup failed]") {
			return false, ""
		}

		// Check for package ok line: ok  <pkg>  <duration/cached>
		if matches := goPkgOkRegex.FindStringSubmatch(line); len(matches) == 3 {
			pkgDetail := strings.TrimSpace(matches[2])

			// Exclude packages with [no tests to run]
			if strings.Contains(pkgDetail, "[no tests to run]") {
				continue
			}

			// Exclude packages with [no test files]
			if strings.Contains(pkgDetail, "[no test files]") {
				continue
			}

			qualifyingCount++
			if strings.Contains(pkgDetail, "(cached)") {
				cachedCount++
			}
		}
	}

	// Require at least one qualifying passed package and zero failures
	if qualifyingCount == 0 {
		return false, ""
	}

	if cachedCount == qualifyingCount {
		return true, "go_test_cached"
	}
	return true, "go_test_passed"
}

// evaluatePytestOutput inspects pytest output and verifies:
// 1. Zero test failures, errors, or interruptions.
// 2. At least one passed test in the summary line.
// 3. Excludes skipped-only or no-test runs.
// Returns (passed, reasonCode).
func evaluatePytestOutput(output string) (bool, string) {
	// Look for failure or error sections anywhere in output
	if strings.Contains(output, "=== FAILURES ===") ||
		strings.Contains(output, "=== ERRORS ===") ||
		strings.Contains(output, "KeyboardInterrupt") ||
		strings.Contains(output, "INTERRUPTED") {
		return false, ""
	}

	lines := strings.Split(output, "\n")

	// Scan from bottom up for the final pytest summary line: =+ ... in ...s =+
	var summaryLine string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if pytestSumRegex.MatchString(line) {
			summaryLine = line
			break
		}
	}

	if summaryLine == "" {
		return false, ""
	}

	// Parse summary items: e.g. "5 passed, 2 skipped" or "1 failed, 2 passed"
	matches := pytestSumRegex.FindStringSubmatch(summaryLine)
	if len(matches) < 2 {
		return false, ""
	}

	body := matches[1]
	numMatches := pytestNumRegex.FindAllStringSubmatch(body, -1)
	if len(numMatches) == 0 {
		return false, ""
	}

	passedCount := 0
	failedCount := 0
	errorCount := 0

	for _, m := range numMatches {
		if len(m) < 3 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		kind := strings.ToLower(m[2])

		switch kind {
		case "passed":
			passedCount += n
		case "failed":
			failedCount += n
		case "error", "errors":
			errorCount += n
		}
	}

	// Any failure or error invalidates the test pass
	if failedCount > 0 || errorCount > 0 {
		return false, ""
	}

	// Require at least one passed test
	if passedCount < 1 {
		return false, ""
	}

	return true, "pytest_passed"
}

// DetectTestsPassed evaluates the tool input and response of a run_shell_command execution.
// Returns (true, reasonCode) if and only if the command was a supported, direct test runner
// and its output unambiguously verifies test success.
func DetectTestsPassed(toolInputRaw json.RawMessage, llmContentRaw json.RawMessage) (bool, string) {
	if len(toolInputRaw) == 0 {
		return false, ""
	}

	var input ShellToolInput
	if err := json.Unmarshal(toolInputRaw, &input); err != nil {
		return false, ""
	}

	// Reject background execution
	if input.IsBackground {
		return false, ""
	}

	// Tokenize command line with strict rejection of shell operators and pipelines
	tokens, err := tokenizeCommand(input.Command)
	if err != nil {
		return false, ""
	}

	// Recognize runner
	runner, err := recognizeTestCommand(tokens)
	if err != nil {
		return false, ""
	}

	// Extract and verify clean, completed synchronous output from Gemini CLI
	cleanOutput, ok := extractCleanShellOutput(llmContentRaw)
	if !ok || cleanOutput == "" {
		return false, ""
	}

	switch runner {
	case "go_test":
		return evaluateGoTestOutput(cleanOutput)
	case "pytest":
		return evaluatePytestOutput(cleanOutput)
	default:
		return false, ""
	}
}
