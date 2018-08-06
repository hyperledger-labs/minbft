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
	"math/rand"
	"sync"
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc string
		seq  int

		accept bool
		reply  bool

		ok  bool
		new bool
	}{{
		desc:   "Accept and reply the first ID",
		seq:    100,
		accept: true,
		reply:  true,
		ok:     true,
		new:    true,
	}, {
		desc:   "Accept another ID",
		seq:    200,
		accept: true,
		ok:     true,
		new:    true,
	}, {
		desc:   "Accept last replied ID before next reply",
		seq:    100,
		accept: true,
		new:    false,
	}, {
		desc:   "Accept last accepted ID before corresponding reply",
		seq:    200,
		accept: true,
		new:    false,
	}, {
		desc:   "Accept delayed older ID before next reply",
		seq:    150,
		accept: true,
		new:    false,
	}, {
		desc:  "Reply the last ID",
		seq:   200,
		reply: true,
		ok:    true,
	}, {
		desc:   "Accept last accepted and replied ID",
		seq:    100,
		accept: true,
		new:    false,
	}, {
		desc:   "Accept delayed older ID after reply",
		seq:    150,
		accept: true,
		new:    false,
	}}

	for _, c := range cases {
		seq := uint64(c.seq)
		if c.accept {
			new := s.AcceptRequestSeq(seq)
			require.Equal(t, c.new, new, c.desc)
		}
		if c.reply {
			err := s.AddReply(makeReply(seq))
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

func TestClientStateConcurrent(t *testing.T) {
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

		new := s.AcceptRequestSeq(uint64(seq))
		require.True(t, new)

		for workerID := 0; workerID < nrConcurrent; workerID++ {
			workerID := workerID
			wg.Add(1)
			ch := s.ReplyChannel(uint64(seq))

			go func() {
				defer wg.Done()
				assert.Equal(t, rly, <-ch, "seq=%d, worker=%d", seq, workerID)
			}()
		}

		go func() {
			err := s.AddReply(rly)
			assert.NoError(t, err, "seq=%d", seq)
		}()

		wg.Wait()
	}
}

func TestClientStateProvider(t *testing.T) {
	const nrConcurrent = 5
	const nrClientStates = 10

	provider := NewProvider()

	wg := new(sync.WaitGroup)
	wg.Add(nrConcurrent)
	for workerID := 0; workerID < nrConcurrent; workerID++ {
		workerID := workerID
		go func() {
			defer wg.Done()

			for clientID := 0; clientID < nrClientStates; clientID++ {
				state := provider(uint32(clientID))
				assert.NotNil(t, state,
					"workerID=%d, clientID=%d", workerID, clientID)
			}
		}()
	}

	wg.Wait()
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
