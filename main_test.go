package main

import (
	"testing"
	"time"
)

func TestShutdownWatchdogForcesExitAfterDeadline(t *testing.T) {
	fired := make(chan struct{})
	stop := startShutdownWatchdog(20*time.Millisecond, func() { close(fired) })
	defer stop()
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("shutdown watchdog did not fire")
	}
}

func TestShutdownWatchdogCanBeCancelled(t *testing.T) {
	fired := make(chan struct{}, 1)
	stop := startShutdownWatchdog(40*time.Millisecond, func() { fired <- struct{}{} })
	stop()
	select {
	case <-fired:
		t.Fatal("cancelled shutdown watchdog fired")
	case <-time.After(100 * time.Millisecond):
	}
}
