package executionservice

import (
	"context"
	"errors"
	"net"
	"time"
)

// Manager serializes worker starts and owns only the process it created.
// Recovery happens before forwarding a new request, never by replaying it.
type Manager struct {
	ctx         context.Context
	config      Config
	gate        chan struct{}
	stop        func()
	closed      bool
	lastFailure error
	retryAfter  time.Time
}

func NewManager(ctx context.Context, config Config) *Manager {
	return &Manager{ctx: ctx, config: config, gate: make(chan struct{}, 1)}
}

func (m *Manager) Ensure(ctx context.Context) error {
	select {
	case m.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
	defer func() { <-m.gate }()
	if m.closed {
		return errors.New("execution service manager is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.ctx.Err(); err != nil {
		return err
	}
	if serviceAvailable(ctx) {
		return nil
	}
	if time.Now().Before(m.retryAfter) {
		return m.lastFailure
	}
	if m.stop != nil {
		// A slow or unhealthy HTTP response is not proof that the worker died.
		// Do not kill running tests just because a readiness probe failed.
		if connection, err := net.DialTimeout("tcp", "127.0.0.1:54321", 300*time.Millisecond); err == nil {
			connection.Close()
			return errors.New("the local execution service is still running but is not ready; check its log before restarting it")
		}
		m.stop()
		m.stop = nil
	}
	stop, err := ensure(m.ctx, ctx, m.config)
	if err != nil {
		if m.config.Logger != nil {
			m.config.Logger.Error("local execution service could not start", "error", err)
		}
		m.lastFailure = err
		m.retryAfter = time.Now().Add(2 * time.Second)
		return err
	}
	m.stop = stop
	m.lastFailure = nil
	return nil
}

func (m *Manager) Close() {
	m.gate <- struct{}{}
	defer func() { <-m.gate }()
	m.closed = true
	if m.stop != nil {
		m.stop()
		m.stop = nil
	}
}
