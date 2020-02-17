// Copyright (c) 2022 NEC Laboratories Europe GmbH.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backofftimer

import (
	"time"

	timerImpl "github.com/hyperledger-labs/minbft/core/internal/timer"
)

// Timer represents a back-off timer.
//
// Start method starts/restarts the timer. The supplied function is
// invoked when the timer expires. Each subsequent restart increases
// the timeout duration.
//
// Stop method stops the timer and resets the timeout duration back to
// the initial value.
type Timer interface {
	Start(f func())
	Stop()
}

// Option represents an optional parameter.
type Option func(*options)

type options struct {
	timerProvider timerImpl.Provider
}

var defaultOptions = options{
	timerProvider: timerImpl.Standard(),
}

// WithTimerProvider specifies an abstract timer implementation to
// use. Standard timer implementation is used by default.
func WithTimerProvider(timerProvider timerImpl.Provider) Option {
	return func(opts *options) {
		opts.timerProvider = timerProvider
	}
}

type timer struct {
	options

	initTimeout    time.Duration
	currentTimeout time.Duration

	timer timerImpl.Timer
}

func New(d time.Duration, args ...Option) Timer {
	opts := defaultOptions
	for _, opt := range args {
		opt(&opts)
	}

	return &timer{
		options:        opts,
		initTimeout:    d,
		currentTimeout: d,
	}
}

func (t *timer) Start(f func()) {
	t.timer = t.timerProvider.AfterFunc(t.currentTimeout, f)
	t.currentTimeout *= 2
}

func (t *timer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}

	t.currentTimeout = t.initTimeout
}
