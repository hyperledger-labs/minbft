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
	"math/rand"
	"sync"
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

var messageImpl = protobufMessages.NewImpl()

func TestReply(t *testing.T) {
	t.Run("Add", testAddReply)
	t.Run("Channel", testReplyChannel)
	t.Run("ChannelConcurrent", testReplyChannelConcurrent)
}

func testAddReply(t *testing.T) {
	s := New(defaultTimeout, defaultTimeout)

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

func testReplyChannel(t *testing.T) {
	s := New(defaultTimeout, defaultTimeout)

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

func testReplyChannelConcurrent(t *testing.T) {
	const nrConcurrent = 5
	const nrRequests = 10

	replies := make([]messages.Reply, nrRequests)
	for i := range replies {
		replies[i] = makeReply(uint64(i + 1))
	}

	s := New(defaultTimeout, defaultTimeout)
	wg := new(sync.WaitGroup)
	for _, rly := range replies {
		rly := rly
		seq := rly.Sequence()

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

func makeReply(seq uint64) messages.Reply {
	result := make([]byte, 1)
	rand.Read(result)
	return messageImpl.NewReply(rand.Uint32(), rand.Uint32(), seq, result)
}
