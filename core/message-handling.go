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

package minbft

import (
	"fmt"
	"sync"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/backofftimer"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/requestlist"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
	"github.com/hyperledger-labs/minbft/messages"
)

// messageStreamHandler fetches serialized messages from in channel,
// handles the received messages, and sends a serialized reply
// message, if any, to reply channel.
type messageStreamHandler func(in <-chan []byte, reply chan<- []byte)

// messageHandler fully handles message.
//
// If there are any messages produced in reply, they will be sent to
// reply channel, otherwise nil channel is returned. The return value
// new indicates that the message has not been processed before.
type messageHandler func(msg messages.Message) (reply <-chan messages.Message, new bool, err error)

// embeddedMessageHandler fully handles embedded messages.
//
// It recursively handles messages embedded into the supplied message.
// It is safe to invoke concurrently.
type embeddedMessageHandler func(msg messages.Message) error

// peerConnector initiates message exchange with a peer replica.
//
// Given a channel of outgoing messages to supply to the replica, it
// returns a channel of messages produced by the replica in reply.
type peerConnector func(out <-chan []byte) (in <-chan []byte, err error)

// messageValidator validates a message.
//
// It fully checks the supplied message for internal consistency and
// authenticity. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type messageValidator func(msg messages.Message) error

// peerMessageValidator validates a peer message.
//
// It continues validation of the supplied peer message.
// It is safe to invoke concurrently.
type peerMessageValidator func(msg messages.PeerMessage) error

// certifiedMessageValidator validates a message certified by USIG.
//
// It continues validation of the supplied message certified by USIG.
// It is safe to invoke concurrently.
type certifiedMessageValidator func(msg messages.CertifiedMessage) error

// messageProcessor processes a valid message.
//
// It fully processes the supplied message in the context of the
// current replica's state. The supplied message is assumed to be
// authentic and internally consistent. The return value new indicates
// if the message had any effect. It is safe to invoke concurrently.
type messageProcessor func(msg messages.Message) (new bool, err error)

// peerMessageProcessor processes a valid peer message.
//
// It continues processing of the supplied peer message. The return
// value new indicates if the message had any effect. It is safe to
// invoke concurrently.
type peerMessageProcessor func(msg messages.PeerMessage) (new bool, err error)

// certifiedMessageProcessor processes a valid message with UI.
//
// It continues processing of the supplied message certified by USIG.
// Messages originated from the same replica are guaranteed to be
// processed only once and in sequence according to the assigned UI.
// The return value new indicates if the message had any effect.
// It is safe to invoke concurrently for messages from different replicas.
type certifiedMessageProcessor func(msg messages.CertifiedMessage) (new bool, err error)

// viewMessageProcessor processes a valid message in current view.
//
// It continues processing of the supplied message, according to the
// current view number. The message is guaranteed to be processed in a
// corresponding view, or not processed at all. The return value new
// indicates if the message had any effect. It is safe to invoke
// concurrently.
type viewMessageProcessor func(msg messages.PeerMessage) (new bool, err error)

// peerMessageApplier applies a peer message to current replica state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required messages
// or side effects. The supplied message is assumed to be authentic
// and internally consistent. It is safe to invoke concurrently.
type peerMessageApplier func(msg messages.PeerMessage) error

// generatedMessageHandler finalizes and handles generated message.
//
// It finalizes the supplied message by attaching an authentication
// tag to the message, then takes further steps to handle the message.
// It is safe to invoke concurrently.
type generatedMessageHandler func(msg messages.ReplicaMessage)

// generatedMessageConsumer receives generated message.
//
// It arranges the supplied message to be delivered to peer replicas
// or the corresponding client, as well as to be handled locally,
// depending on the message type. The message should be ready to
// serialize and deliver to the recipients. It is safe to invoke
// concurrently.
type generatedMessageConsumer func(msg messages.ReplicaMessage)

