package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/preview"
	"agent-sfx/internal/sounds"
	"agent-sfx/internal/worker"
)

const AppVersion = "0.1.0-m1"

type AudioReport struct {
	BackendName string `json:"backend_name"`
	Available   bool   `json:"available"`
	Details     string `json:"details"`
}

type WorkerReport struct {
	Supported  bool   `json:"supported"`
	Running    bool   `json:"running"`
	PID        int    `json:"pid,omitempty"`
	UptimeSec  int64  `json:"uptime_sec,omitempty"`
	Enabled    bool   `json:"enabled"`
	SocketPath string `json:"socket_path"`
}

type EventSoundReport struct {
	Count int      `json:"count"`
	Files []string `json:"files"`
	Valid bool     `json:"valid"`
}

type Report struct {
	Version      string                                `json:"version"`
	OS           string                                `json:"os"`
	Arch         string                                `json:"arch"`
	GoVersion    string                                `json:"go_version"`
	Audio        AudioReport                           `json:"audio"`
	Worker       WorkerReport                          `json:"worker"`
	ConfigFile   string                                `json:"config_file"`
	ConfigExists bool                                  `json:"config_exists"`
	ConfigValid  bool                                  `json:"config_valid"`
	CacheDir     string                                `json:"cache_dir"`
	SoundsDir    string                                `json:"sounds_dir"`
	SoundAssets  map[events.EventKind]EventSoundReport `json:"sound_assets"`
	Adapters     map[string]string                     `json:"adapters"`
	Issues       []string                              `json:"issues,omitempty"`
}

// GenerateReport inspects the runtime environment, audio backend, config, and sound assets.
func GenerateReport(customConfigPath, customSoundsDir string) Report {
	rep := Report{
		Version:     AppVersion,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoVersion:   runtime.Version(),
		SoundAssets: make(map[events.EventKind]EventSoundReport),
		Adapters: map[string]string{
			"gemini": "implemented (Milestone M1 - neutral receiver active, full classification in M2)",
			"claude": "implemented (Milestone M5 - neutral receiver active, event classification in M5)",
		},
	}

	// 1. Audio Player
	player, _ := audio.DetectPlayer()
	rep.Audio = AudioReport{
		BackendName: player.BackendName(),
		Available:   player.IsAvailable(),
	}
	if !player.IsAvailable() {
		switch runtime.GOOS {
		case "linux":
			rep.Audio.Details = "no supported audio utility found; install pipewire-utils (pw-play), pulseaudio-utils (paplay), or alsa-utils (aplay)"
		case "windows":
			rep.Audio.Details = "powershell.exe not found in PATH"
		case "darwin":
			rep.Audio.Details = "afplay not found in /usr/bin/afplay or PATH"
		default:
			rep.Audio.Details = fmt.Sprintf("unsupported OS %s", runtime.GOOS)
		}
		rep.Issues = append(rep.Issues, fmt.Sprintf("Audio: %s", rep.Audio.Details))
	} else {
		rep.Audio.Details = fmt.Sprintf("native %s backend available", player.BackendName())
	}

	// 2. Worker Status
	sockPath, err := ipc.ResolveSocketPath()
	if err != nil {
		rep.Worker = WorkerReport{
			Supported:  false,
			SocketPath: "",
		}
		if runtime.GOOS == "windows" {
			rep.Issues = append(rep.Issues, "Worker: background daemon is not yet supported on Windows")
		}
	} else {
		rep.Worker = WorkerReport{
			Supported:  true,
			SocketPath: sockPath,
		}
		status, err := worker.StatusDaemon(sockPath)
		if err == nil && status.OK {
			rep.Worker.Running = true
			rep.Worker.PID = status.PID
			rep.Worker.UptimeSec = status.Uptime
			rep.Worker.Enabled = status.Enabled
		}
	}

	// 3. Config
	cfg, cfgPath, err := config.Load(customConfigPath)
	rep.ConfigFile = cfgPath
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		rep.ConfigExists = true
	}
	if err != nil {
		rep.ConfigValid = false
		rep.Issues = append(rep.Issues, fmt.Sprintf("Config error: %v", err))
	} else {
		rep.ConfigValid = true
	}

	// 4. Cache
	if cache, err := audio.NewCache(""); err == nil {
		rep.CacheDir = cache.Dir()
	}

	// 5. Sounds Directory
	resolvedSounds := preview.ResolveSoundsDir(customSoundsDir, cfg.SoundsDir)
	rep.SoundsDir = resolvedSounds

	for _, k := range events.AllEvents {
		wavs, err := sounds.ListEventSounds(resolvedSounds, k)
		reportItem := EventSoundReport{
			Count: len(wavs),
			Files: []string{},
			Valid: true,
		}

		if err != nil || len(wavs) == 0 {
			reportItem.Valid = false
			rep.Issues = append(rep.Issues, fmt.Sprintf("Assets: no WAV files found for event %s in %s", k, resolvedSounds))
		} else {
			for _, w := range wavs {
				reportItem.Files = append(reportItem.Files, filepath.Base(w))
				if _, valErr := audio.ValidateWAVFile(w); valErr != nil {
					reportItem.Valid = false
					rep.Issues = append(rep.Issues, fmt.Sprintf("Assets: file %s is invalid: %v", w, valErr))
				}
			}
		}
		rep.SoundAssets[k] = reportItem
	}

	return rep
}

