package main

import (
	"context"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// lifecycle records the app's Start/Stop calls in order. When probe is set,
// each Stop event also records whether the HTTP listener still accepts
// connections at that moment.
type lifecycle struct {
	mu     sync.Mutex
	events []string
	probe  func() string
}

func (l *lifecycle) record(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.probe != nil && strings.HasSuffix(
		event,
		".stop",
	) {
		event += " " + l.probe()
	}
	l.events = append(
		l.events,
		event,
	)
}

func (l *lifecycle) setProbe(probe func() string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.probe = probe
}

func (l *lifecycle) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

// fakePoller blocks in Start until Stop, like the Telegram update loop.
type fakePoller struct {
	log     *lifecycle
	started chan struct{}
	stopped chan struct{}
}

func newFakePoller(log *lifecycle) *fakePoller {
	return &fakePoller{
		log:     log,
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (p *fakePoller) Start() {
	p.log.record("bot.start")
	close(p.started)
	<-p.stopped
}

func (p *fakePoller) Stop() {
	p.log.record("bot.stop")
	close(p.stopped)
}

type fakeScheduler struct {
	log *lifecycle
}

func (s *fakeScheduler) Start() { s.log.record("scheduler.start") }

func (s *fakeScheduler) Stop() { s.log.record("scheduler.stop") }

// fakeRefresher hands over the context the refresh runs with.
type fakeRefresher struct {
	ctx chan context.Context
}

func (r *fakeRefresher) RefreshStaleMiniAppMessages(ctx context.Context) {
	r.ctx <- ctx
}

func receive[T any](
	t *testing.T,
	ch <-chan T,
	what string,
) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf(
			"timed out waiting for %s",
			what,
		)
		var zero T
		return zero
	}
}

func TestAppRunStartsWorkersAndStopsBotThenHTTPThenScheduler(t *testing.T) {
	log := &lifecycle{}
	bot := newFakePoller(log)
	refresher := &fakeRefresher{ctx: make(
		chan context.Context,
		1,
	)}
	listenAddr := make(
		chan string,
		1,
	)
	a := &app{
		bot:       bot,
		scheduler: &fakeScheduler{log: log},
		refresher: refresher,
		server: &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
			BaseContext: func(l net.Listener) context.Context {
				listenAddr <- l.Addr().String()
				return context.Background()
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(
		chan error,
		1,
	)
	go func() { runErr <- a.Run(ctx) }()

	addr := receive(
		t,
		listenAddr,
		"HTTP serve",
	)
	receive(
		t,
		bot.started,
		"bot start",
	)
	refreshCtx := receive(
		t,
		refresher.ctx,
		"stale Mini App refresh",
	)
	log.setProbe(func() string {
		conn, err := net.DialTimeout(
			"tcp",
			addr,
			time.Second,
		)
		if err != nil {
			return "http=closed"
		}
		conn.Close()
		return "http=open"
	})

	cancel()
	if err := receive(
		t,
		runErr,
		"Run to return",
	); err != nil {
		t.Fatalf(
			"Run error = %v, want nil after cancel",
			err,
		)
	}

	want := []string{
		"scheduler.start",
		"bot.start",
		"bot.stop http=open",
		"scheduler.stop http=closed",
	}
	if got := log.list(); !slices.Equal(
		got,
		want,
	) {
		t.Fatalf(
			"lifecycle = %q, want %q",
			got,
			want,
		)
	}
	if refreshCtx.Err() == nil {
		t.Fatal("stale Mini App refresh context was not cancelled on shutdown")
	}
}

func TestAppRunReturnsListenErrorWithoutStartingWorkers(t *testing.T) {
	occupied, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf(
			"occupy port: %v",
			err,
		)
	}
	defer occupied.Close()

	log := &lifecycle{}
	a := &app{
		bot:       newFakePoller(log),
		scheduler: &fakeScheduler{log: log},
		refresher: &fakeRefresher{ctx: make(
			chan context.Context,
			1,
		)},
		server: &http.Server{Addr: occupied.Addr().String()},
	}

	if err := a.Run(context.Background()); err == nil {
		t.Fatal("Run error = nil, want listen failure")
	}
	if got := log.list(); len(got) != 0 {
		t.Fatalf(
			"lifecycle = %q, want nothing started on a port conflict",
			got,
		)
	}
}