// defaultMessageHandlers constructs standard message handlers using
// id as the current replica ID and the supplied interfaces.
func defaultMessageHandlers(id uint32, log messagelog.MessageLog, unicastLogs map[uint32]messagelog.MessageLog, stop <-chan struct{}, config api.Configer, stack Stack, logger logger.Logger) (handleOwnMessage, handlePeerMessage, handleClientMessage messageHandler) {
	n := config.N()
	f := config.F()

	// Commit certificate size is the number of commitments from
	// different replicas, i.e. Prepare/NewView/Commit messages,
	// that triggers request execution. View-change certificate
	// size is the number of replicas required to proceed with
	// view change. Any commit and view-change certificates must
	// intersect in at least one replica.
	commitCertSize := n - f
	viewChangeCertSize := n - commitCertSize + 1

	reqTimeout := makeRequestTimeoutProvider(config)
	prepTimeout := makePrepareTimeoutProvider(config)

	verifyMessageSignature := makeMessageSignatureVerifier(stack, messages.AuthenBytes)
	signMessage := makeMessageSigner(stack, messages.AuthenBytes)
	verifyUI := makeUIVerifier(stack, messages.AuthenBytes)
	assignUI := makeUIAssigner(stack, messages.AuthenBytes)

	clientStates := clientstate.NewProvider(reqTimeout, prepTimeout)
	viewState := viewstate.New()
	viewChangeTimer := backofftimer.New(config.TimeoutViewChange())

	pendingReqs := requestlist.New()
	preparedReqs := requestlist.New()
	captureReq := makeRequestCapturer(clientStates, pendingReqs)
	prepareReq := makeRequestPreparer(clientStates, pendingReqs, preparedReqs)
	retireReq := makeRequestRetirer(clientStates, preparedReqs)
	unprepareReqs := makeRequestUnpreparer(clientStates, pendingReqs, preparedReqs)

	consumeGeneratedMessage := makeGeneratedMessageConsumer(log, clientStates, logger)
	handleGeneratedMessage := makeGeneratedMessageHandler(signMessage, assignUI, consumeGeneratedMessage)

	requestViewChange := makeViewChangeRequestor(id, viewState, handleGeneratedMessage)
	handleReqTimeout := makeRequestTimeoutHandler(requestViewChange, logger)
	handleViewChangeTimeout := makeViewChangeTimeoutHandler(requestViewChange, logger)
	startReqTimer := makeRequestTimerStarter(clientStates, handleReqTimeout, logger)
	stopReqTimer := makeRequestTimerStopper(clientStates)
	startPrepTimer := makePrepareTimerStarter(n, clientStates, unicastLogs, logger)
	stopPrepTimer := makePrepareTimerStopper(clientStates)
	startVCTimer := makeViewChangeTimerStarter(viewChangeTimer, handleViewChangeTimeout, logger)
	stopVCTimer := viewChangeTimer.Stop

	validateRequest := makeRequestValidator(verifyMessageSignature)
	validatePrepare := makePrepareValidator(n)
	validateCommit := makeCommitValidator()
	validateReqViewChange := makeReqViewChangeValidator(verifyMessageSignature)
	validateVCCert := makeViewChangeCertValidator(viewChangeCertSize)
	validateNVCert := makeNewViewCertValidator(viewChangeCertSize)
	validateViewChange := makeViewChangeValidator(validateMessageLog, validateVCCert)
	validateNewView := makeNewViewValidator(n, validateNVCert)
	validateCertifiedMessage := makeCertifiedMessageValidator(validatePrepare, validateCommit, validateViewChange, validateNewView, verifyUI)
	validatePeerMessage := makePeerMessageValidator(n, validateCertifiedMessage, validateReqViewChange)
	validateMessage := makeMessageValidator(validateRequest, validatePeerMessage)

	applyRequest := makeRequestApplier(id, n, startReqTimer, startPrepTimer, handleGeneratedMessage)
	applyPendingRequests := makePendingRequestApplier(pendingReqs, applyRequest)
	executeRequest := makeRequestExecutor(id, retireReq, stopReqTimer, stack, handleGeneratedMessage)
	acceptNewView := makeNewViewAcceptor(extractPreparedRequests, executeRequest)

	acceptCommitment := makeCommitmentAcceptor()
	countCommitment := makeCommitmentCounter(commitCertSize)
	collectCommitment := makeCommitmentCollector(acceptCommitment, countCommitment, executeRequest, acceptNewView)

	applyPrepare := makePrepareApplier(id, prepareReq, collectCommitment, handleGeneratedMessage, stopPrepTimer)
	applyCommit := makeCommitApplier(collectCommitment)
	applyNewView := makeNewViewApplier(id, extractPreparedRequests, prepareReq, collectCommitment, stopVCTimer, applyPendingRequests, handleGeneratedMessage)
	applyPeerMessage := makePeerMessageApplier(applyPrepare, applyCommit, applyNewView)

	processRequest := makeRequestProcessor(captureReq, viewState, applyRequest)

	collectViewChange := makeViewChangeCollector(id, n, viewChangeCertSize)
	processNewViewCert := makeNewViewCertProcessor(id, viewState, log, handleGeneratedMessage)
	processViewChange := makeViewChangeProcessor(collectViewChange, processNewViewCert)

	processViewMessage := makeViewMessageProcessor(viewState, applyPeerMessage)
	processCertifiedMessage := makeCertifiedMessageProcessor(n, processViewMessage, processViewChange)

	collectReqViewChange := makeReqViewChangeCollector(viewChangeCertSize)
	startViewChange := makeViewChangeStarter(id, viewState, log, startVCTimer, unprepareReqs, handleGeneratedMessage)
	processReqViewChange := makeReqViewChangeProcessor(collectReqViewChange, startViewChange)

	processPeerMessage := makePeerMessageProcessor(n, processCertifiedMessage, processReqViewChange)
	processMessage := makeMessageProcessor(processRequest, processPeerMessage)

	// This "thunk" delays evaluation of handleOwnMessage thus
	// resolving circular dependency due to recursive nature of
	// peer message handling.
	handleOwnMessageThunk := func(msg messages.Message) (reply <-chan messages.Message, new bool, err error) {
		return handleOwnMessage(msg)
	}
	handleOwnEmbedded := makeEmbeddedMessageHandler(handleOwnMessageThunk)
	handleOwnMessage = makeOwnMessageHandler(handleOwnEmbedded, processMessage)

	// This "thunk" delays evaluation of handlePeerMessage thus
	// resolving circular dependency due to recursive nature of
	// peer message handling.
	handlePeerMessageThunk := func(msg messages.Message) (reply <-chan messages.Message, new bool, err error) {
		return handlePeerMessage(msg)
	}
	handleEmbedded := makeEmbeddedMessageHandler(handlePeerMessageThunk)
	handlePeerMessage = makePeerMessageHandler(validateMessage, handleEmbedded, processMessage)

	replyRequest := makeRequestReplier(clientStates, stop)
	handleClientMessage = makeClientMessageHandler(validateRequest, processRequest, replyRequest)

	go func() {
		<-stop
		for _, c := range clientStates.Clients() {
			clientStates.ClientState(c).StopAllTimers()
		}
		viewChangeTimer.Stop()
	}()

	return
}

