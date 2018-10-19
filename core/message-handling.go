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

// messageHandler fully handles a message. If there is any message
// produced in reply, it will be send to reply channel, otherwise nil
// channel is returned. The return value new indicates that the
// message hasn't been processed before.
type messageHandler func(msg interface{}) (reply <-chan interface{}, new bool, err error)

// messageConsumer receives a generated message.
//
// The supplied message should be ready for delivery to the
// recipients. It arranges the message to be delivered to peer
// replicas or the corresponding client, depending on the message
// type.
type messageConsumer func(msg interface{})

// generatedUIMessageHandler assigns and attaches a UI to a generated
// message and arranges it to be delivered to peer replicas. It is
// safe to invoke concurrently.
type generatedUIMessageHandler func(msg messages.MessageWithUI)

// defaultMessageHandler construct a standard messageHandler using id
// as the current replica ID and the supplied interfaces.
func defaultMessageHandler(id uint32, log messagelog.MessageLog, config api.Configer, stack Stack, logger *logging.Logger) messageHandler {
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
	releaseSeq := makeRequestSeqReleaser(clientStates)
	prepareSeq := makeRequestSeqPreparer(clientStates)
	retireSeq := makeRequestSeqRetirer(clientStates)
	captureUI := makeUICapturer(peerStates)

	consumeMessage := makeMessageConsumer(log, clientStates, logger)

	countCommits := makeCommitCounter(f)
	executeOperation := makeOperationExecutor(stack)
	executeRequest := makeRequestExecutor(id, executeOperation, signMessage, consumeMessage)
	collectCommit := makeCommitCollector(countCommits, retireSeq, executeRequest)

	handleGeneratedUIMessage := makeGeneratedUIMessageHandler(assignUI, consumeMessage)

	validateRequest := makeRequestValidator(verifyMessageSignature)
	processRequest := makeRequestProcessor(id, n, view, captureSeq, releaseSeq, prepareSeq, handleGeneratedUIMessage)

	handleRequest := makeRequestHandler(validateRequest, processRequest)
	replyRequest := makeRequestReplier(clientStates)
	handlePrepare := makePrepareHandler(id, n, view, verifyUI, captureUI, prepareSeq, handleRequest, collectCommit, handleGeneratedUIMessage)
	handleCommit := makeCommitHandler(id, n, view, verifyUI, captureUI, handlePrepare, collectCommit)

	return makeMessageHandler(handleRequest, replyRequest, handlePrepare, handleCommit)
}

// makeMessageStreamHandler construct an instance of
// messageStreamHandler using the supplied abstract handler.
func makeMessageStreamHandler(handle messageHandler, logger *logging.Logger) messageStreamHandler {
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

// makeMessageHandler construct an instance of messageHandler using
// the supplied abstract handlers.
func makeMessageHandler(handleRequest requestHandler, replyRequest requestReplier, handlePrepare prepareHandler, handleCommit commitHandler) messageHandler {
	return func(msg interface{}) (reply <-chan interface{}, new bool, err error) {
		switch msg := msg.(type) {
		case *messages.Request:
			outChan := make(chan interface{})
			new, err := handleRequest(msg)
			if err != nil {
				return nil, false, err
			}
			go func() {
				defer close(outChan)
				if m, more := <-replyRequest(msg); more {
					outChan <- m
				}
			}()
			return outChan, new, nil
		case *messages.Prepare:
			new, err := handlePrepare(msg)
			if err != nil {
				return nil, false, err
			}
			return nil, new, nil
		case *messages.Commit:
			new, err := handleCommit(msg)
			if err != nil {
				return nil, false, err
			}
			return nil, new, nil
		default:
			panic("Unknown message type")
		}
	}
}

// makeMessageConsumer constructs an instance of messageConsumer using
// the supplied abstractions.
func makeMessageConsumer(log messagelog.MessageLog, provider clientstate.Provider, logger *logging.Logger) messageConsumer {
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
func makeGeneratedUIMessageHandler(assignUI uiAssigner, consume messageConsumer) generatedUIMessageHandler {
	var lock sync.Mutex

	return func(msg messages.MessageWithUI) {
		lock.Lock()
		defer lock.Unlock()

		assignUI(msg)
		consume(msg)
	}
}
