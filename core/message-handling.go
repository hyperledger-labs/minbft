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

	logging "github.com/op/go-logging"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/peerstate"
	"github.com/hyperledger-labs/minbft/core/internal/requestlist"
	"github.com/hyperledger-labs/minbft/core/internal/viewstate"
	"github.com/hyperledger-labs/minbft/messages"
)

// messageStreamHandler fetches serialized messages from in channel,
// handles the received messages, and sends a serialized reply
// message, if any, to reply channel.
type messageStreamHandler func(in <-chan []byte, reply chan<- []byte)

// incomingMessageHandler fully handles incoming message.
//
// If there is any message produced in reply, it will be send to reply
// channel, otherwise nil channel is returned. The return value new
// indicates that the message has not been processed before.
type incomingMessageHandler func(msg messages.Message) (reply <-chan messages.Message, new bool, err error)

// peerMessageSupplier supplies messages for peer replica.
//
// Given a channel, it supplies the channel with messages to be
// delivered to the peer replica.
type peerMessageSupplier func(out chan<- []byte)

// peerConnector initiates message exchange with a peer replica.
//
// Given a channel of outgoing messages to supply to the replica, it
// returns a channel of messages produced by the replica in reply.
type peerConnector func(out <-chan []byte) (in <-chan []byte, err error)

// messageValidator validates a message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type messageValidator func(msg messages.Message) error

// messageProcessor processes a valid message.
//
// It fully processes the supplied message in the context of the
// current replica's state. The supplied message is assumed to be
// authentic and internally consistent. The return value new indicates
// if the message had any effect. It is safe to invoke concurrently.
type messageProcessor func(msg messages.Message) (new bool, err error)

// replicaMessageProcessor processes a valid replica message.
//
// It continues processing of the supplied replica message. Messages
// originated from the current replica are assumed to be already
// processed. The return value new indicates if the message had any
// effect. It is safe to invoke concurrently.
type replicaMessageProcessor func(msg messages.ReplicaMessage) (new bool, err error)

// uiMessageProcessor processes a valid message with UI.
//
// It continues processing of the supplied message with UI. Messages
// originated from the same replica are guaranteed to be processed
// once only and in the sequence assigned by the replica USIG. The
// return value new indicates if the message had any effect. It is
// safe to invoke concurrently.
type uiMessageProcessor func(msg messages.CertifiedMessage) (new bool, err error)

// viewMessageProcessor processes a valid message in current view.
//
// It continues processing of the supplied message, according to the
// current view number. The message is guaranteed to be processed in a
// corresponding view, or not processed at all. The return value new
// indicates if the message had any effect. It is safe to invoke
// concurrently.
type viewMessageProcessor func(msg messages.ReplicaMessage) (new bool, err error)

// replicaMessageApplier applies a replica message to current replica
// state.
//
// The supplied message is applied to the current replica state by
// changing the state accordingly and producing any required messages
// or side effects. The supplied message is assumed to be authentic
// and internally consistent. It is safe to invoke concurrently.
type replicaMessageApplier func(msg messages.ReplicaMessage) error

// messageReplier provides reply to a valid message.
//
// If there is any message to be produced in reply to the supplied
// one, it will be send to the returned reply channel, otherwise nil
// channel is returned. The supplied message is assumed to be
// authentic and internally consistent. It is safe to invoke
// concurrently.
type messageReplier func(msg messages.Message) (reply <-chan messages.Message, err error)

// generatedUIMessageHandler assigns UI and handles generated message.
//
// It assigns and attaches a UI to the supplied message, then takes
// further steps to handle the message. It is safe to invoke
// concurrently.
type generatedUIMessageHandler func(msg messages.CertifiedMessage)

// generatedMessageHandler handles generated message.
//
// It handles the supplied message, generated by the current replica.
// The message is assumed to be completely initialized. It is safe to
// invoke concurrently.
type generatedMessageHandler func(msg messages.ReplicaMessage)

// generatedMessageConsumer receives generated message.
//
// It arranges the supplied message to be delivered to peer replicas
// or the corresponding client, depending on the message type. The
// message should be ready to serialize and deliver to the recipients.
// It is safe to invoke concurrently.
type generatedMessageConsumer func(msg messages.ReplicaMessage)

