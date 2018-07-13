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

// Package peerstate provides means to interact with a representation
// of the state maintained by the replica for each peer replica.
package peerstate

import (
	"fmt"
	"sync"

	"github.com/nec-blockchain/minbft/usig"
)

// Provider returns an instance of state representation associated
// with a peer replica given its ID. It is safe to invoke
// concurrently.
type Provider func(replicaID uint32) State

// NewProvider creates an instance of Provider
func NewProvider() Provider {
	var (
		lock sync.Mutex
		// Replica ID -> replica state
		peerStates = make(map[uint32]State)
	)

	return func(replicaID uint32) State {
		lock.Lock()
		defer lock.Unlock()

		state := peerStates[replicaID]
		if state == nil {
			state = New()
			peerStates[replicaID] = state
		}

		return state
	}
}

// State represents the state maintained by the replica for each peer
// replica. All methods are safe to invoke concurrently.
//
// CaptureUI captures a valid USIG unique identifier ui. A UI cannot
// be captured until all previous UIs have been captured and released.
// If the UI cannot be captured immediately, it will block until the
// UI can be captured. The return value new indicates if the UI has
// not been captured before.
//
// ReleaseUI releases the last captured USIG unique identifier ui so
// that the next UI can be captured. It is an error attempting to
// release the same UI more than once.
type State interface {
	CaptureUI(ui *usig.UI) (new bool)
	ReleaseUI(ui *usig.UI) error
}

// New creates a new instance of peer replica state representation.
func New() State {
	state := &peerState{}
	state.released = sync.NewCond(state)
	return state
}

type peerState struct {
	sync.Mutex
	lastCapturedCV uint64
	lastReleasedCV uint64
	released       *sync.Cond
}

func (s *peerState) CaptureUI(ui *usig.UI) (new bool) {
	s.Lock()
	defer s.Unlock()

	for ui.Counter-1 > s.lastReleasedCV {
		s.released.Wait()
	}

	if ui.Counter <= s.lastCapturedCV {
		return false
	}

	s.lastCapturedCV++

	return true
}

func (s *peerState) ReleaseUI(ui *usig.UI) error {
	s.Lock()
	defer s.Unlock()

	if ui.Counter != s.lastCapturedCV {
		return fmt.Errorf("UI is not the last captured")
	} else if ui.Counter <= s.lastReleasedCV {
		return fmt.Errorf("UI already released")
	}

	s.lastReleasedCV = ui.Counter
	s.released.Broadcast()

	return nil
}
