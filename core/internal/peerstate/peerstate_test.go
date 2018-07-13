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

	"github.com/nec-blockchain/minbft/usig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureReleaseUI(t *testing.T) {
	state := New()

	cases := []struct {
		desc string
		cv   int

		ok  bool
		new bool

		capture bool
		release bool
	}{{
		desc:    "First valid UI",
		cv:      1,
		ok:      true,
		new:     true,
		capture: true,
	}, {
		desc:    "The same UI again before releasing",
		cv:      1,
		ok:      true,
		new:     false,
		capture: true,
	}, {
		desc:    "Release first valid UI",
		cv:      1,
		ok:      true,
		release: true,
	}, {
		desc:    "The same UI again after releasing",
		cv:      1,
		ok:      true,
		new:     false,
		capture: true,
	}, {
		desc:    "Duplicate release",
		cv:      1,
		ok:      false,
		release: true,
	}, {
		desc:    "Second valid UI",
		cv:      2,
		ok:      true,
		new:     true,
		capture: true,
		release: true,
	}, {
		desc:    "Release old UI",
		cv:      1,
		ok:      false,
		release: true,
	}, {
		desc:    "Capture old UI",
		cv:      1,
		ok:      true,
		new:     false,
		capture: true,
	}, {
		desc:    "Third valid UI",
		cv:      3,
		ok:      true,
		new:     true,
		capture: true,
		release: true,
	}}

	for _, c := range cases {
		ui := &usig.UI{Counter: uint64(c.cv)}
		if c.capture {
			assertMsg := fmt.Sprintf("CaptureUI: %s", c.desc)
			new := state.CaptureUI(ui)
			require.Equal(t, c.new, new, assertMsg)
		}
		if c.release {
			assertMsg := fmt.Sprintf("ReleaseUI: %s", c.desc)
			err := state.ReleaseUI(ui)
			if c.ok {
				require.NoError(t, err, assertMsg)
			} else {
				require.Error(t, err, assertMsg)
			}
		}
	}
}

func TestConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrUIs = 10

	wg := new(sync.WaitGroup)

	providePeerState := NewProvider()

	wg.Add(nrConcurrent * nrUIs)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			state := providePeerState(uint32(workerID))

			for cv := 1; cv <= nrUIs; cv++ {
				ui := &usig.UI{Counter: uint64(cv)}
				assertMsg := fmt.Sprintf("Worker %d, UI %d", workerID, cv)

				go func() {
					defer wg.Done()

					new := state.CaptureUI(ui)
					if assert.True(t, new, assertMsg) {
						err := state.ReleaseUI(ui)
						assert.NoError(t, err, assertMsg)
					}
				}()
			}
		}()
	}

	wg.Wait()
}
