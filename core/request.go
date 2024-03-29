// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li <wenting.li@neclab.eu>
//          Sergey Fedorov <sergey.fedorov@neclab.eu>
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

package minbft

import (
	"fmt"
	"time"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/requestlist"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
	"github.com/hyperledger-labs/minbft/messages"
)

// requestReplier provides Reply message given Request message.
//
// It returns a channel that can be used to receive a Reply message
// corresponding to the supplied Request message. It is safe to invoke
// concurrently.
type requestReplier func(request messages.Request) <-chan messages.Reply

// requestValidator validates a Request message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type requestValidator func(request messages.Request) error

// requestProcessor processes a valid Request message.
//
// It fully processes the supplied message. The supplied message is
// assumed to be authentic and internally consistent. The return value
// new indicates if the message has not been processed by this replica
// before. It is safe to invoke concurrently.
type requestProcessor func(request messages.Request) (new bool, err error)

// requestApplier applies Request message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required messages
// or side effects. The supplied message is assumed to be authentic
// and internally consistent. The supplied view number should denote
// the current active view. It is safe to invoke concurrently.
type requestApplier func(request messages.Request, view uint64) error

// pendingRequestApplier applies all pending requests.
//
// It applies all the pending requests to the current replica state in
// the supplied active view number.
type pendingRequestApplier func(view uint64)

// requestExecutor given a Request message executes the requested
// operation, produces the corresponding Reply message ready for
// delivery to the client, and hands it over for further processing.
type requestExecutor func(request messages.Request)

// requestCapturer synchronizes beginning of request processing.
//
// Processing of Request messages generated by the same client is
// synchronized by waiting to ensure each request identifier is
// processed at most once and in the order of increasing identifier
// value. The return value new indicates if the message needs to be
// processed. In that case, the processing has to be completed by
// invoking the returned release function. It is safe to invoke
// concurrently.
type requestCapturer func(request messages.Request) (new bool, release func())

// requestPreparer records the request as prepared.
//
// It records the supplied request as prepared. The request must be
// previously captured. It returns true if the request could not have
// been prepared before. It is safe to invoke concurrently.
type requestPreparer func(request messages.Request) (new bool)

// requestRetirer records request as retired.
//
// It records the supplied request as retired. It returns true if the
// request could not have been retired before. The identifier must be
// previously prepared. It is safe to invoke concurrently.
type requestRetirer func(request messages.Request) (new bool)

// requestUnpreparer un-prepares prepared but not retired requests.
type requestUnpreparer func()

// requestTimerStarter starts request timer.
//
// A request timeout event is triggered if the request timeout elapses
// before corresponding requestTimerStopper is called with the same
// request message passed. The argument view specifies the current
// view number. Only single timer per client is maintained. The timer
// is restarted if the previous timer of the client has not yet
// stopped or expired. It is safe to invoke concurrently.
type requestTimerStarter func(request messages.Request, view uint64)

// requestTimerStopper stops request timer.
//
// Given a Request message, any request timer started for the same
// request is stopped, if it has not already been stopped or expired.
// It is safe to invoke concurrently.
type requestTimerStopper func(request messages.Request)

// requestTimeoutHandler handles request timeout expiration.
//
// The argument view is the view number in which the request timer was
// started. It is safe to invoke concurrently.
type requestTimeoutHandler func(view uint64)

// requestTimeoutProvider returns current request timeout duration.
type requestTimeoutProvider func() time.Duration

// prepareTimerStarter starts prepare timer.
//
// A prepare timeout event is triggered if the prepare timeout elapses
// before corresponding prepareTimerStopper is called with the same
// request message passed. The argument view specifies the current
// view number. Only single timer per client is maintained. The timer
// is restarted if the previous timer of the client has not yet
// stopped or expired. It is safe to invoke concurrently.
type prepareTimerStarter func(request messages.Request, view uint64)

