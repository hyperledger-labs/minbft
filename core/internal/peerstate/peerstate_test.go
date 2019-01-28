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

package peerstate

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/usig"
)

func TestCaptureReleaseUI(t *testing.T) {
	state := New()

	cases := []struct {
		desc string
		cv   int

		new bool
	}{{
		desc: "First valid UI",
		cv:   1,
		new:  true,
	}, {
		desc: "The same UI again",
		cv:   1,
		new:  false,
	}, {
		desc: "Second valid UI",
		cv:   2,
		new:  true,
	}, {
		desc: "Old UI",
		cv:   1,
		new:  false,
	}, {
		desc: "Third valid UI",
		cv:   3,
		new:  true,
	}}

	for _, c := range cases {
		ui := &usig.UI{Counter: uint64(c.cv)}
		new, release := state.CaptureUI(ui)
		require.Equal(t, c.new, new, c.desc)
		if new {
			go release()
		}
	}
}

func TestConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrUIs = 10

	uis := make([]*usig.UI, nrUIs)
	wg := new(sync.WaitGroup)

	state := New()

	wg.Add(nrConcurrent * nrUIs)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			for cv := 1; cv <= nrUIs; cv++ {
				assertMsg := fmt.Sprintf("Worker %d, UI %d", workerID, cv)
				cv := cv
				ui := &usig.UI{Counter: uint64(cv)}

				go func() {
					defer wg.Done()

					if new, release := state.CaptureUI(ui); new {
						assert.Nil(t, uis[cv-1], assertMsg)
						uis[cv-1] = ui
						release()
					} else {
						assert.Equal(t, ui, uis[cv-1], assertMsg)
					}
				}()
			}
		}()
	}

	wg.Wait()
}

func TestProviderConcurrent(t *testing.T) {
	const nrConcurrent = 10

	peerStates := make([]State, nrConcurrent)
	wg := new(sync.WaitGroup)

	providePeerState := NewProvider()

	wg.Add(nrConcurrent)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			defer wg.Done()

			assertMsg := fmt.Sprintf("Worker %d", workerID)
			state := providePeerState(uint32(workerID))
			if assert.NotNil(t, state, assertMsg) {
				peerStates[workerID] = state
			}
		}()
	}

	wg.Wait()

	for i, s1 := range peerStates {
		for j, s2 := range peerStates[i+1:] {
			if s1 == s2 && s1 != nil {
				t.Errorf("Peers %d and %d got the same instance", i, j)
			}
		}
	}
}
