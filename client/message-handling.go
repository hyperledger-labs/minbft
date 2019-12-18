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
	"fmt"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client/internal/requestbuffer"
	"github.com/hyperledger-labs/minbft/messages"
)

// startReplicaConnections initiates connections to all replicas and
// starts message exchange with them given a total number of replicas,
// request buffer to add/fetch messages to/from and a stack of
// interfaces to external modules.
func startReplicaConnections(clientID, n uint32, buf *requestbuffer.T, stack Stack) error {
	outHandler := makeOutgoingMessageHandler(buf)
	authenticator := makeReplyAuthenticator(clientID, stack)
	consumer := makeReplyConsumer(buf)
	handleReply := makeReplyMessageHandler(consumer, authenticator)

	for i := uint32(0); i < n; i++ {
		connector := makeReplicaConnector(i, stack)
		inHandler := makeIncomingMessageHandler(i, handleReply)
		if err := startReplicaConnection(outHandler, inHandler, connector); err != nil {
			return fmt.Errorf("Error connecting to replica %d: %s", i, err)
		}
	}

	return nil
}

// outgoingMessageHandler supplies the channel passed to it with a
// stream of messages to be delivered to a replica.
type outgoingMessageHandler func(out chan<- []byte)

// incomingMessageHandler fetches messages received from a replica
// from the channel passed to it and performs further processing of
// the messages.
type incomingMessageHandler func(in <-chan []byte)

// replicaConnector initiates message exchange with a replica. It
// receives a channel of outgoing messages to supply to the replica
// and returns a channel of messages produced by the replica in reply.
type replicaConnector func(out <-chan []byte) (in <-chan []byte, err error)

func startReplicaConnection(outHandler outgoingMessageHandler, inHandler incomingMessageHandler, connector replicaConnector) error {
	out := make(chan []byte)
	in, err := connector(out)
	if err != nil {
		return err
	}

	go outHandler(out)
	go inHandler(in)

	return nil
}

// makeOutgoingMessageHandler construct an outgoingMessageHandler
// using the supplied request buffer as a source of outgoing messages.
func makeOutgoingMessageHandler(buf *requestbuffer.T) outgoingMessageHandler {
	return func(out chan<- []byte) {
		for req := range buf.RequestStream(nil) {
			mBytes, err := req.MarshalBinary()
			if err != nil {
				panic(err)
			}
			out <- mBytes
		}
	}
}

// makeIncomingMessageHandler constructs an incomingMessageHandler
// using replicaID as the ID of replica which supplies incoming
// messages, and the passed abstraction to handle Reply messages.
func makeIncomingMessageHandler(replicaID uint32, handleReply replyMessageHandler) incomingMessageHandler {
	return func(in <-chan []byte) {
		for msgBytes := range in {
			msg, err := messageImpl.NewFromBinary(msgBytes)
			if err != nil {
				logger.Warningf("Error unmarshaling message from replica %d: %v", replicaID, err)
				continue
			}

			switch msg := msg.(type) {
			case messages.Reply:
				handleReply(msg)
			default:
				logger.Warningf("Received unknown message from replica %d", replicaID)
			}
		}
	}
}

// makeReplicaConnector constructs a replicaConnector for the
// specified replica using the supplied general replica connector.
func makeReplicaConnector(replicaID uint32, connector api.ReplicaConnector) replicaConnector {
	return func(out <-chan []byte) (<-chan []byte, error) {
		streamHandler := connector.ReplicaMessageStreamHandler(replicaID)
		if streamHandler == nil {
			return nil, fmt.Errorf("Connection not possible")
		}
		in := streamHandler.HandleMessageStream(out)
		return in, nil
	}
}

// replyMessageHandler performs processing of the Reply message
// received from a replica
type replyMessageHandler func(reply messages.Reply)

// replyAuthenticator verifies a Reply message for integrity and
// authenticity
type replyAuthenticator func(reply messages.Reply) error

// replyConsumer performs further processing of a valid Reply
// messages, verified for integrity and authenticity. The returned
// value indicates if the message was accepted.
type replyConsumer func(reply messages.Reply) bool

// makeReplyMessageHandler construct a replyMessageHandler using the
// supplied abstractions.
func makeReplyMessageHandler(consumer replyConsumer, authenticator replyAuthenticator) replyMessageHandler {
	return func(reply messages.Reply) {
		replicaID := reply.ReplicaID()

		err := authenticator(reply)
		if err != nil {
			logger.Warningf("Failed to authenticate Reply message from replica %d: %v",
				replicaID, err)
			return
		}

		logger.Debugf("Received Reply message from replica %d", replicaID)

		if ok := consumer(reply); !ok {
			logger.Infof("Dropped Reply message from replica %d", replicaID)
		}
	}
}

// makeReplyAuthenticator constructs a replyAuthenticator using the
// supplied authenticator to perform replica signature verification.
func makeReplyAuthenticator(clientID uint32, authenticator api.Authenticator) replyAuthenticator {
	return func(reply messages.Reply) error {
		if reply.ClientID() != clientID {
			return fmt.Errorf("Client ID mismatch")
		}

		return authenticator.VerifyMessageAuthenTag(api.ReplicaAuthen, reply.ReplicaID(),
			reply.SignedPayload(), reply.Signature())
	}
}

// makeReplyConsumer constructs a replyConsumer using the supplied
// request buffer to add the message to
func makeReplyConsumer(buf *requestbuffer.T) replyConsumer {
	return func(reply messages.Reply) bool {
		return buf.AddReply(reply)
	}
}
