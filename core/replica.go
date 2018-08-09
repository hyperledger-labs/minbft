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
	"os"

	"github.com/golang/protobuf/proto"
	logging "github.com/op/go-logging"

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"
)

const (
	module          = "minbft"
	logFormatString = `%{color}[%{module}] %{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`
)

var logger *logging.Logger

func init() {
	logger = logging.MustGetLogger(module)
	stringFormatter := logging.MustStringFormatter(logFormatString)
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, stringFormatter)
	formattedLoggerBackend := logging.AddModuleLevel(backendFormatter)

	logging.SetBackend(formattedLoggerBackend)

	formattedLoggerBackend.SetLevel(logging.DEBUG, module)
}

// Stack combines interfaces of external modules
type Stack interface {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer
}

var _ api.MessageStreamHandler = (*Replica)(nil)

// Replica represents an instance of replica peer
type Replica struct {
	id uint32 // replica ID, unique in range [0,n)
	n  uint32 // total number of nodes

	stack Stack

	log          messagelog.MessageLog
	handleStream messageStreamHandler
}

// New creates a new instance of replica node
func New(id uint32, configer api.Configer, stack Stack) (*Replica, error) {
	n := configer.N()
	f := configer.F()

	if n < 2*f+1 {
		return nil, fmt.Errorf("%d nodes is not enough to tolerate %d faulty", n, f)
	}

	replica := &Replica{
		id: id,
		n:  n,

		stack: stack,

		log: messagelog.New(),
	}

	handle := defaultMessageHandler(id, replica.log, configer, stack)
	replica.handleStream = makeMessageStreamHandler(handle)

	return replica, nil
}

// Start begins message exchange with peer replicas
func (r *Replica) Start() error {
	for i := uint32(0); i < r.n; i++ {
		if i == r.id {
			continue
		}
		out := make(chan []byte)
		sh, err := r.stack.ReplicaMessageStreamHandler(i)
		if err != nil {
			return fmt.Errorf("Error getting peer replica %d message stream handler: %s", i, err)
		}
		// Reply stream is not used for replica-to-replica
		// communication, thus return value is ignored. Each
		// replica will establish connections to other peers
		// the same way, so they all will be eventually fully
		// connected.
		_, err = sh.HandleMessageStream(out)
		if err != nil {
			return fmt.Errorf("Error establishing connection to peer replica %d: %s", i, err)
		}

		go func() {
			for msg := range r.log.Stream(nil) {
				msgBytes, err := proto.Marshal(msg)
				if err != nil {
					panic(err)
				}
				out <- msgBytes
			}
		}()
	}
	return nil
}

// HandleMessageStream initiates handling of incoming messages and
// supplies reply messages back, if any.
func (r *Replica) HandleMessageStream(in <-chan []byte) (<-chan []byte, error) {
	out := make(chan []byte)

	go r.handleStream(in, out)

	return out, nil
}