// FormatHuman returns a clean, terminal-readable summary of the report.
func (r *Report) FormatHuman() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent SFX Doctor Report (v%s)\n", r.Version)
	fmt.Fprintf(&b, "----------------------------------------\n")
	fmt.Fprintf(&b, "Environment : %s/%s (%s)\n", r.OS, r.Arch, r.GoVersion)

	audioStatus := "OK"
	if !r.Audio.Available {
		audioStatus = "UNAVAILABLE"
	}
	fmt.Fprintf(&b, "Audio Player: %s [%s] (%s)\n", r.Audio.BackendName, audioStatus, r.Audio.Details)

	if !r.Worker.Supported {
		fmt.Fprintf(&b, "Worker Daemon: unsupported on %s\n", r.OS)
	} else if r.Worker.Running {
		muteInfo := ""
		if !r.Worker.Enabled {
			muteInfo = " (live audio muted)"
		}
		fmt.Fprintf(&b, "Worker Daemon: running [PID: %d, uptime: %ds, socket: %s]%s\n",
			r.Worker.PID, r.Worker.UptimeSec, r.Worker.SocketPath, muteInfo)
	} else {
		fmt.Fprintf(&b, "Worker Daemon: stopped [socket: %s]\n", r.Worker.SocketPath)
	}

	cfgStatus := "using defaults"
	if r.ConfigExists {
		if r.ConfigValid {
			cfgStatus = "present & valid"
		} else {
			cfgStatus = "INVALID"
		}
	}
	fmt.Fprintf(&b, "Config File : %s (%s)\n", r.ConfigFile, cfgStatus)
	fmt.Fprintf(&b, "Cache Dir   : %s\n", r.CacheDir)
	fmt.Fprintf(&b, "Sounds Dir  : %s\n", r.SoundsDir)

	fmt.Fprintf(&b, "\nEvent Sounds Status:\n")
	for _, k := range events.AllEvents {
		asset := r.SoundAssets[k]
		status := "OK"
		if !asset.Valid || asset.Count == 0 {
			status = "MISSING/INVALID"
		}
		fmt.Fprintf(&b, "  - %-22s: %d sound(s) [%s]\n", k, asset.Count, status)
	}

	fmt.Fprintf(&b, "\nAgent Adapters:\n")
	for name, desc := range r.Adapters {
		fmt.Fprintf(&b, "  - %-10s: %s\n", name, desc)
	}

	if len(r.Issues) > 0 {
		fmt.Fprintf(&b, "\nWarnings / Diagnosed Issues:\n")
		for _, issue := range r.Issues {
			fmt.Fprintf(&b, "  ! %s\n", issue)
		}
	} else {
		fmt.Fprintf(&b, "\nAll systems ready.\n")
	}

	return b.String()
}

// FormatJSON returns the serialized JSON representation of the report.
func (r *Report) FormatJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
