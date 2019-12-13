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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReqeustSeq(t *testing.T) {
	t.Run("CaptureRelease", testCaptureReleaseRequestSeq)
	t.Run("CaptureReleaseConcurrent", testCaptureReleaseRequestSeqConcurrent)
	t.Run("Prepare", testPrepareRequestSeq)
	t.Run("Retire", testRetireRequestSeq)
}

func testCaptureReleaseRequestSeq(t *testing.T) {
	s := New(defaultTimeout, defaultTimeout)

	cases := []struct {
		desc string
		seq  int

		new bool
	}{{
		desc: "Capture first ID",
		seq:  100,
		new:  true,
	}, {
		desc: "Capture another ID",
		seq:  200,
		new:  true,
	}, {
		desc: "Capture older ID",
		seq:  50,
		new:  false,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		new, release := s.CaptureRequestSeq(seq)
		require.Equal(t, c.new, new, c.desc)
		if new {
			go release()
		}
	}
}

func testCaptureReleaseRequestSeqConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrSeqs = 10

	seqs := make([]bool, nrSeqs)
	wg := new(sync.WaitGroup)

	state := New(defaultTimeout, defaultTimeout)

	wg.Add(nrConcurrent)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			defer wg.Done()

			for seq := 1; seq <= nrSeqs; seq++ {
				assertMsg := fmt.Sprintf("Worker %d, seq %d", workerID, seq)

				if new, release := state.CaptureRequestSeq(uint64(seq)); new {
					assert.False(t, seqs[seq-1], assertMsg)
					seqs[seq-1] = true
					release()
				} else {
					assert.True(t, seqs[seq-1])
				}
			}
		}()
	}

	wg.Wait()
}

func testPrepareRequestSeq(t *testing.T) {
	s := New(defaultTimeout, defaultTimeout)

	cases := []struct {
		desc string
		seq  int

		capture bool
		prepare bool

		new bool
		ok  bool
	}{{
		desc:    "Capture and prepare first ID",
		seq:     100,
		capture: true,
		prepare: true,
		new:     true,
		ok:      true,
	}, {
		desc:    "Prepare the same ID",
		seq:     100,
		prepare: true,
		new:     false,
		ok:      true,
	}, {
		desc:    "Prepare older ID",
		seq:     50,
		prepare: true,
		new:     false,
		ok:      true,
	}, {
		desc:    "Prepare before capture",
		seq:     200,
		prepare: true,
		ok:      false,
	}, {
		desc:    "Capture and prepare another ID",
		seq:     200,
		capture: true,
		prepare: true,
		new:     true,
		ok:      true,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.capture {
			new, release := s.CaptureRequestSeq(seq)
			require.True(t, new, c.desc)
			go release()
		}
		if c.prepare {
			new, err := s.PrepareRequestSeq(seq)
			if c.ok {
				require.Equal(t, c.new, new)
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
		}
	}
}

func testRetireRequestSeq(t *testing.T) {
	s := New(defaultTimeout, defaultTimeout)

	cases := []struct {
		desc string
		seq  int

		prepare bool
		retire  bool

		new bool
		ok  bool
	}{{
		desc:    "Prepare and retire first ID",
		seq:     100,
		prepare: true,
		retire:  true,
		new:     true,
		ok:      true,
	}, {
		desc:   "Retire the same ID",
		seq:    100,
		retire: true,
		new:    false,
		ok:     true,
	}, {
		desc:   "Retire older ID",
		seq:    50,
		retire: true,
		new:    false,
		ok:     true,
	}, {
		desc:   "Retire before prepare",
		seq:    200,
		retire: true,
		ok:     false,
	}, {
		desc:    "Prepare and retire another ID",
		seq:     200,
		prepare: true,
		retire:  true,
		new:     true,
		ok:      true,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.prepare {
			new, release := s.CaptureRequestSeq(seq)
			require.True(t, new, c.desc)
			go release()

			new, err := s.PrepareRequestSeq(seq)
			require.True(t, new, c.desc)
			require.NoError(t, err, c.desc)
		}
		if c.retire {
			new, err := s.RetireRequestSeq(seq)
			if c.ok {
				require.Equal(t, c.new, new)
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
		}
	}
}
