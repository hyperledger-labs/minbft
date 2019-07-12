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
)

const (
	module           = "minbft"
	defaultLogPrefix = `%{color}[%{module}] %{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset}`
)

// Stack combines interfaces of external modules
type Stack interface {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer
}

// Replica represents an instance of replica peer
type replica struct {
	id uint32 // replica ID, unique in range [0,n)
	n  uint32 // total number of nodes

	stack Stack

	log          messagelog.MessageLog
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

	replica := &replica{
		id: id,
		n:  n,

		stack: stack,

		log: messagelog.New(),
	}

	logger := makeLogger(id, logOpts)
	handle := defaultIncomingMessageHandler(id, replica.log, configer, stack, logger)
	replica.handleStream = makeMessageStreamHandler(handle, logger)

	if err := replica.start(); err != nil {
		return nil, fmt.Errorf("Failed to start replica: %s", err)
	}

	return replica, nil
}

// Start begins message exchange with peer replicas
func (r *replica) start() error {
	supply := makePeerMessageSupplier(r.log)

	for i := uint32(0); i < r.n; i++ {
		if i == r.id {
			continue
		}

		conn := makePeerConnector(i, r.stack)
		out := make(chan []byte)

		// Reply stream is not used for replica-to-replica
		// communication, thus return value is ignored. Each
		// replica will establish connections to other peers
		// the same way, so they all will be eventually fully
		// connected.
		if _, err := conn(out); err != nil {
			return fmt.Errorf("Cannot connect to replica %d: %s", i, err)
		}

		go supply(out)
	}
	return nil
}

// HandleMessageStream initiates handling of incoming messages and
// supplies reply messages back, if any.
func (r *replica) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go r.handleStream(in, out)

	return out
}
