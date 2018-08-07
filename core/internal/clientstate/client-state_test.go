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

package clientstate

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureReleaseRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc string
		seq  int

		capture bool
		release bool

		ok  bool
		new bool
	}{{
		desc:    "Capture and release the first ID",
		seq:     100,
		capture: true,
		release: true,
		ok:      true,
		new:     true,
	}, {
		desc:    "Release the same ID again",
		seq:     100,
		release: true,
		ok:      false,
	}, {
		desc:    "Release new ID before capturing",
		seq:     200,
		release: true,
		ok:      false,
	}, {
		desc:    "Capture another ID",
		seq:     200,
		capture: true,
		ok:      true,
		new:     true,
	}, {
		desc:    "Capture older ID",
		seq:     50,
		capture: true,
		new:     false,
	}, {
		desc:    "Release not captured ID",
		seq:     150,
		release: true,
		ok:      false,
	}, {
		desc:    "Release last captured ID",
		seq:     200,
		release: true,
		ok:      true,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.capture {
			new := s.CaptureRequestSeq(seq)
			require.Equal(t, c.new, new, c.desc)
		}
		if c.release {
			err := s.ReleaseRequestSeq(seq)
			if c.ok {
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
		}
	}
}

func TestCaptureReleaseRequestSeqConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrSeqs = 10

	seqs := make([]bool, nrSeqs)
	wg := new(sync.WaitGroup)

	state := New()

	wg.Add(nrConcurrent)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			defer wg.Done()

			for seq := 1; seq <= nrSeqs; seq++ {
				assertMsg := fmt.Sprintf("Worker %d, seq %d", workerID, seq)

				if new := state.CaptureRequestSeq(uint64(seq)); new {
					assert.False(t, seqs[seq-1], assertMsg)
					seqs[seq-1] = true
					err := state.ReleaseRequestSeq(uint64(seq))
					assert.NoError(t, err, assertMsg)
				} else {
					assert.True(t, seqs[seq-1])
				}
			}
		}()
	}

	wg.Wait()
}

func TestPrepareRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc string
		seq  int

		release bool
		prepare bool

		ok bool
	}{{
		desc:    "Release and prepare first ID",
		seq:     100,
		release: true,
		prepare: true,
		ok:      true,
	}, {
		desc:    "Prepare the same ID",
		seq:     100,
		prepare: true,
		ok:      false,
	}, {
		desc:    "Prepare older ID",
		seq:     50,
		prepare: true,
		ok:      false,
	}, {
		desc:    "Prepare before release",
		seq:     200,
		prepare: true,
		ok:      false,
	}, {
		desc:    "Release and prepare another ID",
		seq:     200,
		release: true,
		prepare: true,
		ok:      true,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.release {
			new := s.CaptureRequestSeq(seq)
			require.True(t, new, c.desc)

			err := s.ReleaseRequestSeq(seq)
			require.NoError(t, err, c.desc)
		}
		if c.prepare {
			err := s.PrepareRequestSeq(seq)
			if c.ok {
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
		}
	}
}

func TestRetireRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc string
		seq  int

		prepare bool
		retire  bool

		ok bool
	}{{
		desc:    "Prepare and retire first ID",
		seq:     100,
		prepare: true,
		retire:  true,
		ok:      true,
	}, {
		desc:   "Retire the same ID",
		seq:    100,
		retire: true,
		ok:     false,
	}, {
		desc:   "Retire older ID",
		seq:    50,
		retire: true,
		ok:     false,
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
		ok:      true,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.prepare {
			new := s.CaptureRequestSeq(seq)
			require.True(t, new, c.desc)

			err := s.ReleaseRequestSeq(seq)
			require.NoError(t, err, c.desc)

			err = s.PrepareRequestSeq(seq)
			require.NoError(t, err, c.desc)
		}
		if c.retire {
			err := s.RetireRequestSeq(seq)
			if c.ok {
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
		}
	}
}

func TestAddReply(t *testing.T) {
	s := New()

	cases := []struct {
		desc string
		seq  int

		ok bool
	}{{
		desc: "Add first Reply",
		seq:  100,
		ok:   true,
	}, {
		desc: "Add duplicate Reply",
		seq:  100,
		ok:   false,
	}, {
		desc: "Add another Reply",
		seq:  200,
		ok:   true,
	}, {
		desc: "Add older Reply",
		seq:  150,
		ok:   false,
	}}

	for _, c := range cases {
		reply := makeReply(uint64(c.seq))
		err := s.AddReply(reply)
		if c.ok {
			require.NoError(t, err, c.desc)
		} else {
			require.Error(t, err, c.desc)
		}
	}
}

func TestReplyChannel(t *testing.T) {
	s := New()

	seq1 := uint64(1)
	seq2 := seq1 + 1
	rly1 := makeReply(seq1)
	rly2 := makeReply(seq2)

	ch1Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch1Seq1)

	err := s.AddReply(rly1)
	require.NoError(t, err)

	ch2Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch2Seq1)

	assert.Equal(t, rly1, <-ch1Seq1)
	assert.Equal(t, rly1, <-ch2Seq1)
	_, more := <-ch1Seq1
	assert.False(t, more, "Channel should be closed")
	_, more = <-ch2Seq1
	assert.False(t, more, "Channel should be closed")

	ch1Seq2 := s.ReplyChannel(seq2)
	require.NotNil(t, ch1Seq2)

	err = s.AddReply(rly2)
	require.NoError(t, err)

	ch3Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch3Seq1)

	_, more = <-ch3Seq1
	assert.False(t, more, "Channel should be closed")

	assert.Equal(t, rly2, <-ch1Seq2)
	_, more = <-ch1Seq2
	assert.False(t, more, "Channel should be closed")
}

func TestReplyChannelConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrRequests = 10

	replies := make([]*messages.Reply, nrRequests)
	for i := range replies {
		replies[i] = makeReply(uint64(i + 1))
	}

	s := New()
	wg := new(sync.WaitGroup)
	for _, rly := range replies {
		rly := rly
		seq := rly.Msg.Seq

		for workerID := 0; workerID < nrConcurrent; workerID++ {
			workerID := workerID
			wg.Add(1)
			ch := s.ReplyChannel(uint64(seq))

			go func() {
				defer wg.Done()
				assert.Equal(t, rly, <-ch, "seq=%d, worker=%d", seq, workerID)
			}()
		}

		err := s.AddReply(rly)
		assert.NoError(t, err, "seq=%d", seq)

		wg.Wait()
	}
}

func TestProviderConcurrent(t *testing.T) {
	const nrConcurrent = 10

	clientStates := make([]State, nrConcurrent)
	wg := new(sync.WaitGroup)

	provider := NewProvider()

	wg.Add(nrConcurrent)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID

		go func() {
			defer wg.Done()

			assertMsg := fmt.Sprintf("Worker %d", workerID)
			state := provider(uint32(workerID))
			if assert.NotNil(t, state, assertMsg) {
				clientStates[workerID] = state
			}
		}()
	}

	wg.Wait()

	for i, s1 := range clientStates {
		for j, s2 := range clientStates[i+1:] {
			if s1 == s2 && s1 != nil {
				t.Errorf("Clients %d and %d got the same instance", i, j)
			}
		}
	}
}

func makeReply(seq uint64) *messages.Reply {
	result := make([]byte, 1)
	rand.Read(result)
	return &messages.Reply{
		Msg: &messages.Reply_M{
			Seq:    seq,
			Result: result,
		},
	}
}