// makeMessageStreamHandler construct an instance of
// messageStreamHandler using the supplied abstract handler.
func makeMessageStreamHandler(handleMessage messageHandler, remote string, stop <-chan struct{}, logger logger.Logger) messageStreamHandler {
	handleReplyChan := func(replyChan <-chan messages.Message, outChan chan<- []byte, remote string) {
		for {
			var msgBytes []byte
			select {
			case msg, ok := <-replyChan:
				if !ok {
					return
				}
				msgStr := messages.Stringify(msg)
				logger.Debugf("Sending %s to %s", msgStr, remote)
				var err error
				msgBytes, err = msg.MarshalBinary()
				if err != nil {
					panic(err)
				}
			case <-stop:
				return
			}

			select {
			case outChan <- msgBytes:
			case <-stop:
				return
			}
		}
	}

	return func(in <-chan []byte, out chan<- []byte) {
		for {
			select {
			case msgBytes, ok := <-in:
				if !ok {
					logger.Infof("Lost connection from %s", remote)
					return
				}

				msg, err := messageImpl.NewFromBinary(msgBytes)
				if err != nil {
					logger.Warningf("Error unmarshaling message from %s: %s", remote, err)
					return
				}

				msgStr := messages.Stringify(msg)
				logger.Debugf("Received %s from %s", msgStr, remote)

				replyChan, new, err := handleMessage(msg)
				if err != nil {
					logger.Warningf("Error handling %s from %s: %s", msgStr, remote, err)
					return
				} else if !new {
					logger.Debugf("Dropped %s from %s", msgStr, remote)
				} else {
					logger.Debugf("Handled %s from %s", msgStr, remote)
				}

				if replyChan != nil {
					var remote string // avoid data race with logger
					switch m := msg.(type) {
					case messages.Hello:
						remote = fmt.Sprintf("replica %d", m.ReplicaID())
					case messages.ClientMessage:
						remote = fmt.Sprintf("client %d", m.ClientID())
					}
					handleReplyChan(replyChan, out, remote)
				}
			case <-stop:
				return
			}
		}
	}
}

