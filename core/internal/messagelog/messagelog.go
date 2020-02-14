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

// Package messagelog provides a storage for an ordered set of
// messages.
package messagelog

import (
	"sync"

	"github.com/hyperledger-labs/minbft/messages"
)

// MessageLog represents the message storage. It allows to
// asynchronously append messages, as well as to obtain multiple
// independent asynchronous streams to receive new messages from. Each
// of the streams provides an ordered sequence of all messages as they
// appear in the log. All methods are safe to invoke concurrently.
//
// Append appends a new message to the log. It will never be blocked
// by any of the message streams.
//
// Stream returns an independent channel to receive all messages as
// they appear in the log. Closing the channel passed to this function
// indicates the returned channel should be closed. Nil channel may be
// passed if there's no need to close the returned channel.
//
// Messages returns all messages currently in the log.
//
// Reset replaces messages stored in the log with the supplied ones.
type MessageLog interface {
	Append(msg messages.Message)
	Stream(done <-chan struct{}) <-chan messages.Message
	Messages() []messages.Message
	Reset(msgs []messages.Message)
}

type messageLog struct {
	sync.RWMutex

	// Messages in order added
	msgs []messages.Message

	// Channel to close and recreate when adding new messages
	newAdded chan struct{}

	// Channel to close and recreate on reset
	resetChan chan struct{}
}

// New creates a new instance of the message log.
func New() MessageLog {
	return &messageLog{
		newAdded:  make(chan struct{}),
		resetChan: make(chan struct{}),
	}
}

func (log *messageLog) Append(msg messages.Message) {
	log.Lock()
	defer log.Unlock()

	log.msgs = append(log.msgs, msg)
	close(log.newAdded)
	log.newAdded = make(chan struct{})
}

func (log *messageLog) Stream(done <-chan struct{}) <-chan messages.Message {
	ch := make(chan messages.Message)
	go log.supplyMessages(ch, done)
	return ch
}

func (log *messageLog) supplyMessages(ch chan<- messages.Message, done <-chan struct{}) {
	defer close(ch)

	log.RLock()
	resetChan := log.resetChan
	log.RUnlock()

	var next int
loop:
	for {
		log.RLock()
		select {
		case <-resetChan:
			resetChan = log.resetChan
			next = 0
		default:
		}
		newAdded := log.newAdded
		msgs := log.msgs[next:]
		next = len(log.msgs)
		log.RUnlock()

		for _, msg := range msgs {
			select {
			case ch <- msg:
			case <-resetChan:
				continue loop
			case <-done:
				return
			}
		}

		select {
		case <-newAdded:
		case <-resetChan:
		case <-done:
			return
		}
	}
}

func (log *messageLog) Messages() []messages.Message {
	log.RLock()
	defer log.RUnlock()

	return log.msgs
}

func (log *messageLog) Reset(msgs []messages.Message) {
	log.Lock()
	defer log.Unlock()

	log.msgs = msgs
	close(log.resetChan)
	log.resetChan = make(chan struct{})
}
