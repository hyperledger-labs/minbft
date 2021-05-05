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

// Provider represents the replica state maintained for the clients.
// It is safe to use concurrently.
//
// ClientState method returns the piece of replica state associated
// with a client, given the client ID.
//
// Clients method returns a slice of maintained client IDs.
type Provider interface {
	ClientState(clientID uint32) State
	Clients() (clientIDs []uint32)
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

type provider struct {
	sync.Mutex
	options

	requestTimeout func() time.Duration
	prepareTimeout func() time.Duration

	// Client ID -> client state
	clientStates map[uint32]*clientState
}

// NewProvider creates an instance of Provider. Optional parameters
// can be specified as args.
func NewProvider(requestTimeout func() time.Duration, prepareTimeout func() time.Duration, args ...Option) Provider {
	opts := defaultOptions
	for _, opt := range args {
		opt(&opts)
	}

	return &provider{
		options:        opts,
		requestTimeout: requestTimeout,
		prepareTimeout: prepareTimeout,
		clientStates:   make(map[uint32]*clientState),
	}
}

func (p *provider) ClientState(clientID uint32) State {
	p.Lock()
	defer p.Unlock()

	state := p.clientStates[clientID]
	if state == nil {
		state = newClientState(p.timerProvider, p.requestTimeout, p.prepareTimeout)
		p.clientStates[clientID] = state
	}

	return state
}

func (p *provider) Clients() (clientIDs []uint32) {
	p.Lock()
	defer p.Unlock()

	clientIDs = make([]uint32, 0, len(p.clientStates))
	for c := range p.clientStates {
		clientIDs = append(clientIDs, c)
	}

	return clientIDs
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

type clientState struct {
	*seqState
	*replyState

	requestTimer *timerState
	prepareTimer *timerState
}

func newClientState(timerProvider timer.Provider, requestTimeout, prepareTimeout func() time.Duration) *clientState {
	return &clientState{
		seqState:     newSeqState(),
		replyState:   newReplyState(),
		requestTimer: newTimerState(timerProvider, requestTimeout),
		prepareTimer: newTimerState(timerProvider, prepareTimeout),
	}
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
