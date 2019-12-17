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

type timerState struct {
	sync.Mutex

	timerProvider timer.Provider
	timeout       func() time.Duration
	timer         timer.Timer
	seq           uint64
}

func newTimerState(timerProvider timer.Provider, timeout func() time.Duration) *timerState {
	return &timerState{
		timerProvider: timerProvider,
		timeout:       timeout,
	}
}

func (s *timerState) StartTimer(seq uint64, handleTimeout func()) {
	s.Lock()
	defer s.Unlock()

	if s.timer != nil {
		s.timer.Stop()
	}

	s.seq = seq

	timeout := s.timeout()
	if timeout <= time.Duration(0) {
		return
	}

	s.timer = s.timerProvider.AfterFunc(timeout, handleTimeout)
}

func (s *timerState) StopTimer(seq uint64) {
	s.Lock()
	defer s.Unlock()

	if s.seq != seq {
		return
	}

	if s.timer == nil {
		return
	}

	s.timer.Stop()
}
