package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TraceEntry records metadata-only diagnostics for investigating hook dispatches.
// Never logs prompts, file paths, commands, error contents, or raw payloads.
type TraceEntry struct {
	Timestamp      string `json:"timestamp"`
	HookEventName  string `json:"hook_event_name"`
	ToolName       string `json:"tool_name,omitempty"`
	HasErrorField  bool   `json:"has_error_field"`
	ErrorJSONType  string `json:"error_json_type,omitempty"`
	Classification string `json:"classification"`
	IPCDelivery    string `json:"ipc_delivery,omitempty"`
}

var safeToolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,64}$`)

// SanitizeToolName returns the tool name if it consists only of safe identifier characters;
// otherwise returns "other" to prevent any data leakage.
func SanitizeToolName(name string) string {
	name = strings.TrimSpace(name)
	if safeToolNameRegex.MatchString(name) {
		return name
	}
	if name == "" {
		return ""
	}
	return "other"
}

// TraceLogPath returns the path to the trace log file if opted in via AGENT_SFX_TRACE_LOG.
// If AGENT_SFX_TRACE_LOG is unset or empty, tracing is disabled and returns "".
// Tracing must perform no file or directory creation unless explicitly opted in.
func TraceLogPath() string {
	return strings.TrimSpace(os.Getenv("AGENT_SFX_TRACE_LOG"))
}

// LogTrace appends a single JSON line to the metadata trace log if opted in.
// If AGENT_SFX_TRACE_LOG is unset or empty, no file or directory is created.
// Logging is best-effort and bounded; any error is silently ignored to preserve hook neutrality.
func LogTrace(entry TraceEntry) {
	logPath := TraceLogPath()
	if logPath == "" {
		return
	}

	defer func() {
		_ = recover()
	}()

	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}

	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = fmt.Fprintln(f, string(data))
}
