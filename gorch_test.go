package gorch_test

import (
	"errors"
	"os"
	"reflect"
	"syscall"
	"testing"

	"github.com/snet-commerce/gorch"
)

var (
	controlSignal = os.Interrupt
	appErr        = errors.New("application error")
)

type application struct {
	stop chan struct{}
	err  chan error
}

func newApplication() *application {
	return &application{
		stop: make(chan struct{}, 1),
		err:  make(chan error, 1),
	}
}

func (a *application) Start() error {
	select {
	case <-a.stop:
		return nil
	case err := <-a.err:
		return err
	}
}

func (a *application) Stop() {
	a.stop <- struct{}{}
}

func (a *application) RaiseError() {
	a.err <- appErr
}

func TestOrchestrator_Start(t *testing.T) {
	const maskLen = 3

	app := newApplication()
	o := gorch.New(gorch.WithStopSignals(controlSignal))

	bfrMask, aftMask := initMasks(maskLen)
	expectedMask := []byte{1, 1, 1}

	hookFunc := func(i int, mask []byte) gorch.HookFunc {
		return func() error {
			mask[i] = 1
			return nil
		}
	}

	for i := 0; i < maskLen; i++ {
		o.Before(hookFunc(i, bfrMask)).After(hookFunc(i, aftMask))
	}

	// application must be stopped when signal is sent
	o.After(func() error {
		app.Stop()
		return nil
	})

	// raise control signal to stop pipeline
	if err := raiseSignal(controlSignal); err != nil {
		t.Fatalf("failed to sent stop signal: %s", err)
	}

	// no errors must be raised
	err := o.Start(func() error { return app.Start() })
	if err != nil {
		t.Fatalf("no error must be raised after execution of application pipeline, but error occurred: %s", err)
	}

	// verify mask is set
	if !reflect.DeepEqual(expectedMask, bfrMask) || !reflect.DeepEqual(expectedMask, aftMask) {
		t.Fatal("no error occurred, but not all before or after hooks were executed")
	}

	// zero mask
	zeroMask(bfrMask)
	zeroMask(aftMask)

	// raise error now, so we must get it after pipeline
	app.RaiseError()
	if err = o.Start(func() error { return app.Start() }); !errors.Is(err, appErr) {
		t.Fatalf("application error must be raised, but got: %s", err)
	}

	// verify mask is set
	if !reflect.DeepEqual(expectedMask, aftMask) {
		t.Fatal("even though error occurred not all after hooks were executed")
	}
}

func TestOrchestrator_StartAsync(t *testing.T) {
	const maskLen = 3

	app := newApplication()
	o := gorch.New(gorch.WithStopSignals(controlSignal))
	bfrMask, aftMask := initMasks(maskLen)
	expectedMask := []byte{1, 1, 1}

	hookFunc := func(i int, mask []byte) gorch.HookFunc {
		return func() error {
			mask[i] = 1
			return nil
		}
	}

	for i := 0; i < maskLen; i++ {
		o.Before(hookFunc(i, bfrMask)).After(hookFunc(i, aftMask))
	}

	// no errors must be raised
	if err := o.StartAsync(func() error { return app.Start() }); err != nil {
		t.Fatalf("no error must be raised on startup of application pipeline: %s", err)
	}

	// raise control signal to stop pipeline
	if err := raiseSignal(controlSignal); err != nil {
		t.Fatalf("failed to sent stop signal: %s", err)
	}

	// expect stop signal to be sent
	select {
	case <-o.StopChannel():
		app.Stop()
	case err := <-o.ErrorChannel():
		t.Fatalf("no error must be raised but got %s", err)
	}
	o.Wait()

	// verify mask is set
	if !reflect.DeepEqual(expectedMask, bfrMask) || !reflect.DeepEqual(expectedMask, aftMask) {
		t.Fatal("no error occurred, but not all before or after hooks were executed")
	}

	// zero mask
	zeroMask(bfrMask)
	zeroMask(aftMask)

	// raise error now, so we must get it after pipeline
	app.RaiseError()
	if err := o.StartAsync(func() error { return app.Start() }); err != nil {
		t.Fatalf("no error must be raised on startup of application pipeline: %s", err)
	}

	// expect stop signal to be sent
	select {
	case <-o.StopChannel():
		t.Fatal("no stop signal must be sent")
	case err := <-o.ErrorChannel():
		if !errors.Is(err, appErr) {
			t.Fatalf("application error must be raised, but got: %s", err)
		}
	}
	o.Wait()

	// verify mask is set
	if !reflect.DeepEqual(expectedMask, aftMask) {
		t.Fatal("even though error occurred not all after hooks were executed")
	}
}

func initMasks(length int) (bfr, aft []byte) {
	return make([]byte, length), make([]byte, length)
}

func zeroMask(mask []byte) {
	for i := range mask {
		mask[i] = 0
	}
}

func raiseSignal(sig os.Signal) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(sig)
}
