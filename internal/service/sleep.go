package service

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/wneessen/waybar-weather/internal/logger"
)

const (
	busReconnectDelay   = 5 * time.Second
	subscribeRetryDelay = 10 * time.Second
	reconnectDelay      = 2 * time.Second
	debounceWindow      = 2 // seconds
	networkWakeupDelay  = 5 * time.Second
	signalBufferSize    = 8
)

func (s *Service) monitorSleepResume(ctx context.Context) {
	var lastResumeUnix int64

	for {
		conn := s.connectToSystemBus(ctx)
		if conn == nil {
			return // context cancelled
		}

		if !s.setupSleepMonitoring(ctx, conn) {
			continue // retry connection
		}

		sigCh := make(chan *dbus.Signal, signalBufferSize)
		conn.Signal(sigCh)
		s.logger.Debug("subscribed to logind PrepareForSleep signal")

		s.handleSleepSignals(ctx, sigCh, &lastResumeUnix)

		// Clean up before reconnect
		conn.RemoveSignal(sigCh)
		if err := conn.Close(); err != nil {
			s.logger.Error("failed to close system bus connection", logger.Err(err))
		}

		// If we're here because of ctx cancel, exit; otherwise reconnect
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(reconnectDelay)
		}
	}
}

func (s *Service) connectToSystemBus(ctx context.Context) *dbus.Conn {
	for {
		conn, err := dbus.ConnectSystemBus()
		if err != nil {
			select {
			case <-time.After(busReconnectDelay):
				continue
			case <-ctx.Done():
				return nil
			}
		}

		// Ensure cleanup on context cancellation
		go func() {
			<-ctx.Done()
			if err := conn.Close(); err != nil {
				s.logger.Error("failed to close system bus connection", logger.Err(err))
			}
		}()

		return conn
	}
}

func (s *Service) setupSleepMonitoring(ctx context.Context, conn *dbus.Conn) bool {
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		s.logger.Error("failed to subscribe to logind PrepareForSleep signal", logger.Err(err))
		if closeErr := conn.Close(); closeErr != nil {
			s.logger.Error("failed to close system bus connection", logger.Err(closeErr))
		}
		select {
		case <-time.After(subscribeRetryDelay):
			return false
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func (s *Service) handleSleepSignals(ctx context.Context, sigCh chan *dbus.Signal, lastResumeUnix *int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case sgn, ok := <-sigCh:
			if !ok {
				// connection likely closed; reconnect
				return
			}
			s.processSleepSignal(ctx, sgn, lastResumeUnix)
		}
	}
}

func (s *Service) processSleepSignal(ctx context.Context, sgn *dbus.Signal, lastResumeUnix *int64) {
	if len(sgn.Body) != 1 {
		return
	}
	sleeping, ok := sgn.Body[0].(bool)
	if !ok || sleeping {
		return
	}
	s.handleResumeEvent(ctx, lastResumeUnix)
}

func (s *Service) handleResumeEvent(ctx context.Context, lastResumeUnix *int64) {
	now := time.Now().Unix()
	if now-atomic.LoadInt64(lastResumeUnix) < debounceWindow {
		return // debounce
	}

	atomic.StoreInt64(lastResumeUnix, now)
	s.logger.Debug("resuming from sleep, fetching latest weather data")

	// Give the system time to wake up and establish network connection
	time.Sleep(networkWakeupDelay)
	go s.fetchWeather(ctx)
}
