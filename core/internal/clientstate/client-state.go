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

// Package clientstate provides means to interact with a
// representation of the state maintained by the replica for each
// client instance.
package clientstate

import (
	"fmt"
	"sync"

	"github.com/hyperledger-labs/minbft/messages"
)

// Provider returns an instance of state representation associated
// with a client given its ID. It is safe to invoke concurrently.
type Provider func(clientID uint32) State

// NewProvider creates an instance of Provider
func NewProvider() Provider {
	var (
		lock sync.Mutex
		// Client ID -> client state
		clientStates = make(map[uint32]State)
	)

	return func(clientID uint32) State {
		lock.Lock()
		defer lock.Unlock()

		state := clientStates[clientID]
		if state == nil {
			state = New()
			clientStates[clientID] = state
		}

		return state
	}
}

// State represents the state maintained by the replica for each
// client instance. All methods are safe to invoke concurrently.
//
// AcceptRequestSeq accepts a request identifier seq. If the request
// with the last accepted identifier is still in progress, it will
// block until it is finished. A request is said to be in progress
// (not finished) if its identifier has been accepted, but the
// corresponding Reply message has not yet been supplied. A request
// identifier is new if it is greater than the greatest accepted. The
// return value new indicates if the supplied request identifier was
// new.
//
// AddReply accepts a Reply message. Reply messages should be added in
// sequence of corresponding request identifiers. Only a single Reply
// message should be added for each request identifier. It will never
// be blocked by any of the channels returned from ReplyChannel.
//
// ReplyChannel returns a channel to receive the Reply message
// corresponding to the supplied request identifier. The returned
// channel will be closed after the Reply message is sent to it or
// there will be no Reply message to be added for the supplied request
// identifier.
type State interface {
	AcceptRequestSeq(seq uint64) (new bool)
	AddReply(reply *messages.Reply) error
	ReplyChannel(seq uint64) <-chan *messages.Reply
}

// New creates a new instance of client state representation.
func New() State {
	return &clientState{}
}

type clientState struct {
	sync.Mutex

	// Last accepted request ID
	lastAcceptedSeq uint64

	// Last replied request ID
	lastExecutedSeq uint64

	// Last Reply message
	reply *messages.Reply

	// Channels to close when new Reply added
	replyAdded []chan<- struct{}
}

func (c *clientState) AcceptRequestSeq(seq uint64) (new bool) {
	c.Lock()
	defer c.Unlock()

	for {
		if seq <= c.lastAcceptedSeq {
			return false
		}

		if c.lastAcceptedSeq != c.lastExecutedSeq {
			c.waitForReplyLocked()

			continue // recheck
		}

		c.lastAcceptedSeq = seq

		return true
	}
}

func (c *clientState) AddReply(reply *messages.Reply) error {
	seq := reply.Msg.Seq

	c.Lock()
	defer c.Unlock()

	if seq <= c.lastExecutedSeq {
		return fmt.Errorf("old request ID")
	}

	c.reply = reply
	c.lastExecutedSeq = seq

	for _, ch := range c.replyAdded {
		close(ch)
	}
	c.replyAdded = nil

	return nil
}

func (c *clientState) ReplyChannel(seq uint64) <-chan *messages.Reply {
	out := make(chan *messages.Reply, 1)

	go func() {
		defer close(out)

		c.Lock()
		for c.lastExecutedSeq < seq {
			c.waitForReplyLocked()
		}
		reply := c.reply
		c.Unlock()

		if reply.Msg.Seq == seq {
			out <- reply
		}
	}()

	return out
}

func (c *clientState) waitForReplyLocked() {
	replyAdded := make(chan struct{})
	c.replyAdded = append(c.replyAdded, replyAdded)

	c.Unlock()
	<-replyAdded
	c.Lock()
}