// defaultIncomingMessageHandler construct a standard
// incomingMessageHandler using id as the current replica ID and the
// supplied interfaces.
func defaultIncomingMessageHandler(id uint32, log messagelog.MessageLog, config api.Configer, stack Stack, logger *logging.Logger) incomingMessageHandler {
	n := config.N()
	f := config.F()

	reqTimeout := makeRequestTimeoutProvider(config)
	prepTimeout := makePrepareTimeoutProvider(config)
	handleReqTimeout := func(view uint64) {
		logger.Panic("Request timed out, but view change not implemented")
	}

	verifyMessageSignature := makeMessageSignatureVerifier(stack)
	signMessage := makeReplicaMessageSigner(stack)
	verifyUI := makeUIVerifier(stack)
	assignUI := makeUIAssigner(stack)

	clientStates := clientstate.NewProvider(reqTimeout, prepTimeout)
	peerStates := peerstate.NewProvider()
	viewState := viewstate.New()

	captureSeq := makeRequestSeqCapturer(clientStates)
	prepareSeq := makeRequestSeqPreparer(clientStates)
	retireSeq := makeRequestSeqRetirer(clientStates)
	pendingReq := requestlist.New()
	startReqTimer := makeRequestTimerStarter(clientStates, handleReqTimeout, logger)
	stopReqTimer := makeRequestTimerStopper(clientStates)
	captureUI := makeUICapturer(peerStates)
	provideView := viewState.WaitAndHoldActiveView
	waitView := viewState.WaitAndHoldView

	var applyReplicaMessage replicaMessageApplier

	// Due to recursive nature of replica messages application, an
	// instance of replicaMessageApplier is eventually required
	// for it to be constructed itself. On the other hand, it will
	// actually be invoked only after getting fully constructed.
	// This "thunk" delays evaluation of applyReplicaMessage
	// variable, thus resolving this circular dependency.
	applyReplicaMessageThunk := func(msg messages.ReplicaMessage) error {
		return applyReplicaMessage(msg)
	}

	consumeGeneratedMessage := makeGeneratedMessageConsumer(log, clientStates)
	handleGeneratedMessage := makeGeneratedMessageHandler(applyReplicaMessageThunk, consumeGeneratedMessage, logger)
	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	countCommitment := makeCommitmentCounter(f)
	executeOperation := makeOperationExecutor(stack)
	executeRequest := makeRequestExecutor(id, executeOperation, signMessage, handleGeneratedMessage)
	collectCommitment := makeCommitmentCollector(countCommitment, retireSeq, pendingReq, stopReqTimer, executeRequest)

	validateRequest := makeRequestValidator(verifyMessageSignature)
	validatePrepare := makePrepareValidator(n, verifyUI, validateRequest)
	validateCommit := makeCommitValidator(verifyUI, validatePrepare)
	validateMessage := makeMessageValidator(validateRequest, validatePrepare, validateCommit)

	startPrepTimer := makePrepareTimerStarter(clientStates, logger)
	stopPrepTimer := makePrepareTimerStopper(clientStates)

	applyCommit := makeCommitApplier(collectCommitment)
	applyPrepare := makePrepareApplier(id, prepareSeq, collectCommitment, handleGeneratedUIMessage, stopPrepTimer)
	applyReplicaMessage = makeReplicaMessageApplier(applyPrepare, applyCommit)
	applyRequest := makeRequestApplier(id, n, provideView, handleGeneratedUIMessage, startReqTimer, startPrepTimer)

	var processMessage messageProcessor

	// Due to recursive nature of message processing, an instance
	// of messageProcessor is eventually required for it to be
	// constructed itself. On the other hand, it will actually be
	// invoked only after getting fully constructed. This "thunk"
	// delays evaluation of processMessage variable, thus
	// resolving this circular dependency.
	processMessageThunk := func(msg messages.Message) (new bool, err error) {
		return processMessage(msg)
	}

	processRequest := makeRequestProcessor(captureSeq, pendingReq, applyRequest)
	processViewMessage := makeViewMessageProcessor(waitView, applyReplicaMessage)
	processUIMessage := makeUIMessageProcessor(captureUI, processViewMessage)
	processReplicaMessage := makeReplicaMessageProcessor(id, processMessageThunk, processUIMessage)
	processMessage = makeMessageProcessor(processRequest, processReplicaMessage)

	replyRequest := makeRequestReplier(clientStates)
	replyMessage := makeMessageReplier(replyRequest)

	return makeIncomingMessageHandler(validateMessage, processMessage, replyMessage)
}

