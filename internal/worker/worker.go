package worker

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/scheduler"
	"agent-sfx/internal/sounds"
)

// Daemon coordinates the IPC server, scheduler, and singleton lock.
type Daemon struct {
	cfg       config.Config
	soundsDir string
	clock     scheduler.Clock
	player    audio.Player
	cache     *audio.Cache
	selector  *sounds.Selector

	server    *ipc.Server
	sched     *scheduler.Scheduler
	lock      *Lock
	startTime time.Time

	stopCh  chan struct{}
	stopped bool
}

// NewDaemon initializes a Daemon instance with provided or detected dependencies.
func NewDaemon(
	cfg config.Config,
	soundsDir string,
	clock scheduler.Clock,
	player audio.Player,
	cache *audio.Cache,
	selector *sounds.Selector,
) (*Daemon, error) {
	if clock == nil {
		clock = scheduler.RealClock{}
	}
	if player == nil {
		p, err := audio.DetectPlayer()
		if err != nil {
			return nil, fmt.Errorf("failed to detect audio player: %w", err)
		}
		player = p
	}
	if cache == nil {
		c, err := audio.NewCache("")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize cache: %w", err)
		}
		cache = c
	}
	if selector == nil {
		selector = sounds.NewSelector(nil)
	}

	return &Daemon{
		cfg:       cfg,
		soundsDir: soundsDir,
		clock:     clock,
		player:    player,
		cache:     cache,
		selector:  selector,
		stopCh:    make(chan struct{}),
	}, nil
}

// Run acquires the singleton lock, starts the scheduler and IPC server, and blocks
// until a stop request is received or an OS termination signal is handled.
func (d *Daemon) Run(socketPath string) error {
	socketDir := ipc.SocketDir(socketPath)

	// 1. Atomic singleton acquisition
	lock, err := AcquireSingletonLock(socketDir)
	if err != nil {
		return fmt.Errorf("singleton acquisition failed: %w", err)
	}
	d.lock = lock
	defer func() { _ = d.lock.Release() }()

	// Write worker.pid for diagnostics
	pidFile := filepath.Join(socketDir, "worker.pid")
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	defer func() { _ = os.Remove(pidFile) }()

	// 2. Start scheduler
	d.sched = scheduler.New(d.cfg, d.clock, d.player, d.cache, d.selector, d.soundsDir)
	d.sched.Start()
	defer d.sched.Stop()

	// 3. Setup IPC server
	d.startTime = d.clock.Now()
	srv, err := ipc.NewServer(socketPath, d.handleIPC)
	if err != nil {
		return fmt.Errorf("failed to create ipc server: %w", err)
	}
	d.server = srv
	defer func() { _ = d.server.Close() }()

	// 4. Listen for signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.server.Serve()
	}()

	select {
	case <-sigCh:
		// Graceful exit on signal
	case <-d.stopCh:
		// Graceful exit on IPC stop request
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("ipc server error: %w", err)
		}
	}

	return nil
}

func (d *Daemon) handleIPC(req ipc.Request) ipc.Response {
	switch req.Type {
	case ipc.TypeEvent:
		if req.Event == nil {
			return ipc.Response{OK: false, Error: "missing event"}
		}
		accepted := d.sched.Enqueue(*req.Event)
		return ipc.Response{OK: accepted}

	case ipc.TypeStatus:
		uptime := int64(d.clock.Now().Sub(d.startTime).Seconds())
		return ipc.Response{
			OK:      true,
			PID:     os.Getpid(),
			Uptime:  uptime,
			Playing: d.sched.IsPlaying(),
		}

	case ipc.TypeStop:
		d.requestStop()
		return ipc.Response{OK: true}

	default:
		return ipc.Response{OK: false, Error: fmt.Sprintf("unsupported request type: %s", req.Type)}
	}
}

func (d *Daemon) requestStop() {
	if !d.stopped {
		d.stopped = true
		close(d.stopCh)
	}
}