// startPeerConnections initiates asynchronous message exchange with
// peer replicas.
func startPeerConnections(ownID, n uint32, connector api.ReplicaConnector, handleMessage messageHandler, stop <-chan struct{}, wg *sync.WaitGroup, logger logger.Logger) error {
	for peerID := uint32(0); peerID < n; peerID++ {
		if peerID == ownID {
			continue
		}

		remote := fmt.Sprintf("replica %d", peerID)
		connect := makePeerConnector(peerID, connector)
		handleReplyStream := makeMessageStreamHandler(handleMessage, remote, stop, logger)
		if err := startPeerConnection(ownID, connect, handleReplyStream, stop, wg); err != nil {
			return fmt.Errorf("cannot connect to replica %d: %s", peerID, err)
		}
	}

	return nil
}

// startPeerConnection initiates asynchronous message exchange with a
// peer replica.
func startPeerConnection(ownID uint32, connect peerConnector, handleReplyStream messageStreamHandler, stop <-chan struct{}, wg *sync.WaitGroup) error {
	out := make(chan []byte)
	in, err := connect(out)
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer close(out)
		defer wg.Done()

		h := messageImpl.NewHello(ownID)
		msgBytes, err := h.MarshalBinary()
		if err != nil {
			panic(err)
		}
		select {
		case <-stop:
			return
		case out <- msgBytes:
		}

		handleReplyStream(in, nil)
	}()

	return nil
}

// handleOwnPeerMessages handles messages generated by the local
// replica for the peer replicas.
func handleOwnPeerMessages(log messagelog.MessageLog, handleOwnMessage messageHandler, stop <-chan struct{}, logger logger.Logger) {
	for msg := range log.Stream(stop) {
		if _, new, err := handleOwnMessage(msg); err != nil {
			panic(fmt.Errorf("error handling own message: %s", err))
		} else if new {
			logger.Debugf("Handled own %s", messages.Stringify(msg))
		}
	}
}

// makePeerConnector constructs a peerConnector using the supplied
// peer replica ID and a general replica connector.
func makePeerConnector(peerID uint32, connector api.ReplicaConnector) peerConnector {
	return func(out <-chan []byte) (in <-chan []byte, err error) {
		sh := connector.ReplicaMessageStreamHandler(peerID)
		if sh == nil {
			return nil, fmt.Errorf("connection not possible")
		}
		return sh.HandleMessageStream(out), nil
	}
}

func makeHelloHandler(ownID, n uint32, messageLog messagelog.MessageLog, unicastLogs map[uint32]messagelog.MessageLog, stop <-chan struct{}) messageHandler {
	return func(msg messages.Message) (<-chan messages.Message, bool, error) {
		h, ok := msg.(messages.Hello)
		if !ok {
			return nil, false, fmt.Errorf("unexpected message type")
		}
		peerID := h.ReplicaID()
		if peerID >= n || peerID == ownID {
			return nil, false, fmt.Errorf("unexpected peer ID")
		}

		var replyChan = make(chan messages.Message)
		go func() {
			defer close(replyChan)

			var done = make(chan struct{})
			defer close(done)

			var bcChan, ucChan <-chan messages.Message
			bcChan = messageLog.Stream(done)
			if ucLog := unicastLogs[peerID]; ucLog != nil {
				ucChan = ucLog.Stream(done)
			}

			for {
				var msg messages.Message

				select {
				case msg = <-bcChan:
				case msg = <-ucChan:
				case <-stop:
					return
				}

				select {
				case replyChan <- msg:
				case <-stop:
					return
				}
			}
		}()

		return replyChan, true, nil
	}
}

func makeOwnMessageHandler(handleEmbedded embeddedMessageHandler, process messageProcessor) messageHandler {
	return func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		err = handleEmbedded(msg)
		if err != nil {
			return nil, false, fmt.Errorf("error handling embedded messages: %s", err)
		}

		new, err = process(msg)
		if err != nil {
			return nil, false, fmt.Errorf("error processing message: %s", err)
		}

		return nil, new, nil
	}
}

