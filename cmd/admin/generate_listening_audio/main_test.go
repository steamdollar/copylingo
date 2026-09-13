package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForNextCycleSkipsNonPositiveDelay(t *testing.T) {
	if err := waitForNextCycle(context.Background(), 0); err != nil {
		t.Fatalf("waitForNextCycle() error = %v", err)
	}
}

func TestWaitForNextCycleHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForNextCycle(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForNextCycle() error = %v, want context.Canceled", err)
	}
}