// makeMessageStreamHandler construct an instance of
// messageStreamHandler using the supplied abstract handler.
func makeMessageStreamHandler(handle incomingMessageHandler, logger *logging.Logger) messageStreamHandler {
	return func(in <-chan []byte, reply chan<- []byte) {
		for msgBytes := range in {
			msg, err := messageImpl.NewFromBinary(msgBytes)
			if err != nil {
				logger.Warningf("Failed to unmarshal message: %s", err)
				continue
			}

			msgStr := messageString(msg)

			logger.Debugf("Received %s", msgStr)

			if replyChan, new, err := handle(msg); err != nil {
				logger.Warningf("Failed to handle %s: %s", msgStr, err)
			} else if replyChan != nil {
				m, more := <-replyChan
				if !more {
					continue
				}
				replyBytes, err := m.MarshalBinary()
				if err != nil {
					panic(err)
				}
				reply <- replyBytes
			} else if !new {
				logger.Infof("Dropped %s", msgStr)
			} else {
				logger.Debugf("Handled %s", msgStr)
			}
		}
	}
}

// startPeerConnections initiates asynchronous message exchange with
// peer replicas.
func startPeerConnections(replicaID, n uint32, connector api.ReplicaConnector, log messagelog.MessageLog, logger *logging.Logger) error {
	supply := makePeerMessageSupplier(log)

	for peerID := uint32(0); peerID < n; peerID++ {
		if peerID == replicaID {
			continue
		}

		connect := makePeerConnector(peerID, connector)
		if err := startPeerConnection(connect, supply); err != nil {
			return fmt.Errorf("Cannot connect to replica %d: %s", peerID, err)
		}
	}

	return nil
}

// startPeerConnection initiates asynchronous message exchange with a
// peer replica.
func startPeerConnection(connect peerConnector, supply peerMessageSupplier) error {
	out := make(chan []byte)

	// So far, reply stream is not used for replica-to-replica
	// communication, thus return value is ignored. Each replica
	// will establish connections to other peers the same way, so
	// they all will be eventually fully connected.
	if _, err := connect(out); err != nil {
		return err
	}

	go supply(out)

	return nil
}

// makePeerMessageSupplier construct a peerMessageSupplier using the
// supplied message log.
func makePeerMessageSupplier(log messagelog.MessageLog) peerMessageSupplier {
	return func(out chan<- []byte) {
		for msg := range log.Stream(nil) {
			msgBytes, err := msg.MarshalBinary()
			if err != nil {
				panic(err)
			}
			out <- msgBytes
		}
	}
}

// makePeerConnector constructs a peerConnector using the supplied
// peer replica ID and a general replica connector.
func makePeerConnector(peerID uint32, connector api.ReplicaConnector) peerConnector {
	return func(out <-chan []byte) (in <-chan []byte, err error) {
		sh := connector.ReplicaMessageStreamHandler(peerID)
		if sh == nil {
			return nil, fmt.Errorf("Connection not possible")
		}
		return sh.HandleMessageStream(out), nil
	}
}

// makeIncomingMessageHandler constructs an instance of
// incomingMessageHandler using id as the current replica ID, and the
// supplied abstractions.
func makeIncomingMessageHandler(validate messageValidator, process messageProcessor, reply messageReplier) incomingMessageHandler {
	return func(msg messages.Message) (replyChan <-chan messages.Message, new bool, err error) {
		err = validate(msg)
		if err != nil {
			err = fmt.Errorf("Validation failed: %s", err)
			return nil, false, err
		}

		new, err = process(msg)
		if err != nil {
			err = fmt.Errorf("Error processing message: %s", err)
			return nil, false, err
		}

		replyChan, err = reply(msg)
		if err != nil {
			err = fmt.Errorf("Error replying message: %s", err)
			return nil, false, err
		}

		return replyChan, new, nil
	}
}

