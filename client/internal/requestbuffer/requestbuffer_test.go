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

package requestbuffer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

func TestAddRequest(t *testing.T) {
	rb := New()

	seq0 := uint64(0)
	req0 := makeRequest(seq0)
	_, ok := rb.AddRequest(req0)
	assert.False(t, ok, "Request with zero sequence ID must be rejected")

	seq1 := seq0 + 1
	req1 := makeRequest(seq1)
	ch, ok := rb.AddRequest(req1)
	require.True(t, ok)
	assert.NotNil(t, ch)

	rb.RemoveRequest(seq1)

	_, ok = rb.AddRequest(req1)
	assert.False(t, ok, "Subsequent Request sequence ID cannot be the same")

	seq2 := seq1 + 1
	req2 := makeRequest(seq2)
	ch, ok = rb.AddRequest(req2)
	require.True(t, ok)
	assert.NotNil(t, ch)

	rb.RemoveRequest(seq2)

	_, ok = rb.AddRequest(req1)
	assert.False(t, ok, "Subsequent Request sequence ID cannot decrease")
}

func TestRemoveRequest(t *testing.T) {
	rb := New()

	seq1 := uint64(1)
	rb.RemoveRequest(seq1) // no panic

	req1 := makeRequest(seq1)
	ch1, ok := rb.AddRequest(req1)
	require.True(t, ok)

	seq2 := seq1 + 1
	rb.RemoveRequest(seq2)
	select {
	case _, more := <-ch1:
		assert.True(t, more, "Must not close Reply channel after trying to remove wrong Request")
	default:
	}

	rb.RemoveRequest(seq1)
	_, more := <-ch1
	assert.False(t, more, "Must close Reply channel after Request removal")
}

func TestAddReply(t *testing.T) {
	rb := New()

	seq1 := uint64(1)
	rly1r0 := makeReply(uint32(0), seq1)
	ok := rb.AddReply(rly1r0)
	assert.NotNil(t, ok, "Must drop Reply with empty buffer")

	req1 := makeRequest(seq1)
	ch, ok := rb.AddRequest(req1)
	require.True(t, ok)

	seq2 := seq1 + 1
	rly2r0 := makeReply(uint32(0), seq2)
	ok = rb.AddReply(rly2r0)
	assert.NotNil(t, ok, "Must drop Reply with no corresponding Request")

	rly1r1 := makeReply(uint32(1), seq1)

	go func() {
		ok := rb.AddReply(rly1r0) // nolint: vetshadow
		assert.True(t, ok)
		ok = rb.AddReply(rly1r1)
		assert.True(t, ok)
	}()

	assert.Equal(t, rly1r0, <-ch, "Must receive added Reply")
	assert.Equal(t, rly1r1, <-ch, "Must receive added Reply")

	ok = rb.AddReply(rly1r0)
	assert.False(t, ok, "Must drop another Reply from the same replica")

	rb.RemoveRequest(seq1)

	ok = rb.AddReply(rly1r0)
	assert.NotNil(t, ok, "Must drop Reply after Request removal")
}

func TestRequestStream(t *testing.T) {
	rb := New()

	doneChan := make(chan struct{})
	requestChan := rb.RequestStream(doneChan)

	seq := uint64(1)
	req := makeRequest(seq)

	go func() {
		assert.Equal(t, req, <-requestChan)
		close(doneChan)
	}()

	_, ok := rb.AddRequest(req)
	require.True(t, ok)

	<-doneChan
	_, more := <-requestChan
	assert.False(t, more, "Must close Request channel after closed done channel")
}

func TestConcurrent(t *testing.T) {
	const nrReplicas = 3
	const nrRequests = 5

	rb := New()

	wg := new(sync.WaitGroup)
	wg.Add(nrReplicas)
	for id := uint32(0); id < nrReplicas; id++ {
		ch := rb.RequestStream(nil)
		go func(id uint32) {
			defer wg.Done()
			for seq := uint64(1); seq <= nrRequests; seq++ {
				req := makeRequest(seq)
				assert.Equal(t, req, <-ch)

				rly := makeReply(id, seq)
				ok := rb.AddReply(rly)
				assert.True(t, ok)
			}
		}(id)
	}

	wg.Add(nrRequests)
	for seq := uint64(1); seq <= nrRequests; seq++ {
		req := makeRequest(seq)
		ch, ok := rb.AddRequest(req)
		assert.True(t, ok)
		assert.NotNil(t, ch)

		go func(seq uint64) {
			defer wg.Done()
			expectedReplies := make(map[uint32]*messages.Reply)
			receivedReplies := make(map[uint32]*messages.Reply)
			for id := uint32(0); id < nrReplicas; id++ {
				expectedReplies[id] = makeReply(id, seq)
				rly := <-ch
				receivedReplies[rly.Msg.ReplicaId] = rly
			}

			assert.Equal(t, expectedReplies, receivedReplies)
			rb.RemoveRequest(seq)

			_, more := <-ch
			assert.False(t, more)
		}(seq)
	}

	wg.Wait()
}

func TestWithFaulty(t *testing.T) {
	const nrReplicas = 5
	const nrFaulty = 2
	const nrRequests = 10

	rb := New()

	done := make(chan struct{})
	for id := uint32(0); id < nrReplicas; id++ {
		ch := rb.RequestStream(done)
		if id < nrFaulty {
			continue
		}
		go func(id uint32) {
			for req := range ch {
				rly := makeReply(id, req.Msg.Seq)
				_ = rb.AddReply(rly)
			}
		}(id)
	}

	wg := new(sync.WaitGroup)
	wg.Add(nrRequests)
	for seq := uint64(1); seq <= nrRequests; seq++ {
		req := makeRequest(seq)
		ch, ok := rb.AddRequest(req)
		require.True(t, ok)

		go func(seq uint64) {
			defer wg.Done()
			for i := uint32(0); i < nrReplicas-nrFaulty; i++ {
				<-ch
			}

			rb.RemoveRequest(seq)
		}(seq)
	}

	wg.Wait()
	close(done)
}

func makeRequest(seq uint64) *messages.Request {
	return &messages.Request{
		Msg: &messages.Request_M{
			Seq: seq,
		},
	}
}

func makeReply(replicaID uint32, seq uint64) *messages.Reply {
	return &messages.Reply{
		Msg: &messages.Reply_M{
			ReplicaId: replicaID,
			Seq:       seq,
		},
	}
}
