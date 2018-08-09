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

	"github.com/nec-blockchain/minbft/api"
	"github.com/nec-blockchain/minbft/core/internal/clientstate"
	"github.com/nec-blockchain/minbft/core/internal/messagelog"
	"github.com/nec-blockchain/minbft/core/internal/peerstate"
	"github.com/nec-blockchain/minbft/messages"
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

// generatedUIMessageHandler assigns and attaches a UI to a generated
// message and arranges it to be delivered to peer replicas. It is
// safe to invoke concurrently.
type generatedUIMessageHandler func(msg messages.MessageWithUI)

// uiMessageConsumer receives a generated message with UI attached and
// arranges it to be delivered to peer replicas.
type uiMessageConsumer func(msg messages.MessageWithUI)

// defaultMessageHandler construct a standard messageHandler using id
// as the current replica ID and the supplied interfaces.
func defaultMessageHandler(id uint32, log messagelog.MessageLog, config api.Configer, stack Stack) messageHandler {
	n := config.N()

	clientStates := clientstate.NewProvider()
	peerStates := peerstate.NewProvider()

	view := func() uint64 { return 0 } // view change is not implemented
	verifyUI := makeUIVerifier(stack)
	captureUI := makeUICapturer(peerStates)
	releaseUI := makeUIReleaser(peerStates)
	collectCommit := defaultCommitCollector(id, clientStates, config, stack)
	handleGeneratedUIMessage := defaultGeneratedUIMessageHandler(stack, log)

	handleRequest := defaultRequestHandler(id, n, view, stack, clientStates, handleGeneratedUIMessage)
	handlePrepare := makePrepareHandler(id, n, view, verifyUI, captureUI, handleRequest, collectCommit, handleGeneratedUIMessage, releaseUI)
	handleCommit := makeCommitHandler(id, n, view, verifyUI, captureUI, handlePrepare, collectCommit, releaseUI)

	return makeMessageHandler(handleRequest, handlePrepare, handleCommit)
}

// defaultGeneratedUIMessageHandler construct a standard
// generatedUIMessageHandler using the supplied interfaces.
func defaultGeneratedUIMessageHandler(auth api.Authenticator, log messagelog.MessageLog) generatedUIMessageHandler {
	assignUI := makeUIAssigner(auth)
	consume := makeUIMessageConsumer(log)
	return makeGeneratedUIMessageHandler(assignUI, consume)
}

// makeMessageStreamHandler construct an instance of
// messageStreamHandler using the supplied abstract handler.
func makeMessageStreamHandler(handle messageHandler) messageStreamHandler {
	return func(in <-chan []byte, reply chan<- []byte) {
		for msgBytes := range in {
			msg := &messages.Message{}
			if err := proto.Unmarshal(msgBytes, msg); err != nil {
				logger.Warningf("Failed to unmarshal message: %s", err)
				continue
			}

			if replyChan, new, err := handle(messages.UnwrapMessage(msg)); err != nil {
				logger.Warning(err)
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
				logger.Infof("Dropped message: %v", msg)
			}
		}
	}
}

// makeMessageHandler construct an instance of messageHandler using
// the supplied abstract handlers.
func makeMessageHandler(handleRequest requestHandler, handlePrepare prepareHandler, handleCommit commitHandler) messageHandler {
	return func(msg interface{}) (reply <-chan interface{}, new bool, err error) {
		switch msg := msg.(type) {
		case *messages.Request:
			outChan := make(chan interface{})
			replyChan, new, err := handleRequest(msg, false)
			if err != nil {
				err = fmt.Errorf("Failed to handle Request message: %s", err)
				return nil, false, err
			}
			go func() {
				defer close(outChan)
				if m, more := <-replyChan; more {
					outChan <- m
				}
			}()
			return outChan, new, nil
		case *messages.Prepare:
			new, err := handlePrepare(msg)
			if err != nil {
				err = fmt.Errorf("Failed to handle Prepare message: %s", err)
				return nil, false, err
			}
			return nil, new, nil
		case *messages.Commit:
			new, err := handleCommit(msg)
			if err != nil {
				err = fmt.Errorf("Failed to handle Commit message: %s", err)
				return nil, false, err
			}
			return nil, new, nil
		default:
			panic("Unknown message type")
		}
	}
}

// makeGeneratedUIMessageHandler constructs generatedUIMessageHandler
// using the supplied abstractions.
func makeGeneratedUIMessageHandler(assignUI uiAssigner, consume uiMessageConsumer) generatedUIMessageHandler {
	var lock sync.Mutex

	return func(msg messages.MessageWithUI) {
		lock.Lock()
		defer lock.Unlock()

		assignUI(msg)
		consume(msg)
	}
}

// makeUIMessageConsumer construct uiMessageConsumer using the
// supplied message log as the destination.
func makeUIMessageConsumer(log messagelog.MessageLog) uiMessageConsumer {
	return func(uiMsg messages.MessageWithUI) {
		msg := messages.WrapMessage(uiMsg)
		log.Append(msg)
	}
}
