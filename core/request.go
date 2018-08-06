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
	"sync/atomic"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/core/internal/clientstate"
	"github.com/hyperledger-labs/minbft/messages"
)

// requestHandler fully handles a Request message.
//
// The Request message will be fully verified and processed. The
// return value new indicates if the Request has not been processed by
// this replica before.
type requestHandler func(request *messages.Request) (new bool, err error)

// requestReplier returns a channel that can be used to receive a
// Reply message corresponding to the supplied Request message. It is
// safe to invoke concurrently.
type requestReplier func(request *messages.Request) <-chan *messages.Reply

// requestExecutor given a Request message executes the requested
// operation, produces the corresponding Reply message ready for
// delivery to the client, and hands it over for further processing.
type requestExecutor func(request *messages.Request)

// operationExecutor executes an operation on the local instance of
// the replicated state machine. The result of operation execution
// will be send to the returned channel once it is ready. It is not
// allowed to invoke concurrently.
type operationExecutor func(operation []byte) (resultChan <-chan []byte)

// requestSeqAcceptor accepts request identifier of Request message.
//
// A new request identifier cannot be accepted until a Reply message
// for the greatest accepted identifier from that client has been
// supplied. A request identifier is new if it is greater than the
// greatest accepted. If the request identifier cannot be accepted
// immediately, it will block until the identifier can be accepted.
// The return value new indicates if the request identifier in the
// supplied message was new. It is safe to invoke concurrently.
type requestSeqAcceptor func(request *messages.Request) (new bool)

// replyConsumer performs further processing of the supplied Reply
// message produced locally. The message should be ready to serialize
// and deliver to the client. It is safe to invoke concurrently.
type replyConsumer func(reply *messages.Reply, clientID uint32)

// defaultRequestHandler constructs a standard requestHandler using id
// as the current replica ID, n as the total number of nodes, and the
// supplied abstract interfaces.
func defaultRequestHandler(id, n uint32, view viewProvider, authen api.Authenticator, clientStates clientstate.Provider, handleGeneratedUIMessage generatedUIMessageHandler) requestHandler {
	verifier := makeMessageSignatureVerifier(authen)
	seqAcceptor := makeRequestSeqAcceptor(clientStates)

	return makeRequestHandler(id, n, view, verifier, seqAcceptor, handleGeneratedUIMessage)
}

// defaultRequestExecutor constructs a standard requestExecutor using
// id as the current replica ID, and the supplied abstract interfaces.
func defaultRequestExecutor(id uint32, clientStates clientstate.Provider, stack Stack) requestExecutor {
	executeOperation := makeOperationExecutor(stack)
	signMessage := makeReplicaMessageSigner(stack)
	consumeReply := makeReplyConsumer(clientStates)
	return makeRequestExecutor(id, executeOperation, signMessage, consumeReply)
}

// makeRequestHandler constructs an instance of requestHandler using
// id as the current replica ID, n as the total number of nodes, and
// the supplied abstract interfaces.
func makeRequestHandler(id, n uint32, view viewProvider, verifier messageSignatureVerifier, seqAcceptor requestSeqAcceptor, handleGeneratedUIMessage generatedUIMessageHandler) requestHandler {
	return func(request *messages.Request) (new bool, err error) {
		logger.Debugf("Replica %d handling Request from client %d: seq=%d op=%s",
			id, request.Msg.ClientId, request.Msg.Seq, request.Msg.Payload)

		if err = verifier(request); err != nil {
			err = fmt.Errorf("Failed to authenticate Request message: %s", err)
			return false, err
		}

		if new = seqAcceptor(request); !new {
			return false, nil
		}

		view := view()
		primary := isPrimary(view, id, n)

		// TODO: A new request ID has arrived; the request
		// timer should be re-/started in backup replicas at
		// this point.

		if primary {
			prepare := &messages.Prepare{
				Msg: &messages.Prepare_M{
					View:      view,
					ReplicaId: id,
					Request:   request,
				},
			}
			logger.Debugf("Replica %d generated Prepare: view=%d client=%d seq=%d",
				prepare.Msg.ReplicaId, prepare.Msg.View,
				prepare.Msg.Request.Msg.ClientId, prepare.Msg.Request.Msg.Seq)
			handleGeneratedUIMessage(prepare)
		}

		return true, nil
	}
}

// makeRequestReplier constructs an instance of requestReplier using
// the supplied client state provider.
func makeRequestReplier(provider clientstate.Provider) requestReplier {
	return func(request *messages.Request) <-chan *messages.Reply {
		state := provider(request.Msg.ClientId)
		return state.ReplyChannel(request.Msg.Seq)
	}
}

// makeRequestExecutor constructs an instance of requestExecutor using
// the supplied replica ID, operation executor, message signer, and
// reply consumer.
func makeRequestExecutor(replicaID uint32, executor operationExecutor, signer replicaMessageSigner, consumer replyConsumer) requestExecutor {
	return func(request *messages.Request) {
		resultChan := executor(request.Msg.Payload)
		go func() {
			result := <-resultChan

			reply := &messages.Reply{
				Msg: &messages.Reply_M{
					ReplicaId: replicaID,
					Seq:       request.Msg.Seq,
					Result:    result,
				},
			}
			signer(reply)
			logger.Debugf("Replica %d generated Reply for client %d: seq=%d result=%s",
				replicaID, request.Msg.ClientId, reply.Msg.Seq, reply.Msg.Result)
			consumer(reply, request.Msg.ClientId)
		}()
	}
}

// makeOperationExecutor constructs an instance of operationExecutor
// using the supplied interface to external request consumer module.
func makeOperationExecutor(consumer api.RequestConsumer) operationExecutor {
	busy := uint32(0) // atomic flag to check for concurrent execution

	return func(op []byte) <-chan []byte {
		if wasBusy := atomic.SwapUint32(&busy, uint32(1)); wasBusy != uint32(0) {
			panic("Concurrent operation execution detected")
		}
		resultChan := consumer.Deliver(op)
		atomic.StoreUint32(&busy, uint32(0))

		return resultChan
	}
}

// makeRequestSeqAcceptor constructs an instance of requestSeqAcceptor
// using the supplied client state provider.
func makeRequestSeqAcceptor(provideClientState clientstate.Provider) requestSeqAcceptor {
	return func(request *messages.Request) (new bool) {
		clientID := request.Msg.ClientId
		seq := request.Msg.Seq

		return provideClientState(clientID).AcceptRequestSeq(seq)
	}
}

// makeReplyConsumer constructs an instance of replyConsumer using the
// supplied client state provider.
func makeReplyConsumer(provider clientstate.Provider) replyConsumer {
	return func(reply *messages.Reply, clientID uint32) {
		if err := provider(clientID).AddReply(reply); err != nil {
			panic(err) // Erroneous Reply must never be supplied
		}
	}
}
