package gorch

import (
	"errors"
	"os"
	"os/signal"
	"sync"
)

type optionFunc func(o *options)

type options struct {
	signals []os.Signal
}

type HookFunc func() error

func WithStopSignals(signals ...os.Signal) optionFunc {
	return func(o *options) {
		o.signals = append(o.signals, signals...)
	}
}

type Orchestrator struct {
	wg      sync.WaitGroup
	stopCh  chan os.Signal
	errorCh chan error
	before  []HookFunc
	after   []HookFunc
}

func New(optFns ...optionFunc) *Orchestrator {
	stopCh := make(chan os.Signal, 1)

	var opts options
	for i := range optFns {
		optFn := optFns[i]
		if optFn != nil {
			optFn(&opts)
		}
	}

	if len(opts.signals) == 0 {
		opts.signals = append(opts.signals, os.Interrupt)
	}
	signal.Notify(stopCh, opts.signals...)

	return &Orchestrator{
		wg:      sync.WaitGroup{},
		stopCh:  stopCh,
		errorCh: make(chan error, 1),
		before:  make([]HookFunc, 0),
		after:   make([]HookFunc, 0),
	}
}

func (o *Orchestrator) Before(bfr ...HookFunc) *Orchestrator {
	o.before = append(o.before, bfr...)
	return o
}

func (o *Orchestrator) After(aft ...HookFunc) *Orchestrator {
	o.after = append(o.after, aft...)
	return o
}

func (o *Orchestrator) StopChannel() <-chan os.Signal {
	return o.stopCh
}

func (o *Orchestrator) ErrorChannel() <-chan error {
	return o.errorCh
}

func (o *Orchestrator) Start(start HookFunc) error {
	if err := o.beforeHooks(); err != nil {
		return err
	}

	go func() {
		if err := start(); err != nil {
			o.errorCh <- err
		}
	}()

	errs := make([]error, 0)
	select {
	case <-o.stopCh:
	case err := <-o.errorCh:
		errs = append(errs, err)
	}

	for _, hook := range o.after {
		if err := hook(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (o *Orchestrator) StartAsync(start HookFunc) error {
	if err := o.beforeHooks(); err != nil {
		return err
	}

	o.wg.Add(1)
	go func() {
		errs := make([]error, 0)

		if err := start(); err != nil {
			errs = append(errs, err)
		}

		for _, hook := range o.after {
			if err := hook(); err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			o.errorCh <- errors.Join(errs...)
		}

		o.wg.Done()
	}()

	return nil
}

func (o *Orchestrator) Wait() {
	o.wg.Wait()
}

func (o *Orchestrator) beforeHooks() error {
	for _, hook := range o.before {
		if err := hook(); err != nil {
			return err
		}
	}
	return nil
}
