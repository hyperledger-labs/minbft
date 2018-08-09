// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

// Package timer provides abstract interfaces to represent an event of
// elapsed time. It also provides implementation of those interfaces
// using Timer type from the standard time package. These interfaces
// make it easier to implement unit tests for components that operate
// with timeout events.
package timer

import (
	"fmt"
	"time"
)

// Timer is an interface to manipulate with an event of elapsed time.
type Timer interface {
	// Expired returns a channel to receive the current time when
	// the timer expires.
	Expired() <-chan time.Time

	// Reset reschedules the timer to expire after duration d. It
	// should only be used on an inactive timer.
	Reset(d time.Duration)

	// Stop cancels the timer. It returns true if the timer has
	// been stopped by this invocation before expiration.
	Stop() bool
}

// Provider is an interface of abstract timer implementation.
type Provider interface {
	// NewTimer creates a new Timer that is to expire at least
	// after duration d.
	NewTimer(d time.Duration) Timer

	// AfterFunc creates a new Timer that is to expire at least
	// after duration d. The supplied callback function f is
	// invoked in a spawned goroutine upon the timer expiration.
	AfterFunc(d time.Duration, f func()) Timer
}

// Standard returns standard timer implementation.
func Standard() Provider {
	return stdProvider{}
}

type stdProvider struct{}

func (stdProvider) NewTimer(d time.Duration) Timer {
	return &stdTimer{time.NewTimer(d)}
}

func (stdProvider) AfterFunc(d time.Duration, f func()) Timer {
	return &stdTimer{time.AfterFunc(d, f)}
}

type stdTimer struct{ *time.Timer }

func (t stdTimer) Expired() <-chan time.Time {
	return t.C
}

func (t stdTimer) Reset(d time.Duration) {
	if t.Stop() {
		panic(fmt.Errorf("Resetting active timer"))
	}
	t.Timer.Reset(d)
}

func (t stdTimer) Stop() bool {
	return t.Timer.Stop()
}
