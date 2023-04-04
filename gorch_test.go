package gorch_test

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/snet-commerce/gorch"
)

var stopSignal = os.Interrupt

func TestOrchestrator_BeforeError(t *testing.T) {
	bfrErr := errors.New("before error")

	orch := gorch.New(gorch.WithStopSignals(stopSignal))
	orch.Before(func() error {
		return bfrErr
	})

	for err := range orch.Serve() {
		if !errors.Is(bfrErr, err) {
			t.Fatalf("expect an {%v} error to be raised but got {%v}", bfrErr, err)
		}
	}
}

func TestOrchestrator_StarterFailed(t *testing.T) {
	startErr := errors.New("start error")
	orch := gorch.New(gorch.WithStopSignals(stopSignal))

	errCh := make(chan error, 1)
	starter1 := func() error {
		return <-errCh
	}

	starter2 := func() error {
		c := make(chan error, 1)
		return <-c
	}

	errCh <- startErr
	for err := range orch.Serve(starter1, starter2) {
		if errors.Is(startErr, err) {
			orch.Stop()
			continue
		}
		t.Fatalf("expect an {%v} error to be raised but got {%v}", startErr, err)
	}
}

func TestOrchestrator_AfterError(t *testing.T) {
	afterErr := errors.New("after error")
	orch := gorch.New(gorch.WithStopSignals(stopSignal))

	errCh := make(chan error, 1)
	starter := func() error {
		for err := range errCh {
			return err
		}
		return nil
	}

	go func() {
		if err := raiseSignal(stopSignal); err != nil {
			t.Errorf("failed to sent stop signal: %v", err)
		}
	}()
	for err := range orch.Serve(starter) {
		if !errors.Is(afterErr, err) {
			t.Fatalf("expect an {%v} error to be raised but got {%v}", afterErr, err)
		}
	}
}

func raiseSignal(sig os.Signal) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(sig)
}