func makePeerMessageHandler(validate messageValidator, handleEmbedded embeddedMessageHandler, process messageProcessor) messageHandler {
	return func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		err = validate(msg)
		if err != nil {
			return nil, false, fmt.Errorf("validation failed: %s", err)
		}

		err = handleEmbedded(msg)
		if err != nil {
			return nil, false, fmt.Errorf("error handling embedded messages: %s", err)
		}

		new, err = process(msg)
		if err != nil {
			return nil, false, fmt.Errorf("error processing message: %s", err)
		}

		return nil, new, nil
	}
}

func makeEmbeddedMessageHandler(handle messageHandler) embeddedMessageHandler {
	return func(msg messages.Message) error {
		handleOne := func(m messages.Message) error {
			if _, _, err := handle(m); err != nil {
				return fmt.Errorf("error handling %s: %s", messages.Stringify(m), err)
			}
			return nil
		}

		switch msg := msg.(type) {
		case messages.Request:
		case messages.Prepare:
			return handleOne(msg.Request())
		case messages.Commit:
			return handleOne(msg.Proposal())
		case messages.ReqViewChange:
		case messages.ViewChange:
			for _, m := range msg.ViewChangeCert() {
				if err := handleOne(m); err != nil {
					return err
				}
			}
			for _, m := range msg.MessageLog() {
				if err := handleOne(m); err != nil {
					return err
				}
			}
		case messages.NewView:
			for _, m := range msg.NewViewCert() {
				if err := handleOne(m); err != nil {
					return err
				}
			}
		default:
			panic("Unknown message type")
		}

		return nil
	}
}

func makeClientMessageHandler(validateRequest requestValidator, processRequest requestProcessor, replyRequest requestReplier) messageHandler {
	return func(msg messages.Message) (_ <-chan messages.Message, new bool, err error) {
		req, ok := msg.(messages.Request)
		if !ok {
			return nil, false, fmt.Errorf("unexpected message type")
		}

		err = validateRequest(req)
		if err != nil {
			return nil, false, fmt.Errorf("invalid Reqeust: %s", err)
		}

		new, err = processRequest(req)
		if err != nil {
			return nil, false, fmt.Errorf("error processing Request: %s", err)
		}

		replyChan := make(chan messages.Message, 1)
		defer close(replyChan)

		if reply, ok := <-replyRequest(req); ok {
			replyChan <- reply
		}

		return replyChan, new, nil
	}
}

