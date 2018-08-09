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
// PrepareRequestSeq records the request identifier seq as prepared.
// An identifier can only be prepared if it is greater than the last
// prepared and has been captured and released before.
//
// RetireRequestSeq records the request identifier seq as retired. An
// identifier can only be retired if it is greater than the last
// retired and has been prepared before.
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
	PrepareRequestSeq(seq uint64) error
	RetireRequestSeq(seq uint64) error

	AddReply(reply *messages.Reply) error
	ReplyChannel(seq uint64) <-chan *messages.Reply
}

// New creates a new instance of client state representation.
func New() State {
	s := &clientState{}
	s.seqState = newSeqState()
	s.replyState = newReplyState()

	return s
}

type clientState struct {
	*seqState
	*replyState
}
