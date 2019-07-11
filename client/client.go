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

// Package client implements client part of the consensus protocol.
package client

import (
	"fmt"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client/internal/requestbuffer"
	"github.com/hyperledger-labs/minbft/messages"

	logging "github.com/op/go-logging"
)

const (
	module = "client"
)

var logger = logging.MustGetLogger(module)

// Stack combines the interfaces of the external modules
type Stack interface {
	api.Authenticator
	api.ReplicaConnector
}

// Client represents an instance of client part of the consensus
// protocol.
//
// Request requests execution of the supplied operation on the
// replicated state machine and returns a channel to receive the
// result of execution from.
type Client interface {
	Request(operation []byte) (resultChan <-chan []byte)
}

// ReplyCollector processes replies and extracts results
//
// Collect gathers Reply messages received from
// the passed channel, finishes the request processing, and sends the
// result of request execution to the passed channel.
type ReplyCollector interface {
	Collect(in <-chan *messages.Reply, out chan<- []byte, remover requestRemover)
}

// New creates an instance of Client given a client ID, total number
// of replica nodes n, number of tolerated faulty replica nodes f, and
// a stack of external interfaces.
func New(id uint32, n, f uint32, stack Stack) (Client, error) {
	if n < f*2+1 {
		return nil, fmt.Errorf("Insufficient number of replica nodes")
	}

	buf := requestbuffer.New()

	if err := startReplicaConnections(id, n, buf, stack); err != nil {
		return nil, fmt.Errorf("Failed to initiate connections to replicas: %s", err)
	}

	seq := makeSequenceGenerator()
	collector := makeReplyCollector(f)
	return makeRequestHandler(id, seq, stack, collector, buf, f), nil
}

// New creates an instance of Client given a client ID, total number
// of replica nodes n, number of tolerated faulty replica nodes f, and
// a stack of external interfaces.
func GetClient(id uint32, n, f uint32, stack Stack, collector ReplyCollector) (Client, error) {
	if n < f*2+1 {
		return nil, fmt.Errorf("Insufficient number of replica nodes")
	}

	buf := requestbuffer.New()

	if err := startReplicaConnections(id, n, buf, stack); err != nil {
		return nil, fmt.Errorf("Failed to initiate connections to replicas: %s", err)
	}

	seq := makeSequenceGenerator()
	return makeRequestHandler(id, seq, stack, collector, buf, f), nil
}

// GetMinBFTCollector constructs a replyCollector compatible with MinBFT
// using the supplied tolerance to remove the request from when its
// processing is finished.
func GetMinBFTCollector(f uint32) ReplyCollector {

	return makeReplyCollector(f)
}

// Request implements Client interface on requestHandler
func (handler requestHandler) Request(operation []byte) <-chan []byte {
	return handler(operation)
}
