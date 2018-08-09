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

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client/internal/requestbuffer"
	"github.com/hyperledger-labs/minbft/messages"
)

// startReplicaConnections initiates connections to all replicas and
// starts message exchange with them given a total number of replicas,
// request buffer to add/fetch messages to/from and a stack of
// interfaces to external modules.
func startReplicaConnections(n uint32, buf *requestbuffer.T, stack Stack) error {
	outHandler := makeOutgoingMessageHandler(buf)
	for i := uint32(0); i < n; i++ {
		connector := makeReplicaConnector(i, stack)
		inHandler := makeIncomingMessageHandler(i, buf, stack)
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
		handleOutgoingMessages(buf.RequestStream(nil), out)
	}
}

// makeIncomingMessageHandler constructs an incomingMessageHandler
// using the supplied request buffer as the destination and the
// supplied authenticator for message authentication.
func makeIncomingMessageHandler(replicaID uint32, buf *requestbuffer.T, authen api.Authenticator) incomingMessageHandler {
	replyHandler := makeReplyMessageHandler(buf, authen)
	return func(in <-chan []byte) {
		handleIncomingMessages(replicaID, in, replyHandler)
	}
}

// makeReplicaConnector constructs a replicaConnector for the
// specified replica using the supplied general replica connector.
func makeReplicaConnector(replicaID uint32, connector api.ReplicaConnector) replicaConnector {
	return func(out <-chan []byte) (<-chan []byte, error) {
		streamHandler, err := connector.ReplicaMessageStreamHandler(replicaID)
		if err != nil {
			return nil, fmt.Errorf("Error getting message stream handler: %s", err)
		}
		in, err := streamHandler.HandleMessageStream(out)
		if err != nil {
			return nil, fmt.Errorf("Error establishing connection: %s", err)
		}
		return in, nil
	}
}

// handleOutgoingMessages receives Request messages from in channel,
// serializes them, and sends them to out channel.
func handleOutgoingMessages(in <-chan *messages.Request, out chan<- []byte) {
	for req := range in {
		msg := messages.WrapMessage(req)
		mBytes, err := proto.Marshal(msg)
		if err != nil {
			panic(err)
		}
		out <- mBytes
	}
}

// replyMessageHandler performs processing of the Reply message
// received from a replica
type replyMessageHandler func(reply *messages.Reply)

// handleIncomingMessages receives incoming messages from the supplied
// channel, deserializes then and handles Reply messages using the
// supplied reply handler.
func handleIncomingMessages(replicaID uint32, in <-chan []byte, replyHandler replyMessageHandler) {
	for msgBytes := range in {
		msg := &messages.Message{}
		if err := proto.Unmarshal(msgBytes, msg); err != nil {
			logger.Warningf("Error unmarshaling message from replica %d: %v", replicaID, err)
			continue
		}

		switch t := msg.Type.(type) {
		case *messages.Message_Reply:
			replyHandler(t.Reply)
		default:
			logger.Warningf("Received unknown message from replica %d", replicaID)
		}
	}
}

// replyAuthenticator verifies a Reply message for integrity and
// authenticity
type replyAuthenticator func(reply *messages.Reply) error

// replyConsumer performs further processing of a valid Reply
// messages, verified for integrity and authenticity. The returned
// value indicates if the message was accepted.
type replyConsumer func(reply *messages.Reply) bool

// makeReplyMessageHandler construct a replyMessageHandler using the
// supplied request buffer as the destination and the supplied
// authenticator for message authentication.
func makeReplyMessageHandler(buf *requestbuffer.T, authen api.Authenticator) replyMessageHandler {
	authenticator := makeReplyAuthenticator(authen)
	consumer := makeReplyConsumer(buf)
	return func(reply *messages.Reply) {
		handleReplyMessage(reply, consumer, authenticator)
	}
}

// handleReplyMessage performs processing of the Reply message
// received from a replica using the supplied Reply authenticator and
// consumer.
func handleReplyMessage(reply *messages.Reply, consumer replyConsumer, authenticator replyAuthenticator) {
	replyMsg := reply.Msg

	err := authenticator(reply)
	if err != nil {
		logger.Warningf("Failed to authenticate Reply message from replica %d: %v",
			replyMsg.ReplicaId, err)
		return
	}

	logger.Debugf("Received Reply message from replica %d", replyMsg.ReplicaId)

	if ok := consumer(reply); !ok {
		logger.Infof("Dropped Reply message from replica %d", replyMsg.ReplicaId)
	}
}

// makeReplyAuthenticator constructs a replyAuthenticator using the
// supplied authenticator to perform replica signature verification.
func makeReplyAuthenticator(authenticator api.Authenticator) replyAuthenticator {
	return func(reply *messages.Reply) error {
		replyMsg := reply.Msg

		msgBytes, err := proto.Marshal(replyMsg)
		if err != nil {
			panic(err)
		}

		return authenticator.VerifyMessageAuthenTag(api.ReplicaAuthen, replyMsg.ReplicaId,
			msgBytes, reply.Signature)
	}
}

// makeReplyConsumer constructs a replyConsumer using the supplied
// request buffer to add the message to
func makeReplyConsumer(buf *requestbuffer.T) replyConsumer {
	return func(reply *messages.Reply) bool {
		return buf.AddReply(reply)
	}
}
