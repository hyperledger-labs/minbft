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

// Package requestbuffer provides a storage for a consistent set of
// Request and corresponding Reply messages. It allows to
// asynchronously add and remove messages, as well as to subscribe for
// new messages.
package requestbuffer

import (
	"sync"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

// T implements the storage to keep and coordinate flow and processing
// of Request and Reply messages. All methods are safe to invoke
// concurrently. Current implementation has capacity for a single
// request only.
type T struct {
	lock sync.RWMutex

	// Sequence ID of the last added Request message
	lastSeq uint64

	// The only request to hold in the buffer
	request *request

	// List of buffered channels to notify about new messages
	newAdded []chan<- struct{}
}

// New creates a new instance of the request buffer
func New() *T {
	return &T{}
}

// AddRequest adds a new Request message to the buffer. Each
// subsequent invocation should supply a message with increasing
// non-zero sequence ID. It will block if there's no capacity
// available for the new message. The corresponding Reply messages are
// to be received from the returned channel as they are added with
// AddReply. The returned channel is closed when RemoveRequest is
// invoked for the Request message. The returned boolean value
// indicates if the message was accepted.
func (rb *T) AddRequest(msg *messages.Request) (<-chan *messages.Reply, bool) {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	for rb.request != nil {
		req := rb.request
		rb.lock.Unlock()
		<-req.removed
		rb.lock.Lock()
	}

	if msg.Msg.Seq <= rb.lastSeq {
		return nil, false
	}

	rb.lastSeq = msg.Msg.Seq
	rb.request = newRequest(msg)

	for _, ch := range rb.newAdded {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	replyChannel := make(chan *messages.Reply)
	go rb.request.supplyReplies(replyChannel)

	return replyChannel, true
}

// AddReply adds a new Reply message to the buffer. Messages with no
// corresponding Request message in the buffer will be dropped. Any
// subsequent Reply message with the same replica ID corresponding to
// the same Request will be dropped. The return value indicates if the
// message was accepted.
func (rb *T) AddReply(msg *messages.Reply) bool {
	rb.lock.RLock()
	request := rb.request
	rb.lock.RUnlock()

	if request == nil {
		return false
	} else if request.Msg.Seq != msg.Msg.Seq {
		return false
	}

	return request.addReply(msg)
}

// RemoveRequest removes a Request message and all corresponding Reply
// messages given a sequence ID of the Request message.
func (rb *T) RemoveRequest(seq uint64) {
	rb.lock.Lock()
	defer rb.lock.Unlock()

	if rb.request == nil {
		return
	} else if rb.request.Msg.Seq != seq {
		return
	}

	close(rb.request.removed)
	rb.request = nil
}

// RequestStream returns a channel to receive all Request messages as
// they are added with AddRequest. Closing the channel passed to this
// function indicates the returned channel should be closed. Nil
// channel may be passed if there's no need to close the returned
// channel.
func (rb *T) RequestStream(cancel <-chan struct{}) <-chan *messages.Request {
	ch := make(chan *messages.Request)
	go rb.supplyRequests(ch, cancel)

	return ch
}

func (rb *T) supplyRequests(ch chan<- *messages.Request, cancel <-chan struct{}) {
	defer close(ch)

	newAdded := make(chan struct{}, 1)
	rb.lock.Lock()
	rb.newAdded = append(rb.newAdded, newAdded)
	rb.lock.Unlock()

	lastSeq := uint64(0)
	for {
		rb.lock.RLock()
		request := rb.request
		rb.lock.RUnlock()

		if request != nil && request.Msg.Seq > lastSeq {
			select {
			case ch <- request.Request:
				lastSeq = request.Msg.Seq
			case <-request.removed:
			case <-cancel:
				return
			}
		}

		select {
		case <-newAdded:
		case <-cancel:
			return
		}
	}
}

// request encapsulates a single Request message in the buffer
// together with corresponding Reply messages.
type request struct {
	// The Request message
	*messages.Request

	// This channel is closed after the request is removed from
	// the buffer
	removed chan struct{}

	// Collection of corresponding Reply messages
	*replySet
}

func newRequest(msg *messages.Request) *request {
	return &request{
		Request:  msg,
		removed:  make(chan struct{}),
		replySet: newReplySet(),
	}
}

func (r *request) addReply(msg *messages.Reply) bool {
	return r.replySet.addReply(msg)
}

func (r *request) supplyReplies(ch chan<- *messages.Reply) {
	r.replySet.supplyReplies(ch, r.removed)
}

type replySet struct {
	sync.RWMutex

	// Set of replica IDs of added Reply messages
	replicas map[uint32]bool

	// Reply messages in order added
	msgs []*messages.Reply

	// Buffered channel to notify about new messages
	newAdded chan struct{}
}

func newReplySet() *replySet {
	return &replySet{
		replicas: make(map[uint32]bool),
		newAdded: make(chan struct{}, 1),
	}
}

func (rs *replySet) addReply(msg *messages.Reply) bool {
	rs.Lock()
	defer rs.Unlock()

	if rs.replicas[msg.Msg.ReplicaId] {
		return false
	}

	rs.replicas[msg.Msg.ReplicaId] = true
	rs.msgs = append(rs.msgs, msg)

	select {
	case rs.newAdded <- struct{}{}:
	default:
	}

	return true
}

func (rs *replySet) supplyReplies(ch chan<- *messages.Reply, cancel <-chan struct{}) {
	defer close(ch)

	next := 0
	for {
		rs.RLock()
		msgs := rs.msgs[next:]
		next = len(rs.msgs)
		rs.RUnlock()

		for _, msg := range msgs {
			select {
			case ch <- msg:
			case <-cancel:
				return
			}
		}

		select {
		case <-rs.newAdded:
		case <-cancel:
			return
		}
	}
}