// makeMessageValidator constructs an instance of messageValidator
// using the supplied abstractions.
func makeMessageValidator(validateRequest requestValidator, validatePeerMessage peerMessageValidator) messageValidator {
	return func(msg messages.Message) error {
		switch msg := msg.(type) {
		case messages.Request:
			return validateRequest(msg)
		case messages.PeerMessage:
			return validatePeerMessage(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makePeerMessageValidator(n uint32, validateCertified certifiedMessageValidator, validateReqViewChange reqViewChangeValidator) peerMessageValidator {
	return func(msg messages.PeerMessage) error {
		replicaID := msg.ReplicaID()
		if replicaID >= n {
			return fmt.Errorf("unexpected replica ID")
		}

		switch msg := msg.(type) {
		case messages.CertifiedMessage:
			return validateCertified(msg)
		case messages.ReqViewChange:
			return validateReqViewChange(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makeCertifiedMessageValidator(validatePrepare prepareValidator, validateCommit commitValidator, validateViewChange viewChangeValidator, validateNewView newViewValidator, verifyUI uiVerifier) certifiedMessageValidator {
	return func(msg messages.CertifiedMessage) error {
		if err := verifyUI(msg); err != nil {
			return fmt.Errorf("invalid UI: %s", err)
		}

		switch msg := msg.(type) {
		case messages.Prepare:
			return validatePrepare(msg)
		case messages.Commit:
			return validateCommit(msg)
		case messages.ViewChange:
			return validateViewChange(msg)
		case messages.NewView:
			return validateNewView(msg)
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageProcessor constructs an instance of messageProcessor
// using the supplied abstractions.
func makeMessageProcessor(processRequest requestProcessor, processPeerMessage peerMessageProcessor) messageProcessor {
	return func(msg messages.Message) (new bool, err error) {
		switch msg := msg.(type) {
		case messages.Request:
			return processRequest(msg)
		case messages.PeerMessage:
			return processPeerMessage(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makePeerMessageProcessor(n uint32, processCertifiedMessage certifiedMessageProcessor, processReqViewChange reqViewChangeProcessor) peerMessageProcessor {
	locks := make([]sync.Mutex, n)

	return func(msg messages.PeerMessage) (new bool, err error) {
		lock := &locks[msg.ReplicaID()]
		lock.Lock()
		defer lock.Unlock()

		switch msg := msg.(type) {
		case messages.CertifiedMessage:
			return processCertifiedMessage(msg)
		case messages.ReqViewChange:
			return processReqViewChange(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makeCertifiedMessageProcessor(n uint32, processViewMessage viewMessageProcessor, processViewChange viewChangeProcessor) certifiedMessageProcessor {
	lastUI := make([]uint64, n)

	return func(msg messages.CertifiedMessage) (new bool, err error) {
		replicaID := msg.ReplicaID()
		ui := msg.UI()

		nextUI := lastUI[replicaID] + 1
		if ui.Counter < nextUI {
			return false, nil
		} else if ui.Counter > nextUI {
			return false, fmt.Errorf("unexpected UI counter value")
		}

		switch msg := msg.(type) {
		case messages.ViewChange:
			new, err = processViewChange(msg)
		case messages.PeerMessage:
			new, err = processViewMessage(msg)
		default:
			panic("Unknown message type")
		}

		if err == nil {
			lastUI[replicaID] = nextUI
		}

		return new, err
	}
}

func makeViewMessageProcessor(viewState viewstate.State, applyPeerMessage peerMessageApplier) viewMessageProcessor {
	return func(msg messages.PeerMessage) (new bool, err error) {
		var messageView, expectedView uint64

		switch msg := msg.(type) {
		case messages.Prepare, messages.Commit:
			switch msg := msg.(type) {
			case messages.Prepare:
				messageView = msg.View()
			case messages.Commit:
				switch prop := msg.Proposal().(type) {
				case messages.Prepare:
					messageView = prop.View()
				case messages.NewView:
					messageView = prop.NewView()
				default:
					panic("Unknown proposal message type")
				}
			}

			var currentView uint64
			var release func()
			currentView, expectedView, release = viewState.HoldView()
			defer release()

			if messageView > currentView {
				// A correct replica must first ensure
				// transition into the new view.
				return false, fmt.Errorf("message refers to unexpected view")
			}
		case messages.NewView:
			messageView = msg.NewView()

			var ok bool
			var release func()
			ok, expectedView, release = viewState.AdvanceCurrentView(messageView)
			if !ok {
				return false, nil
			}
			defer release()
		default:
			panic("Unknown message type")
		}

		if messageView != expectedView {
			return false, nil
		}

		if err := applyPeerMessage(msg); err != nil {
			return false, fmt.Errorf("failed to apply message: %s", err)
		}

		return true, nil
	}
}

// makePeerMessageApplier constructs an instance of peerMessageApplier using
// the supplied abstractions.
func makePeerMessageApplier(applyPrepare prepareApplier, applyCommit commitApplier, applyNewView newViewApplier) peerMessageApplier {
	return func(msg messages.PeerMessage) error {
		switch msg := msg.(type) {
		case messages.Prepare:
			return applyPrepare(msg)
		case messages.Commit:
			return applyCommit(msg)
		case messages.NewView:
			return applyNewView(msg)
		default:
			panic("Unknown message type")
		}
	}
}

// makeGeneratedMessageHandler constructs generatedMessageHandler
// using the supplied abstractions.
func makeGeneratedMessageHandler(sign messageSigner, assignUI uiAssigner, consume generatedMessageConsumer) generatedMessageHandler {
	var uiLock sync.Mutex

	return func(msg messages.ReplicaMessage) {
		switch msg := msg.(type) {
		case messages.CertifiedMessage:
			uiLock.Lock()
			defer uiLock.Unlock()

			assignUI(msg)
		case messages.SignedMessage:
			sign(msg)
		}

		consume(msg)
	}
}

func makeGeneratedMessageConsumer(log messagelog.MessageLog, clientStates clientstate.Provider, logger logger.Logger) generatedMessageConsumer {
	return func(msg messages.ReplicaMessage) {
		logger.Debugf("Generated %s", messages.Stringify(msg))

		switch msg := msg.(type) {
		case messages.Reply:
			clientStates.ClientState(msg.ClientID()).AddReply(msg)
		case messages.ReplicaMessage:
			log.Append(msg)
		default:
			panic("Unknown message type")
		}
	}
}
