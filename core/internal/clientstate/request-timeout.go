// Copyright (c) 2018-2019 NEC Laboratories Europe GmbH.
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

package clientstate

import (
	"sync"
	"time"

	"github.com/hyperledger-labs/minbft/core/internal/timer"
)

type requestTimerState struct {
	sync.Mutex

	timerProvider timer.Provider

	// Request timeout timer
	requestTimer   timer.Timer
	requestTimeout func() time.Duration
}

func newRequestTimeoutState(timerProvider timer.Provider, requestTimeout func() time.Duration) *requestTimerState {
	return &requestTimerState{
		timerProvider:  timerProvider,
		requestTimeout: requestTimeout,
	}
}

func (s *requestTimerState) StartRequestTimer(handleTimeout func()) {
	s.Lock()
	defer s.Unlock()

	timerProvider := s.timerProvider
	timeout := s.requestTimeout()

	if s.requestTimer != nil {
		s.requestTimer.Stop()
	}

	if timeout <= time.Duration(0) {
		return
	}

	s.requestTimer = timerProvider.AfterFunc(timeout, handleTimeout)
}

func (s *requestTimerState) StopRequestTimer() {
	s.Lock()
	defer s.Unlock()

	if s.requestTimer == nil {
		return
	}

	s.requestTimer.Stop()
}
