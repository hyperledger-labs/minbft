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
	"sync"

	"github.com/hyperledger-labs/minbft/api"
	commonLogger "github.com/hyperledger-labs/minbft/common/logger"
	"github.com/hyperledger-labs/minbft/core/internal/messagelog"

	protobufMessages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

var messageImpl = protobufMessages.NewImpl()

// Stack combines interfaces of external modules
type Stack interface {
	api.ReplicaConnector
	api.Authenticator
	api.RequestConsumer
}

type Replica interface {
	api.Replica
	Terminate()
}

// Replica represents an instance of replica peer
type replica struct {
	handlePeerStream   messageStreamHandler
	handleClientStream messageStreamHandler
	stopChan           chan struct{}
	wg                 *sync.WaitGroup
}

type options struct {
	logger commonLogger.Logger
}

//Option is a function to set replica optional parameters
type Option func(*options)

func makeDefaultOptions(id uint32, opts ...Option) options {
	opt := options{}
	for _, o := range opts {
		o(&opt)
	}
	if opt.logger == nil {
		l := commonLogger.NewReplicaLogger(id)
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

// New creates a new instance of replica node
func New(id uint32, configer api.Configer, stack Stack, opts ...Option) (Replica, error) {
	n := configer.N()
	f := configer.F()

	if n < 2*f+1 {
		return nil, fmt.Errorf("%d nodes is not enough to tolerate %d faulty", n, f)
	}

	logger := makeDefaultOptions(id, opts...).logger

	messageLog := messagelog.New()
	unicastLogs := make(map[uint32]messagelog.MessageLog)
	for peerID := uint32(0); peerID < n; peerID++ {
		if peerID == id {
			continue
		}

		// TODO: we need to handle the situation when the
		// messagelog accumulates messages indefinitely if the
		// destination replica stops receiving them.
		unicastLogs[peerID] = messagelog.New()
	}

	stop := make(chan struct{})
	handleOwnMessage, handlePeerMessage, handleClientMessage := defaultMessageHandlers(id, messageLog, unicastLogs, stop, configer, stack, logger)
	handleHelloMessage := makeHelloHandler(id, n, messageLog, unicastLogs, stop)

	wg := new(sync.WaitGroup)
	if err := startPeerConnections(id, n, stack, handlePeerMessage, stop, wg, logger); err != nil {
		return nil, fmt.Errorf("failed to start peer connections: %s", err)
	}

	wg.Add(1)
	go func() {
		handleOwnPeerMessages(messageLog, handleOwnMessage, stop, logger)
		wg.Done()
	}()

	return &replica{
		handlePeerStream:   makeMessageStreamHandler(handleHelloMessage, "peer", stop, logger),
		handleClientStream: makeMessageStreamHandler(handleClientMessage, "client", stop, logger),
		stopChan:           stop,
		wg:                 wg,
	}, nil
}

func (r *replica) Terminate() {
	close(r.stopChan)
	r.wg.Wait()
}

func (r *replica) PeerMessageStreamHandler() api.MessageStreamHandler {
	return r.handlePeerStream
}

func (r *replica) ClientMessageStreamHandler() api.MessageStreamHandler {
	return r.handleClientStream
}

func (handle messageStreamHandler) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		handle(in, out)
	}()
	return out
}
