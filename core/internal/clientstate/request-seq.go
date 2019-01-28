// Copyright (c) 2018 NEC Laboratories Europe GmbH.
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
	"fmt"
	"sync"
)

type seqState struct {
	sync.Mutex

	// Last captured request ID
	lastCapturedSeq uint64

	// Last released request ID
	lastReleasedSeq uint64

	// Cond to signal on when ID is released
	seqReleased *sync.Cond

	// Last prepared request ID
	lastPreparedSeq uint64

	// Last retired request ID
	lastRetiredSeq uint64
}

func newSeqState() *seqState {
	s := &seqState{}
	s.seqReleased = sync.NewCond(s)
	return s
}

func (s *seqState) CaptureRequestSeq(seq uint64) (new bool, release func()) {
	s.Lock()
	defer s.Unlock()

	for {
		if seq <= s.lastCapturedSeq {
			// The request ID is not new.
			// Wait until it gets released.
			for seq > s.lastReleasedSeq {
				s.seqReleased.Wait()
			}

			return false, nil
		}

		// The request ID might be new. Check if the greatest
		// captured ID got released.
		if s.lastCapturedSeq > s.lastReleasedSeq {
			// The greatest captured ID is not released.
			// Wait for it to get released and try again.
			s.seqReleased.Wait()

			continue
		}

		s.lastCapturedSeq = seq

		return true, func() {
			s.Lock()
			defer s.Unlock()

			s.lastReleasedSeq = s.lastCapturedSeq
			s.seqReleased.Broadcast()
		}
	}
}

func (s *seqState) PrepareRequestSeq(seq uint64) (new bool, err error) {
	s.Lock()
	defer s.Unlock()

	if seq <= s.lastPreparedSeq {
		return false, nil
	} else if seq > s.lastCapturedSeq {
		return false, fmt.Errorf("Request ID not captured")
	}

	s.lastPreparedSeq = seq

	return true, nil
}

func (s *seqState) RetireRequestSeq(seq uint64) (new bool, err error) {
	s.Lock()
	defer s.Unlock()

	if seq <= s.lastRetiredSeq {
		return false, nil
	} else if seq > s.lastPreparedSeq {
		return false, fmt.Errorf("Request ID not prepared")
	}

	s.lastRetiredSeq = seq

	return true, nil
}
