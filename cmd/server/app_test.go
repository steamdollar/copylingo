package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
)

type recordingCloser struct {
	closed bool
}

func (closer *recordingCloser) Close() error {
	closer.closed = true
	return nil
}

// A port conflict returns before shutdown runs; Run must still release DB/Redis.
func TestRunClosesConnectionsWhenListenFails(t *testing.T) {
	occupiedListener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatalf(
			"occupy port: %v",
			err,
		)
	}
	defer occupiedListener.Close()

	connection := &recordingCloser{}
	application := &app{
		server:  &http.Server{Addr: occupiedListener.Addr().String()},
		closers: []io.Closer{connection},
	}

	if err := application.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want listen error")
	}
	if !connection.closed {
		t.Fatal("Run() returned without closing connections")
	}
}
