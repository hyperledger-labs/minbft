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

	"github.com/golang/protobuf/proto"

	logging "github.com/op/go-logging"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
	"github.com/hyperledger-labs/minbft/core/internal/peerstate"
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
type incomingMessageHandler func(msg interface{}) (reply <-chan interface{}, new bool, err error)

// messageValidator validates a message.
//
// It authenticates and checks the supplied message for internal
// consistency. It does not use replica's current state and has no
// side-effect. It is safe to invoke concurrently.
type messageValidator func(msg interface{}) error

// messageProcessor processes a valid message.
//
// It fully processes the supplied message in the context of the
// current replica's state. The supplied message is assumed to be
// authentic and internally consistent. The return value new indicates
// if the message has not been processed by this replica before. It is
// safe to invoke concurrently.
type messageProcessor func(msg interface{}) (new bool, err error)

// messageReplier provides reply to a valid message.
//
// If there is any message to be produced in reply to the supplied
// one, it will be send to the returned reply channel, otherwise nil
// channel is returned. The supplied message is assumed to be
// authentic and internally consistent. It is safe to invoke
// concurrently.
type messageReplier func(msg interface{}) (reply <-chan interface{}, err error)

// generatedMessageHandler handles generated message.
//
// It arranges the supplied message to be delivered to peer replicas,
// or the corresponding client, depending on the message type. The
// message should be ready to serialize and deliver to the recipients.
type generatedMessageHandler func(msg interface{})

// generatedUIMessageHandler assigns and attaches a UI to a generated
// message and arranges it to be delivered to peer replicas. It is
// safe to invoke concurrently.
type generatedUIMessageHandler func(msg messages.MessageWithUI)

// defaultIncomingMessageHandler construct a standard
// incomingMessageHandler using id as the current replica ID and the
// supplied interfaces.
func defaultIncomingMessageHandler(id uint32, log messagelog.MessageLog, config api.Configer, stack Stack, logger *logging.Logger) incomingMessageHandler {
	n := config.N()
	f := config.F()

	view := func() uint64 { return 0 } // view change is not implemented

	verifyMessageSignature := makeMessageSignatureVerifier(stack)
	signMessage := makeReplicaMessageSigner(stack)
	verifyUI := makeUIVerifier(stack)
	assignUI := makeUIAssigner(stack)

	clientStates := clientstate.NewProvider()
	peerStates := peerstate.NewProvider()

	captureSeq := makeRequestSeqCapturer(clientStates)
	prepareSeq := makeRequestSeqPreparer(clientStates)
	retireSeq := makeRequestSeqRetirer(clientStates)
	captureUI := makeUICapturer(peerStates)

	handleGeneratedMessage := makeGeneratedMessageHandler(log, clientStates, logger)

	countCommitment := makeCommitmentCounter(f)
	executeOperation := makeOperationExecutor(stack)
	executeRequest := makeRequestExecutor(id, executeOperation, signMessage, handleGeneratedMessage)
	collectCommitment := makeCommitmentCollector(countCommitment, retireSeq, executeRequest)

	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, handleGeneratedMessage)

	validateRequest := makeRequestValidator(verifyMessageSignature)
	validatePrepare := makePrepareValidator(n, verifyUI, validateRequest)
	validateCommit := makeCommitValidator(verifyUI, validatePrepare)
	validateMessage := makeMessageValidator(validateRequest, validatePrepare, validateCommit)

	applyCommit := makeCommitApplier(collectCommitment)
	applyPrepare := makePrepareApplier(id, prepareSeq, collectCommitment, handleGeneratedUIMessage, applyCommit)
	applyRequest := makeRequestApplier(id, n, view, handleGeneratedUIMessage, applyPrepare)

	processRequest := makeRequestProcessor(captureSeq, applyRequest)
	processPrepare := makePrepareProcessor(id, processRequest, captureUI, view, applyPrepare)
	processCommit := makeCommitProcessor(id, processPrepare, captureUI, view, applyCommit)
	processMessage := makeMessageProcessor(processRequest, processPrepare, processCommit)

	replyRequest := makeRequestReplier(clientStates)
	replyMessage := makeMessageReplier(replyRequest)

	return makeIncomingMessageHandler(validateMessage, processMessage, replyMessage)
}