// prepareTimerStopper stops prepare timer.
//
// Given a Request message, any prepare timer started for the same
// request is stopped, if it has not already been stopped or expired.
// It is safe to invoke concurrently.
type prepareTimerStopper func(request messages.Request)

// prepareTimeoutProvider returns current prepare timeout duration.
type prepareTimeoutProvider func() time.Duration

// makeRequestValidator constructs an instance of requestValidator
// using the supplied abstractions.
func makeRequestValidator(verify messageSignatureVerifier) requestValidator {
	return func(request messages.Request) error {
		return verify(request)
	}
}

// makeRequestProcessor constructs an instance of requestProcessor
// using id as the current replica ID, n as the total number of nodes,
// and the supplied abstractions.
func makeRequestProcessor(captureReq requestCapturer, viewState viewstate.State, applyRequest requestApplier) requestProcessor {
	return func(request messages.Request) (new bool, err error) {
		currentView, expectedView, releaseView := viewState.HoldView()
		defer releaseView()

		new, releaseReq := captureReq(request)
		if !new {
			return false, nil
		}
		defer releaseReq()

		if currentView != expectedView {
			return true, nil
		}

		if err := applyRequest(request, currentView); err != nil {
			return false, fmt.Errorf("failed to apply Request: %s", err)
		}

		return true, nil
	}
}

func makeRequestApplier(id, n uint32, startReqTimer requestTimerStarter, startPrepTimer prepareTimerStarter, handleGeneratedMessage generatedMessageHandler) requestApplier {
	return func(request messages.Request, view uint64) error {
		// The primary has to start the request timer, too.
		// Suppose, the primary is correct, but its messages
		// are delayed, and other replicas switch to a new
		// view. In that case, other replicas might rely on
		// this correct replica to trigger another view
		// change, should the new primary be faulty.
		startReqTimer(request, view)

		if isPrimary(view, id, n) {
			handleGeneratedMessage(messageImpl.NewPrepare(id, view, request))
		} else {
			startPrepTimer(request, view)
		}

		return nil
	}
}

func makePendingRequestApplier(pendingReqs requestlist.List, applyRequest requestApplier) pendingRequestApplier {
	return func(view uint64) {
		for _, req := range pendingReqs.All() {
			_ = applyRequest(req, view)
		}
	}
}

// makeRequestReplier constructs an instance of requestReplier using
// the supplied client state provider.
func makeRequestReplier(provider clientstate.Provider, stop <-chan struct{}) requestReplier {
	return func(request messages.Request) <-chan messages.Reply {
		state := provider.ClientState(request.ClientID())
		return state.ReplyChannel(request.Sequence(), stop)
	}
}

// makeRequestExecutor constructs an instance of requestExecutor using
// the supplied replica ID and abstractions.
func makeRequestExecutor(id uint32, retireReq requestRetirer, stopReqTimer requestTimerStopper, consumer api.RequestConsumer, handleGeneratedMessage generatedMessageHandler) requestExecutor {
	return func(request messages.Request) {
		clientID := request.ClientID()
		seq := request.Sequence()

		if new := retireReq(request); !new {
			return // request already accepted for execution
		}

		stopReqTimer(request)

		resultChan := consumer.Deliver(request.Operation())

		go func() {
			result := <-resultChan
			handleGeneratedMessage(messageImpl.NewReply(id, clientID, seq, result))
		}()
	}
}

// makeRequestCapturer constructs an instance of requestSeqCapturer
// using the supplied client state provider.
func makeRequestCapturer(clientStates clientstate.Provider, pendingReqs requestlist.List) requestCapturer {
	return func(request messages.Request) (new bool, release func()) {
		clientID := request.ClientID()
		seq := request.Sequence()

		new, release = clientStates.ClientState(clientID).CaptureRequestSeq(seq)
		if !new {
			return false, nil
		}

		pendingReqs.Add(request)

		return new, release
	}
}

