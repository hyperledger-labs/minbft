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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger-labs/minbft/core/internal/timer"
	"github.com/hyperledger-labs/minbft/messages"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

var messageImpl = protobufMessages.NewImpl()

func TestReply(t *testing.T) {
	t.Run("Channel", testReplyChannel)
	t.Run("ChannelConcurrent", testReplyChannelConcurrent)
}

func testReplyChannel(t *testing.T) {
	s := newClientState(timer.Standard(), defaultTimeout, defaultTimeout)

	seq1 := uint64(1)
	seq2 := seq1 + 1
	rly1 := makeReply(seq1)
	rly2 := makeReply(seq2)

	ch1Seq1 := s.ReplyChannel(seq1)
	require.NotNil(t, ch1Seq1)

	s.AddReply(rly1)

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

	s.AddReply(rly2)

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

	s := newClientState(timer.Standard(), defaultTimeout, defaultTimeout)
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

		s.AddReply(rly)

		wg.Wait()
	}
}

func makeReply(seq uint64) messages.Reply {
	result := make([]byte, 1)
	rand.Read(result)
	return messageImpl.NewReply(rand.Uint32(), rand.Uint32(), seq, result)
}
