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
// CaptureRequestSeq captures a request identifier seq. The greatest
// captured identifier has to be released before a new identifier can
// be captured. A new captured identifier has to be released before
// any identifier that is greater than the last released can be
// captured. An identifier is new if it is greater than the greatest
// captured. If an identifier cannot be captured immediately, it will
// block until the identifier can be captured. The return value new
// indicates if the supplied identifier was new.
//
// ReleaseRequestSeq releases the last captured new request identifier
// seq so that another new identifier of the same identifier can be
// captured. It is an error attempting to release an identifier that
// has not been captured before or to release the same identifier more
// than once.
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
	CaptureRequestSeq(seq uint64) (new bool)
	ReleaseRequestSeq(seq uint64) error
	AddReply(reply *messages.Reply) error
	ReplyChannel(seq uint64) <-chan *messages.Reply
}

// New creates a new instance of client state representation.
func New() State {
	c := &clientState{}
	c.seqReleased = sync.NewCond(c)
	return c
}

type clientState struct {
	sync.Mutex

	// Last captured request ID
	lastCapturedSeq uint64

	// Last released request ID
	lastReleasedSeq uint64

	// Cond to signal on when ID is released
	seqReleased *sync.Cond

	// Last replied request ID
	lastRepliedSeq uint64

	// Last Reply message
	reply *messages.Reply

	// Channels to close when new Reply added
	replyAdded []chan<- struct{}
}

func (c *clientState) CaptureRequestSeq(seq uint64) (new bool) {
	c.Lock()
	defer c.Unlock()

	for {
		if seq <= c.lastCapturedSeq {
			// The request ID is not new.
			// Wait until it gets released.
			for seq > c.lastReleasedSeq {
				c.seqReleased.Wait()
			}

			return false
		}

		// The request ID might be new. Check if the greatest
		// captured ID got released.
		if c.lastCapturedSeq > c.lastReleasedSeq {
			// The greatest captured ID is not released.
			// Wait for it to get released and try again.
			c.seqReleased.Wait()

			continue
		}

		c.lastCapturedSeq = seq

		return true
	}
}

func (c *clientState) ReleaseRequestSeq(seq uint64) error {
	c.Lock()
	defer c.Unlock()

	if seq != c.lastCapturedSeq {
		return fmt.Errorf("request ID is not the last captured")
	} else if seq <= c.lastReleasedSeq {
		return fmt.Errorf("request ID already released")
	}

	c.lastReleasedSeq = seq
	c.seqReleased.Broadcast()

	return nil
}

func (c *clientState) AddReply(reply *messages.Reply) error {
	seq := reply.Msg.Seq

	c.Lock()
	defer c.Unlock()

	if seq <= c.lastRepliedSeq {
		return fmt.Errorf("old request ID")
	}

	c.reply = reply
	c.lastRepliedSeq = seq

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
		for c.lastRepliedSeq < seq {
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
