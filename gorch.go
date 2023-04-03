package gorch

import (
	"errors"
	"os"
	"os/signal"
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
	signals []os.Signal
	before  []HookFunc
	after   []HookFunc
}

func New(optFns ...optionFunc) *Orchestrator {
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

	return &Orchestrator{
		signals: opts.signals,
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

func (o *Orchestrator) Serve(starters ...HookFunc) <-chan error {
	errCh := make(chan error, 1)
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, o.signals...)

	if err := o.beforeHooks(); err != nil {
		errCh <- err
		close(errCh)
		return errCh
	}

	go func() {
		for i := range starters {
			startFn := starters[i]
			go func(start HookFunc) {
				if err := start(); err != nil {
					errCh <- err
				}
			}(startFn)
		}

		<-stopCh
		if errs := o.afterHooks(); len(errs) > 0 {
			errCh <- errors.Join(errs...)
		}

		close(errCh)
	}()

	return errCh
}

func (o *Orchestrator) beforeHooks() error {
	for _, hook := range o.before {
		if err := hook(); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) afterHooks() []error {
	errs := make([]error, 0)
	for _, hook := range o.after {
		if err := hook(); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
