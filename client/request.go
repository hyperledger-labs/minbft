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

package client

import (
	"crypto/sha256"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client/internal/requestbuffer"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

// requestHandler initiates the specified operation to execute on the
// replicated state machine and returns a channel to receive the
// result from.
type requestHandler func(operation []byte) <-chan []byte

// makeRequestHandler constructs a requestHandler uisng the supplied
// clientID, sequence number generator, authenticator, request buffer,
// and number of tolerated faulty replicas.
func makeRequestHandler(clientID uint32, seq sequenceGenerator, authen api.Authenticator, buf *requestbuffer.T, f uint32) requestHandler {
	submitter := makeRequestSubmitter(clientID, seq, authen, buf)
	collector := makeReplyCollector(f, buf)
	return func(operation []byte) <-chan []byte {
		return handleRequest(operation, submitter, collector)
	}
}

// requestSubmitter initiates processing of a request to execute an
// operation on the replicated state machine and returns a channel to
// fetch corresponding Reply messages from.
type requestSubmitter func(operation []byte) <-chan *messages.Reply

// replyCollector collects f+1 matching Reply messages received from
// the passed channel, finishes the request processing, and sends the
// result of request execution to the passed channel.
type replyCollector func(in <-chan *messages.Reply, out chan<- []byte)

// handleRequest initiates the specified operation to execute on the
// replicated state machine using the passed request submitter and
// returns a channel to receive the result of execution from.
func handleRequest(operation []byte, submitter requestSubmitter, collector replyCollector) <-chan []byte {
	resultChan := make(chan []byte, 1)
	replyChan := submitter(operation)
	go collector(replyChan, resultChan)
	return resultChan
}

// makeReplyCollector constructs a replyCollector using the supplied
// tolerance and request buffer to remove the request from when its
// processing is finished.
func makeReplyCollector(f uint32, buf *requestbuffer.T) replyCollector {
	remover := makeRequestRemover(buf)
	return func(in <-chan *messages.Reply, out chan<- []byte) {
		collectReplies(f, in, remover, out)
	}
}

// requestRemover removes and stops further processing of the request
// given its sequence number.
type requestRemover func(seq uint64)

// collectReplies collects f+1 matching Reply messages fetched from
// the supplied channel, removes the corresponding request using the
// supplied request remover, and sends the result of request execution
// to the supplied channel.
func collectReplies(f uint32, replyChan <-chan *messages.Reply, remover requestRemover, resultChan chan<- []byte) {
	type resultHashType [sha256.Size]byte
	matchingResults := make(map[resultHashType]uint32)

	for reply := range replyChan {
		replyMsg := reply.Msg
		result := replyMsg.Result
		hash := sha256.Sum256(result)
		matchingResults[hash]++
		if matchingResults[hash] > f {
			remover(replyMsg.Seq)
			resultChan <- result
			break
		}
	}
}

// makeRequestRemover constructs a requestRemover using the supplied
// request buffer to remove the Request message from.
func makeRequestRemover(buf *requestbuffer.T) requestRemover {
	return func(seq uint64) {
		buf.RemoveRequest(seq)
	}
}

// sequenceGenerator returns a new monotonically increasing sequence
// number on each invocation.
type sequenceGenerator func() uint64

// makeRequestSubmitter constructs a requestSubmitter using the
// supplied client ID, sequence number generator, authenticator and
// request buffer to add the produced Request message to.
func makeRequestSubmitter(clientID uint32, seq sequenceGenerator, authen api.Authenticator, buf *requestbuffer.T) requestSubmitter {
	preparer := makeRequestPreparer(clientID, authen, seq)
	consumer := makeRequestConsumer(buf)
	return func(operation []byte) <-chan *messages.Reply {
		return submitRequest(operation, preparer, consumer)
	}
}

// requestPreparer prepares a new signed Request message given an
// operation to execute by the replicated state machine
type requestPreparer func(operation []byte) *messages.Request

// requestConsumer consumes a signed Request message for further
// processing and returns a channel to fetch corresponding Reply
// messages from. The returned boolean value indicates if the Request
// message was accepted.
type requestConsumer func(request *messages.Request) (<-chan *messages.Reply, bool)

// submitRequest makes a new Request message for a given operation to
// execute by the replicated state machine using the supplied
// requestPreparer and passes it to the supplied requestConsumer. It
// returns a channel to fetch corresponding messages from.
func submitRequest(operation []byte, preparer requestPreparer, consumer requestConsumer) <-chan *messages.Reply {
	request := preparer(operation)
	replyChan, ok := consumer(request)
	if !ok {
		panic("Request message rejected")
	}
	return replyChan
}

// makeRequestPreparer constructs a requestPreparer using the supplied
// client ID, authenticator, and sequence number generator.
func makeRequestPreparer(clientID uint32, authenticator api.Authenticator, seq sequenceGenerator) requestPreparer {
	constructor := makeRequestConstructor(clientID, seq)
	signer := makeRequestSigner(authenticator)

	return func(operation []byte) *messages.Request {
		return prepareRequest(operation, constructor, signer)
	}
}

// makeRequestConsumer constructs a requestConsumer using the supplied
// request buffer to add the Request message to and retrieve
// corresponding Reply messages from
func makeRequestConsumer(buf *requestbuffer.T) requestConsumer {
	return func(request *messages.Request) (<-chan *messages.Reply, bool) {
		return buf.AddRequest(request)
	}
}

// requestConstructor constructs a new Request message ready to sign,
// given an operation to execute by the replicated state machine.
type requestConstructor func(operation []byte) *messages.Request

// requestSigner signs the supplied Request message
type requestSigner func(request *messages.Request) error

// prepareRequest prepares a new singed Request message given an
// operation to execute by the replicated state machine, request
// constructor and signer.
func prepareRequest(operation []byte, constructor requestConstructor, signer requestSigner) *messages.Request {
	request := constructor(operation)
	if err := signer(request); err != nil {
		logger.Fatalf("Failed to sign request message: %s", err)
	}

	return request
}

// makeRequestConstructor constructs a requestConstructor using the
// supplied client ID and sequenceGenerator
func makeRequestConstructor(clientID uint32, seq sequenceGenerator) requestConstructor {
	return func(operation []byte) *messages.Request {
		return &messages.Request{
			Msg: &messages.Request_M{
				ClientId: clientID,
				Seq:      seq(),
				Payload:  operation,
			},
		}
	}
}

// makeRequestSigner constructs a requestSigner using the supplied
// authenticator to generate message authentication tags.
func makeRequestSigner(authenticator api.Authenticator) requestSigner {
	return func(request *messages.Request) error {
		reqBytes, err := proto.Marshal(request.Msg)
		if err != nil {
			panic(err)
		}
		sig, err := authenticator.GenerateMessageAuthenTag(api.ClientAuthen, reqBytes)
		if err != nil {
			return err
		}
		request.Signature = sig
		return nil
	}
}

// makeSequenceGenerator constructs a sequenceGenerator to provide
// consecutive sequence numbers starting from current Unix time in
// nanoseconds.g
func makeSequenceGenerator() sequenceGenerator {
	nextSeq := uint64(time.Now().UTC().UnixNano())

	return func() uint64 {
		seq := nextSeq
		nextSeq++
		return seq
	}
}