// makeMessageValidator constructs an instance of messageValidator
// using the supplied abstractions.
func makeMessageValidator(validateRequest requestValidator, validatePrepare prepareValidator, validateCommit commitValidator) messageValidator {
	return func(msg messages.Message) error {
		switch msg := msg.(type) {
		case messages.Request:
			return validateRequest(msg)
		case messages.Prepare:
			return validatePrepare(msg)
		case messages.Commit:
			return validateCommit(msg)
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageProcessor constructs an instance of messageProcessor
// using the supplied abstractions.
func makeMessageProcessor(processRequest requestProcessor, processReplicaMessage replicaMessageProcessor) messageProcessor {
	return func(msg messages.Message) (new bool, err error) {
		switch msg := msg.(type) {
		case messages.Request:
			return processRequest(msg)
		case messages.ReplicaMessage:
			return processReplicaMessage(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makeReplicaMessageProcessor(id uint32, process messageProcessor, processUIMessage uiMessageProcessor) replicaMessageProcessor {
	return func(msg messages.ReplicaMessage) (new bool, err error) {
		if msg.ReplicaID() == id {
			return false, nil
		}

		for _, m := range messages.EmbeddedMessages(msg) {
			if _, err := process(m); err != nil {
				return false, fmt.Errorf("Failed to process embedded message: %s", err)
			}
		}

		switch msg := msg.(type) {
		case messages.CertifiedMessage:
			return processUIMessage(msg)
		default:
			panic("Unknown message type")
		}
	}
}

func makeUIMessageProcessor(captureUI uiCapturer, processViewMessage viewMessageProcessor) uiMessageProcessor {
	return func(msg messages.CertifiedMessage) (new bool, err error) {
		new, release := captureUI(msg)
		if !new {
			return false, nil
		}
		defer release()

		return processViewMessage(msg)
	}
}

func makeViewMessageProcessor(waitView viewWaiter, applyReplicaMessage replicaMessageApplier) viewMessageProcessor {
	return func(msg messages.ReplicaMessage) (new bool, err error) {
		switch msg := msg.(type) {
		case messages.Prepare, messages.Commit:
			var view uint64

			switch msg := msg.(type) {
			case messages.Prepare:
				view = msg.View()
			case messages.Commit:
				view = msg.Prepare().View()
			}

			ok, release := waitView(view)
			if !ok {
				return false, nil
			}
			defer release()
		default:
			panic("Unknown message type")
		}

		if err := applyReplicaMessage(msg); err != nil {
			return false, fmt.Errorf("Failed to apply message: %s", err)
		}

		return true, nil
	}
}

// makeReplicaMessageApplier constructs an instance of replicaMessageApplier using
// the supplied abstractions.
func makeReplicaMessageApplier(applyPrepare prepareApplier, applyCommit commitApplier) replicaMessageApplier {
	return func(msg messages.ReplicaMessage) error {
		switch msg := msg.(type) {
		case messages.Prepare:
			return applyPrepare(msg)
		case messages.Commit:
			return applyCommit(msg)
		case messages.Reply:
			return nil
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageReplier constructs an instance of messageReplier using
// the supplied abstractions.
func makeMessageReplier(replyRequest requestReplier) messageReplier {
	return func(msg messages.Message) (reply <-chan messages.Message, err error) {
		outChan := make(chan messages.Message)

		switch msg := msg.(type) {
		case messages.Request:
			go func() {
				defer close(outChan)
				if m, more := <-replyRequest(msg); more {
					outChan <- m
				}
			}()
			return outChan, nil
		case messages.Prepare, messages.Commit:
			return nil, nil
		default:
			panic("Unknown message type")
		}
	}
}

// makeGeneratedMessageHandler constructs an instance of
// generatedMessageHandler using the supplied abstractions.
func makeGeneratedMessageHandler(apply replicaMessageApplier, consume generatedMessageConsumer, logger *logging.Logger) generatedMessageHandler {
	return func(msg messages.ReplicaMessage) {
		logger.Debugf("Generated %s", messageString(msg))

		if err := apply(msg); err != nil {
			panic(fmt.Errorf("Failed to apply generated message: %s", err))
		}

		consume(msg)
	}
}

// makeGeneratedUIMessageHandler constructs generatedUIMessageHandler
// using the supplied abstractions.
func makeGeneratedUIMessageHandler(assignUI uiAssigner, handle generatedMessageHandler) generatedUIMessageHandler {
	var lock sync.Mutex

	return func(msg messages.CertifiedMessage) {
		lock.Lock()
		defer lock.Unlock()

		assignUI(msg)
		handle(msg)
	}
}

func makeGeneratedMessageConsumer(log messagelog.MessageLog, provider clientstate.Provider) generatedMessageConsumer {
	return func(msg messages.ReplicaMessage) {
		switch msg := msg.(type) {
		case messages.Reply:
			clientID := msg.ClientID()
			if err := provider(clientID).AddReply(msg); err != nil {
				// Erroneous Reply must never be supplied
				panic(fmt.Errorf("Failed to consume generated Reply: %s", err))
			}
		case messages.ReplicaMessage:
			log.Append(msg)
		default:
			panic("Unknown message type")
		}
	}
}