// makeRequestPreparer constructs an instance of requestSeqPreparer
// using the supplied interface.
func makeRequestPreparer(clientStates clientstate.Provider, pendingReqs, preparedReqs requestlist.List) requestPreparer {
	return func(request messages.Request) (new bool) {
		clientID := request.ClientID()
		seq := request.Sequence()

		if new, err := clientStates.ClientState(clientID).PrepareRequestSeq(seq); err != nil {
			panic(err)
		} else if !new {
			return false
		}

		preparedReqs.Add(request)
		pendingReqs.Remove(request)

		return true
	}
}

// makeRequestRetirer constructs an instance of requestSeqRetirer
// using the supplied interface.
func makeRequestRetirer(clientStates clientstate.Provider, preparedReqs requestlist.List) requestRetirer {
	return func(request messages.Request) (new bool) {
		clientID := request.ClientID()
		seq := request.Sequence()

		if new, err := clientStates.ClientState(clientID).RetireRequestSeq(seq); err != nil {
			panic(err)
		} else if !new {
			return false
		}

		preparedReqs.Remove(request)

		return true
	}
}

func makeRequestUnpreparer(clientStates clientstate.Provider, pendingReqs, preparedReqs requestlist.List) requestUnpreparer {
	return func() {
		for _, req := range preparedReqs.All() {
			clientStates.ClientState(req.ClientID()).UnprepareRequestSeq()
			pendingReqs.Add(req)
			preparedReqs.Remove(req)
		}
	}
}

// makeRequestTimerStarter constructs an instance of
// requestTimerStarter.
func makeRequestTimerStarter(clientStates clientstate.Provider, handleTimeout requestTimeoutHandler, logger logger.Logger) requestTimerStarter {
	return func(request messages.Request, view uint64) {
		clientID := request.ClientID()
		seq := request.Sequence()
		clientStates.ClientState(clientID).StartRequestTimer(seq, func() {
			logger.Warningf("Request timer expired: client=%d seq=%d view=%d", clientID, seq, view)
			handleTimeout(view)
		})
	}
}

// makeRequestTimerStopper constructs an instance of
// requestTimerStopper.
func makeRequestTimerStopper(clientStates clientstate.Provider) requestTimerStopper {
	return func(request messages.Request) {
		clientStates.ClientState(request.ClientID()).StopRequestTimer(request.Sequence())
	}
}

// makeRequestTimeoutProvider constructs an instance of
// requestTimeoutProvider.
func makeRequestTimeoutProvider(config api.Configer) requestTimeoutProvider {
	// The View Change operation is not yet implemented, thus it
	// simply returns the initial request timeout duration. When
	// the View Change is implemented, this duration might be
	// required to increase dynamically when the View Change is
	// triggered, to guarantee liveness in case of increased
	// network delay.
	return func() time.Duration {
		return config.TimeoutRequest()
	}
}

// makePrepareTimerStarter constructs an instance of
// prepareTimerStarter.
func makePrepareTimerStarter(n uint32, clientStates clientstate.Provider, unicastLogs map[uint32]messagelog.MessageLog, logger logger.Logger) prepareTimerStarter {
	return func(request messages.Request, view uint64) {
		clientID := request.ClientID()
		seq := request.Sequence()
		clientStates.ClientState(clientID).StartPrepareTimer(seq, func() {
			logger.Infof("Prepare timer expired: client=%d seq=%d view=%d", clientID, seq, view)
			unicastLogs[uint32(view%uint64(n))].Append(request)
		})
	}
}

// makePrepareTimerStopper constructs an instance of
// prepareTimerStopper.
func makePrepareTimerStopper(clientStates clientstate.Provider) prepareTimerStopper {
	return func(request messages.Request) {
		clientStates.ClientState(request.ClientID()).StopPrepareTimer(request.Sequence())
	}
}

// makePrepareTimeoutProvider constructs an instance of
// prepareTimeoutProvider.
func makePrepareTimeoutProvider(config api.Configer) prepareTimeoutProvider {
	return func() time.Duration {
		return config.TimeoutPrepare()
	}
}
