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
	commonLogger "github.com/hyperledger-labs/minbft/common/logger"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

var messageImpl = protobufMessages.NewImpl()

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
//
// Terminate causes the instance to stop all activity so that the
// garbage collector can reclaim any resources allocated for it.
type Client interface {
	Request(operation []byte) (resultChan <-chan []byte)
	Terminate()
}

type options struct {
	logger commonLogger.Logger
}

//Option is a function to set client optional parameters
type Option func(*options)

func makeDefaultOptions(id uint32, opts ...Option) options {
	opt := options{}
	for _, o := range opts {
		o(&opt)
	}
	if opt.logger == nil {
		l := commonLogger.NewClientLogger(id)
		opt.logger = l
	}
	return opt
}

//WithLogger sets an externally created logger
func WithLogger(logger commonLogger.Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

type client struct {
	handleRequest requestHandler
	stopChan      chan struct{}
}

// New creates an instance of Client given a client ID, total number
// of replica nodes n, number of tolerated faulty replica nodes f, and
// a stack of external interfaces.
func New(id uint32, n, f uint32, stack Stack, opts ...Option) (Client, error) {
	if n < f*2+1 {
		return nil, fmt.Errorf("insufficient number of replica nodes")
	}

	stopChan := make(chan struct{})
	buf := requestbuffer.New()
	logger := makeDefaultOptions(id, opts...).logger

	if err := startReplicaConnections(id, n, buf, stack, stopChan, logger); err != nil {
		return nil, fmt.Errorf("failed to initiate connections to replicas: %s", err)
	}

	seq := makeSequenceGenerator()
	handleRequest := makeRequestHandler(id, seq, stack, buf, f, stopChan, logger)

	return &client{handleRequest, stopChan}, nil
}

func (c *client) Request(operation []byte) <-chan []byte {
	return c.handleRequest(operation)
}

func (c *client) Terminate() {
	close(c.stopChan)
}
