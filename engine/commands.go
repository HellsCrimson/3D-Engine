package engine

import (
	"errors"
	"sync"

	"3d-engine/utils"
)

// ErrShuttingDown is returned to callers waiting on a command that the engine
// will never run because the frame loop has stopped.
var ErrShuttingDown = errors.New("engine is shutting down")

// Command runs on the frame-loop goroutine, which is the only place it is safe
// to allocate GL resources.
type Command func(a *App) error

type queuedCommand struct {
	fn Command
	// done is nil for fire-and-forget commands.
	done chan error
}

// commandQueue lets any goroutine hand work back to the frame loop. This is the
// general form of the pending-scene-path mechanism SceneManager used to carry
// on its own: anything that must touch the GPU goes through here.
type commandQueue struct {
	mu     sync.Mutex
	queue  []queuedCommand
	closed bool
}

func (q *commandQueue) push(cmd queuedCommand) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return false
	}
	q.queue = append(q.queue, cmd)
	return true
}

// take swaps the pending commands out so they can run without holding the lock,
// which is what lets a command queue further commands (they run next frame).
func (q *commandQueue) take() []queuedCommand {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil
	}
	pending := q.queue
	q.queue = nil
	return pending
}

func (q *commandQueue) close() {
	q.mu.Lock()
	pending := q.queue
	q.queue = nil
	q.closed = true
	q.mu.Unlock()

	for _, cmd := range pending {
		if cmd.done != nil {
			cmd.done <- ErrShuttingDown
		}
	}
}

// Defer queues fn to run on the frame-loop goroutine at the start of the next
// frame, where GL calls are legal. Safe to call from any goroutine. Errors are
// logged; use Do if the caller needs to see them.
func (a *App) Defer(fn Command) {
	if !a.commands.push(queuedCommand{fn: fn}) {
		utils.Logger().Println("dropped deferred command:", ErrShuttingDown)
	}
}

// Do queues fn and blocks until the frame loop has run it, returning its error.
// It must not be called from the frame-loop goroutine — that deadlocks, since
// the loop is what drains the queue.
func (a *App) Do(fn Command) error {
	done := make(chan error, 1)
	if !a.commands.push(queuedCommand{fn: fn, done: done}) {
		return ErrShuttingDown
	}
	return <-done
}

// drainCommands runs everything queued since the last frame. Called from Run.
func (a *App) drainCommands() {
	for _, cmd := range a.commands.take() {
		err := cmd.fn(a)
		if cmd.done != nil {
			cmd.done <- err
			continue
		}
		if err != nil {
			utils.Logger().Printf("Deferred command failed: %v", err)
		}
	}
}