// makeMessageStreamHandler construct an instance of
// messageStreamHandler using the supplied abstract handler.
func makeMessageStreamHandler(handle incomingMessageHandler, logger *logging.Logger) messageStreamHandler {
	return func(in <-chan []byte, reply chan<- []byte) {
		for msgBytes := range in {
			wrappedMsg := &messages.Message{}
			if err := proto.Unmarshal(msgBytes, wrappedMsg); err != nil {
				logger.Warningf("Failed to unmarshal message: %s", err)
				continue
			}

			msg := messages.UnwrapMessage(wrappedMsg)
			msgStr := messageString(msg)

			logger.Debugf("Received %s", msgStr)

			if replyChan, new, err := handle(msg); err != nil {
				logger.Warningf("Failed to handle %s: %s", msgStr, err)
			} else if replyChan != nil {
				m, more := <-replyChan
				if !more {
					continue
				}
				replyMsg := messages.WrapMessage(m)
				replyBytes, err := proto.Marshal(replyMsg)
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

// makeIncomingMessageHandler constructs an instance of
// incomingMessageHandler using id as the current replica ID, and the
// supplied abstractions.
func makeIncomingMessageHandler(validate messageValidator, process messageProcessor, reply messageReplier) incomingMessageHandler {
	return func(msg interface{}) (replyChan <-chan interface{}, new bool, err error) {
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
	return func(msg interface{}) error {
		switch msg := msg.(type) {
		case *messages.Request:
			return validateRequest(msg)
		case *messages.Prepare:
			return validatePrepare(msg)
		case *messages.Commit:
			return validateCommit(msg)
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageProcessor constructs an instance of messageProcessor
// using the supplied abstractions.
func makeMessageProcessor(processRequest requestProcessor, processPrepare prepareProcessor, processCommit commitProcessor) messageProcessor {
	return func(msg interface{}) (new bool, err error) {
		switch msg := msg.(type) {
		case *messages.Request:
			return processRequest(msg)
		case *messages.Prepare:
			return processPrepare(msg)
		case *messages.Commit:
			return processCommit(msg)
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageReplier constructs an instance of messageReplier using
// the supplied abstractions.
func makeMessageReplier(replyRequest requestReplier) messageReplier {
	return func(msg interface{}) (reply <-chan interface{}, err error) {
		outChan := make(chan interface{})

		switch msg := msg.(type) {
		case *messages.Request:
			go func() {
				defer close(outChan)
				if m, more := <-replyRequest(msg); more {
					outChan <- m
				}
			}()
			return outChan, nil
		case *messages.Prepare, *messages.Commit:
			return nil, nil
		default:
			panic("Unknown message type")
		}
	}
}

// makeGeneratedMessageHandler constructs an instance of
// generatedMessageHandler using the supplied abstractions.
func makeGeneratedMessageHandler(log messagelog.MessageLog, provider clientstate.Provider, logger *logging.Logger) generatedMessageHandler {
	return func(msg interface{}) {
		logger.Debugf("Generated %s", messageString(msg))

		switch msg := msg.(type) {
		case *messages.Reply:
			clientID := msg.Msg.ClientId
			if err := provider(clientID).AddReply(msg); err != nil {
				panic(err) // Erroneous Reply must never be supplied
			}
		case *messages.Prepare, *messages.Commit:
			log.Append(messages.WrapMessage(msg))
		default:
			panic("Unknown message type")
		}
	}
}

// makeGeneratedUIMessageHandler constructs generatedUIMessageHandler
// using the supplied abstractions.
func makeGeneratedUIMessageHandler(assignUI uiAssigner, handle generatedMessageHandler) generatedUIMessageHandler {
	var lock sync.Mutex

	return func(msg messages.MessageWithUI) {
		lock.Lock()
		defer lock.Unlock()

		assignUI(msg)
		handle(msg)
	}
}
