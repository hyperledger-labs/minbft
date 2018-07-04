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
	"sync"
	"testing"

	"github.com/nec-blockchain/minbft/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc   string
		seq    int
		ok     bool
		new    bool
		accept int // need to accept the ID
		reply  int // need to reply the ID
	}{{
		desc: "First valid new ID",
		seq:  1,
		ok:   true,
		new:  true,
	}, {
		desc: "Another valid new ID",
		seq:  2,
		ok:   true,
		new:  true,
	}, {
		desc:   "Accept the first ID",
		accept: 1,
	}, {
		desc: "Another ID before Reply",
		seq:  2,
		ok:   true,
		new:  true,
	}, {
		desc: "Last accepted ID before Reply",
		seq:  1,
		ok:   true,
		new:  false,
	}, {
		desc:  "Reply the last accepted ID",
		reply: 1,
	}, {
		desc: "Last accepted ID after Reply",
		seq:  1,
		ok:   true,
		new:  false,
	}, {
		desc: "Another valid new ID",
		seq:  2,
		ok:   true,
		new:  true,
	}, {
		desc:   "Accept another ID",
		accept: 2,
	}, {
		desc: "No longer valid ID",
		seq:  1,
		ok:   false,
	}}

	for _, c := range cases {
		switch {
		case c.accept != 0:
			_, err := s.AcceptRequestSeq(uint64(c.accept))
			require.NoError(t, err, c.desc)
		case c.reply != 0:
			err := s.AddReply(makeReply(uint64(c.reply)))
			require.NoError(t, err, c.desc)
		case c.seq != 0:
			new, err := s.CheckRequestSeq(uint64(c.seq))
			if c.ok {
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
			require.Equal(t, c.new, new, c.desc)
		}

	}
}

func TestAcceptRequestSeq(t *testing.T) {
	s := New()

	cases := []struct {
		desc  string
		seq   int
		ok    bool
		new   bool
		reply int // need to reply the ID
	}{{
		desc: "First ID to accept",
		seq:  1,
		ok:   true,
		new:  true,
	}, {
		desc:  "Reply last accepted ID",
		reply: 1,
	}, {
		desc: "Last accepted ID",
		seq:  1,
		ok:   true,
		new:  false,
	}, {
		desc: "One more ID to accept",
		seq:  2,
		ok:   true,
		new:  true,
	}, {
		desc: "No longer valid ID",
		seq:  1,
		ok:   false,
	}}

	for _, c := range cases {
		switch {
		case c.reply != 0:
			err := s.AddReply(makeReply(uint64(c.reply)))
			require.NoError(t, err, c.desc)
			continue
		default:
			new, err := s.AcceptRequestSeq(uint64(c.seq))
			if c.ok {
				require.NoError(t, err, c.desc)
			} else {
				require.Error(t, err, c.desc)
			}
			require.Equal(t, c.new, new, c.desc)
		}
	}
}

func TestAddReply(t *testing.T) {
	s := New()

	seq1 := uint64(1)
	rly1 := makeReply(seq1)

	err := s.AddReply(rly1)
	assert.Error(t, err, "No accepted request ID")

	new, err := s.CheckRequestSeq(seq1)
	require.NoError(t, err)
	require.True(t, new)

	err = s.AddReply(rly1)
	assert.Error(t, err, "Checked, but not accepted request ID")

	new, err = s.AcceptRequestSeq(seq1)
	require.NoError(t, err)
	require.True(t, new)

	seq2 := seq1 + 1
	rly2 := makeReply(seq2)

	accepted := make(chan struct{})
	go func() {
		new, err := s.AcceptRequestSeq(seq2) // nolint: vetshadow
		assert.NoError(t, err, "Waiting for Reply before accepting next ID")
		assert.True(t, new)
		close(accepted)
	}()

	err = s.AddReply(rly2)
	assert.Error(t, err, "Request ID mismatch")

	err = s.AddReply(rly1)
	require.NoError(t, err, "Accepted request ID")

	err = s.AddReply(rly1)
	assert.Error(t, err, "Already supplied Reply")

	<-accepted
}

func TestReplyChannel(t *testing.T) {
	s := New()

	seq1 := uint64(1)
	seq2 := seq1 + 1
	rly1 := makeReply(seq1)
	rly2 := makeReply(seq2)

	new, err := s.AcceptRequestSeq(seq1)
	require.NoError(t, err)
	require.True(t, new)

	ch1Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch1Seq1)

	err = s.AddReply(rly1)
	require.NoError(t, err)

	ch2Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch2Seq1)

	assert.Equal(t, rly1, <-ch1Seq1)
	assert.Equal(t, rly1, <-ch2Seq1)
	_, more := <-ch1Seq1
	assert.False(t, more, "Channel should be closed")
	_, more = <-ch2Seq1
	assert.False(t, more, "Channel should be closed")

	new, err = s.AcceptRequestSeq(seq2)
	require.NoError(t, err)
	require.True(t, new)

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
		assertMsg := fmt.Sprintf("seq=%d", seq)

		new, err := s.AcceptRequestSeq(uint64(seq))
		require.NoError(t, err, assertMsg)
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
	return &messages.Reply{
		Msg: &messages.Reply_M{
			Seq: seq,
		},
	}
}
