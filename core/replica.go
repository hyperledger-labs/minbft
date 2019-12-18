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

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

const (
	module            = "minbft"
	defaultLogPrefix  = `%{color}[%{module}] %{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset}`
	logMaxStringWidth = 256
)

var messageImpl = protobufMessages.NewImpl()

// Stack combines interfaces of external modules
type Stack interface {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer
}

// Replica represents an instance of replica peer
type replica struct {
	handleStream messageStreamHandler
}

// New creates a new instance of replica node
func New(id uint32, configer api.Configer, stack Stack, opts ...Option) (api.Replica, error) {
	n := configer.N()
	f := configer.F()

	if n < 2*f+1 {
		return nil, fmt.Errorf("%d nodes is not enough to tolerate %d faulty", n, f)
	}

	logOpts := newOptions(opts...)

	messageLog := messagelog.New()
	logger := makeLogger(id, logOpts)

	if err := startPeerConnections(id, n, stack, messageLog, logger); err != nil {
		return nil, fmt.Errorf("Failed to start peer connections: %s", err)
	}

	handle := defaultIncomingMessageHandler(id, messageLog, configer, stack, logger)
	handleStream := makeMessageStreamHandler(handle, logger)

	return &replica{handleStream}, nil
}

func (r *replica) PeerMessageStreamHandler() api.MessageStreamHandler {
	// TODO: Handle peer/client connections differently
	return r.handleStream
}

func (r *replica) ClientMessageStreamHandler() api.MessageStreamHandler {
	// TODO: Handle peer/client connections differently
	return r.handleStream
}

func (handle messageStreamHandler) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go handle(in, out)

	return out
}
