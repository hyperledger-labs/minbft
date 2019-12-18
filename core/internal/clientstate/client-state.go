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
	"time"

	"github.com/hyperledger-labs/minbft/core/internal/timer"
	"github.com/hyperledger-labs/minbft/messages"
)

// Provider returns an instance of state representation associated
// with a client given its ID. It is safe to invoke concurrently.
type Provider func(clientID uint32) State

// NewProvider creates an instance of Provider. Optional parameters
// can be specified as opts.
func NewProvider(requestTimeout func() time.Duration, prepareTimeout func() time.Duration, opts ...Option) Provider {
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
			state = New(requestTimeout, prepareTimeout, opts...)
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
// indicates if the supplied identifier was new. In that case, the
// newly captured request identifier has to be released by invoking
// the returned release function.
//
// PrepareRequestSeq records the request identifier seq as prepared.
// An identifier can only be prepared if it is greater than the last
// prepared and has been captured before.
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
//
// StartRequestTimer starts a timer for the supplied request
// identifier to expire after the duration of request timeout. The
// supplied callback function handleTimeout is invoked asynchronously
// upon timer expiration. If the previous timer started for any
// request identifier has not yet expired, it will be canceled and a
// new timer started.
//
// StopRequestTimer stops timer started for the same request
// identifier by StartRequestTimer, if any.
//
// StartPrepareTimer starts a timer for the supplied request
// identifier to expire after the duration of prepare timeout. The
// supplied callback function handleTimeout is invoked asynchronously
// upon timer expiration. If the previous timer started for any
// request identifier has not yet expired, it will be canceled and a
// new timer started.
//
// StopPrepareTimer stops timer started for the same request
// identifier by StartPrepareTimer, if any.
type State interface {
	CaptureRequestSeq(seq uint64) (new bool, release func())
	PrepareRequestSeq(seq uint64) (new bool, err error)
	RetireRequestSeq(seq uint64) (new bool, err error)

	AddReply(reply messages.Reply) error
	ReplyChannel(seq uint64) <-chan messages.Reply

	StartRequestTimer(seq uint64, handleTimeout func())
	StopRequestTimer(seq uint64)

	StartPrepareTimer(seq uint64, handleTimeout func())
	StopPrepareTimer(seq uint64)
}

// New creates a new instance of client state representation. Optional
// arguments opts specify initialization parameters.
func New(requestTimeout func() time.Duration, prepareTimeout func() time.Duration, opts ...Option) State {
	s := &clientState{opts: defaultOptions}

	for _, opt := range opts {
		opt(&s.opts)
	}

	s.seqState = newSeqState()
	s.replyState = newReplyState()
	s.requestTimer = newTimerState(s.opts.timerProvider, requestTimeout)
	s.prepareTimer = newTimerState(s.opts.timerProvider, prepareTimeout)

	return s
}

// Option represents a parameter to initialize State with.
type Option func(*options)

type options struct {
	timerProvider timer.Provider
}

var defaultOptions = options{
	timerProvider: timer.Standard(),
}

// WithTimerProvider specifies the abstract timer implementation to
// use. Standard timer implementation is used by default.
func WithTimerProvider(timerProvider timer.Provider) Option {
	return func(opts *options) {
		opts.timerProvider = timerProvider
	}
}

type clientState struct {
	*seqState
	*replyState

	requestTimer *timerState
	prepareTimer *timerState

	opts options
}

func (s *clientState) StartRequestTimer(seq uint64, handleTimeout func()) {
	s.requestTimer.StartTimer(seq, handleTimeout)
}

func (s *clientState) StopRequestTimer(seq uint64) {
	s.requestTimer.StopTimer(seq)
}

func (s *clientState) StartPrepareTimer(seq uint64, handleTimeout func()) {
	s.prepareTimer.StartTimer(seq, handleTimeout)
}

func (s *clientState) StopPrepareTimer(seq uint64) {
	s.prepareTimer.StopTimer(seq)
}
