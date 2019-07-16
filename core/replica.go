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

	api "github.com/hyperledger-labs/minbft/api"
	logging "github.com/op/go-logging"
)

const (
	module           = "minbft"
	defaultLogPrefix = `%{color}[%{module}] %{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset}`
)

// Stack combines interfaces of external modules
type Stack interface {
	api.ReplicaConnector
	api.Authenticator
	api.ProtocolHandler
	api.RequestConsumer
}

var _ api.MessageStreamHandler = (*Replica)(nil)

// Replica represents an instance of replica peer
type Replica struct {
	handleStream messageStreamHandler
	stack        Stack
}

// NewReplica creates a new instance of replica node
func NewReplica(configer api.Configer, stack Stack, logger *logging.Logger) (*Replica, error) {
	n := configer.N()
	f := configer.F()

	if n < 2*f+1 {
		return nil, fmt.Errorf("%d nodes is not enough to tolerate %d faulty", n, f)
	}

	replica := &Replica{
		handleStream: makeMessageStreamHandler(stack, logger),
		stack:        stack,
	}

	if err := replica.stack.Start(); err != nil {
		return nil, fmt.Errorf("Failed to start replica: %s", err)
	}

	return replica, nil
}

// HandleMessageStream initiates handling of incoming messages and
// supplies reply messages back, if any.
func (r *Replica) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go r.handleStream(in, out)

	return out
}
