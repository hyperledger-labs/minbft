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

package messagelog

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

func TestAppend(t *testing.T) {
	log := New()

	done := make(chan struct{})
	defer close(done)

	log.Append(makeMsg())

	// Should not block if there is no stream
	log.Append(makeMsg())

	_ = log.Stream(done)

	// Should not block if there is a stream
	log.Append(makeMsg())
	log.Append(makeMsg())

	_ = log.Stream(done)

	// Should not block if there are multiple streams
	log.Append(makeMsg())
	log.Append(makeMsg())
}

func TestStream(t *testing.T) {
	const nrMessages = 5

	log := New()
	msgs := makeManyMsgs(nrMessages)

	done := make(chan struct{})
	ch1 := log.Stream(done)
	ch2 := log.Stream(done)

	for _, msg := range msgs {
		log.Append(msg)
	}

	receiveMessages := func(ch <-chan *messages.Message) {
		for i, msg := range msgs {
			assertMsgEqualf(t, msg, <-ch, "Unexpected message %d", i)
		}
	}

	receiveMessages(ch1)
	receiveMessages(ch2)

	close(done)
	_, more := <-ch1
	assert.False(t, more, "Channel not closed")
	_, more = <-ch2
	assert.False(t, more, "Channel not closed")
}

func TestConcurrent(t *testing.T) {
	const nrStreams = 3
	const nrMessages = 5

	log := New()
	msgs := makeManyMsgs(nrMessages)

	wg := new(sync.WaitGroup)
	wg.Add(nrStreams)
	for id := 0; id < nrStreams; id++ {
		go func(streamID int) {
			defer wg.Done()

			done := make(chan struct{})
			ch := log.Stream(done)

			for i, msg := range msgs {
				assertMsgEqualf(t, msg, <-ch,
					"Unexpected message %d from stream %d", i, streamID)
			}

			close(done)
			_, more := <-ch
			assert.Falsef(t, more, "Stream %d channel not closed", streamID)
		}(id)
	}

	for _, msg := range msgs {
		log.Append(msg)
	}

	wg.Wait()
}

func TestWithFaulty(t *testing.T) {
	const nrStreams = 5
	const nrFaulty = 2
	const nrMessages = 10

	log := New()
	msgs := makeManyMsgs(nrMessages)

	wg := new(sync.WaitGroup)
	wg.Add(nrStreams)
	for id := 0; id < nrStreams; id++ {
		go func(streamID int) {
			done := make(chan struct{})
			defer close(done)
			ch := log.Stream(done)

			if streamID < nrFaulty {
				wg.Done()
				wg.Wait()
				return
			}

			for i, msg := range msgs {
				assertMsgEqualf(t, msg, <-ch,
					"Unexpected message %d from stream %d", i, streamID)
			}

			wg.Done()
		}(id)
	}

	for _, msg := range msgs {
		log.Append(msg)
	}

	wg.Wait()
}

func makeManyMsgs(nrMessages int) []*messages.Message {
	msgs := make([]*messages.Message, nrMessages)
	for i := 0; i < nrMessages; i++ {
		msgs[i] = makeMsg()
	}
	return msgs
}

func makeMsg() *messages.Message {
	return &messages.Message{}
}

func assertMsgEqualf(t assert.TestingT, expected, actual *messages.Message, msg string, args ...interface{}) bool {
	expectedPtr := reflect.ValueOf(expected).Pointer()
	actualPtr := reflect.ValueOf(actual).Pointer()
	return assert.Equalf(t, expectedPtr, actualPtr, msg, args...)
}
