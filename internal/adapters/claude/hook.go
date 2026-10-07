package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/preview"
	"agent-sfx/internal/worker"
)

const (
	MaxHookStdinBytes = 1024 * 1024 // 1 MiB defensive maximum
	HookReadTimeout   = 200 * time.Millisecond
	HookIPCTimeout    = 25 * time.Millisecond
)

var (
	ErrOversizedHookInput = errors.New("hook input exceeds 1 MiB limit")
)

// HookResult stores timing metrics for diagnostics.
type HookResult struct {
	TotalDuration time.Duration `json:"total_duration"`
	ReadDuration  time.Duration `json:"read_duration"`
	IPCDuration   time.Duration `json:"ipc_duration"`
	SentEvent     bool          `json:"sent_event"`
	Dropped       bool          `json:"dropped"`
	DropReason    string        `json:"drop_reason,omitempty"`
}

// Receiver handles incoming Claude Code hook execution.
type Receiver struct {
	SocketPath string
	BinaryPath string
	SoundsDir  string
	ConfigPath string
	TestMode   bool
}

// ReadBoundedStdin reads up to MaxHookStdinBytes within HookReadTimeout.
func ReadBoundedStdin(ctx context.Context, r io.Reader) ([]byte, error) {
	readCtx, cancel := context.WithTimeout(ctx, HookReadTimeout)
	defer cancel()

	done := make(chan struct{})
	var data []byte
	var readErr error

	go func() {
		defer close(done)
		lr := io.LimitReader(r, int64(MaxHookStdinBytes)+1)
		data, readErr = io.ReadAll(lr)
	}()

	select {
	case <-readCtx.Done():
		return nil, readCtx.Err()
	case <-done:
		if readErr != nil {
			return nil, readErr
		}
		if len(data) > MaxHookStdinBytes {
			return nil, ErrOversizedHookInput
		}
		return data, nil
	}
}

// ProcessHook processes an incoming Claude Code hook invocation:
// 1. Defensively reads stdin with size and time bounds.
// 2. Emits exactly "{}\n" on stdout under all circumstances.
// 3. Normalizes payload to a canonical event or triggers silent worker launch.
// 4. Bounded 25ms IPC dispatch to worker.
func (rcv *Receiver) ProcessHook(ctx context.Context, stdin io.Reader, stdout io.Writer) (*HookResult, error) {
	start := time.Now()
	res := &HookResult{}
	trace := TraceEntry{}
	var eventToSend *events.Event
	traceClassification := "ignored"

	// Guarantee neutral "{}\n" stdout output regardless of outcome
	defer func() {
		_, _ = fmt.Fprintln(stdout, "{}")
		res.TotalDuration = time.Since(start)

		if trace.HookEventName != "" {
			if eventToSend != nil {
				trace.Classification = string(eventToSend.Kind)
			} else {
				trace.Classification = traceClassification
			}
			LogTrace(trace)
		}
	}()

	readStart := time.Now()
	payload, err := ReadBoundedStdin(ctx, stdin)
	res.ReadDuration = time.Since(readStart)

	if err != nil {
		res.Dropped = true
		res.DropReason = fmt.Sprintf("stdin read error: %v", err)
		traceClassification = "stdin_read_error"
		return res, nil
	}

	if len(payload) == 0 {
		res.Dropped = true
		res.DropReason = "empty payload"
		traceClassification = "empty_payload"
		return res, nil
	}

	// Extract metadata-only diagnostics
	var rawMap map[string]any
	if err := json.Unmarshal(payload, &rawMap); err == nil {
		if evName, ok := rawMap["hook_event_name"].(string); ok {
			trace.HookEventName = evName
		}
		if tName, ok := rawMap["tool_name"].(string); ok {
			trace.ToolName = SanitizeToolName(tName)
		}
		if errVal, exists := rawMap["error"]; exists {
			trace.HasErrorField = true
			switch errVal.(type) {
			case string:
				trace.ErrorJSONType = "string"
			case map[string]any:
				trace.ErrorJSONType = "object"
			case nil:
				trace.ErrorJSONType = "null"
			default:
				trace.ErrorJSONType = "other"
			}
		}
	}

	// 1. Silent SessionStart handling: spawn detached worker if absent
	var base struct {
		HookEventName string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(payload, &base); err == nil && base.HookEventName == "SessionStart" {
		traceClassification = "session_start"

		resolvedConfig := rcv.ConfigPath
		if resolvedConfig == "" {
			if defPath, err := config.DefaultConfigPath(); err == nil {
				resolvedConfig = defPath
			}
		}

		resolvedSounds := rcv.SoundsDir
		if resolvedSounds == "" {
			resolvedSounds = preview.ResolveSoundsDir("", "")
		}

		_ = worker.SpawnDetachedWorker(rcv.BinaryPath, rcv.SocketPath, resolvedConfig, resolvedSounds)
		res.Dropped = true
		res.DropReason = "SessionStart triggered silent worker startup"
		return res, nil
	}

	// 2. Normalize payload
	eventsList, normErr := Normalize(payload)
	if normErr != nil {
		res.Dropped = true
		res.DropReason = fmt.Sprintf("normalization error: %v", normErr)
		traceClassification = "normalization_error"
		return res, nil
	}

	if len(eventsList) > 0 {
		eventToSend = &eventsList[0]
	} else if rcv.TestMode {
		var synth events.Event
		if err := json.Unmarshal(payload, &synth); err == nil && synth.Kind.IsValid() {
			eventToSend = &synth
		}
	}

	if eventToSend == nil {
		res.Dropped = true
		res.DropReason = "unsupported hook event or ignored"
		traceClassification = "ignored"
		return res, nil
	}

	// 3. Dispatch to worker over IPC with 25ms deadline (no retries)
	ipcStart := time.Now()
	ipcCtx, cancel := context.WithTimeout(ctx, HookIPCTimeout)
	defer cancel()

	_, err = ipc.Send(ipcCtx, rcv.SocketPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event:   eventToSend,
	})
	res.IPCDuration = time.Since(ipcStart)

	if err != nil {
		res.Dropped = true
		res.DropReason = fmt.Sprintf("ipc send failed: %v", err)
		trace.IPCDelivery = "failed"
		return res, nil
	}

	trace.IPCDelivery = "ok"
	res.SentEvent = true
	return res, nil
}
