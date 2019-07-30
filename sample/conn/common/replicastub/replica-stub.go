// Copyright (c) 2019 NEC Laboratories Europe GmbH.
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

// Package replicastub implements a representation of replica that
// might not be immediately available.
package replicastub

import "github.com/hyperledger-labs/minbft/api"

// ReplicaStub represents a representation of replica that might not
// be immediately available. It dispatches its API method calls to the
// appropriate instance once it is assigned.
//
// AssignReplica method assigns an instance of replica representation.
type ReplicaStub interface {
	api.ConnectionHandler
	AssignReplica(replica api.ConnectionHandler)
}

type replicaStub struct {
	replica api.ConnectionHandler
	ready   chan struct{}
}

// New creates a new instance of ReplicaStub.
func New() ReplicaStub {
	return &replicaStub{
		ready: make(chan struct{}),
	}
}

func (s *replicaStub) PeerMessageStreamHandler() api.MessageStreamHandler {
	sh := newMessageStreamHandlerStub()

	go func() {
		sh.assign(s.waitReplica().PeerMessageStreamHandler())
	}()

	return sh
}

func (s *replicaStub) ClientMessageStreamHandler() api.MessageStreamHandler {
	sh := newMessageStreamHandlerStub()

	go func() {
		sh.assign(s.waitReplica().ClientMessageStreamHandler())
	}()

	return sh
}

func (s *replicaStub) AssignReplica(replica api.ConnectionHandler) {
	s.replica = replica
	close(s.ready)
}

func (s *replicaStub) waitReplica() api.ConnectionHandler {
	<-s.ready
	return s.replica
}

type messageStreamHandlerStub struct {
	handler api.MessageStreamHandler
	ready   chan struct{}
}

func newMessageStreamHandlerStub() *messageStreamHandlerStub {
	return &messageStreamHandlerStub{
		ready: make(chan struct{}),
	}
}

func (s *messageStreamHandlerStub) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)

		<-s.ready
		if s.handler == nil {
			return
		}
		for msg := range s.handler.HandleMessageStream(in) {
			out <- msg
		}
	}()

	return out
}

func (s *messageStreamHandlerStub) assign(sh api.MessageStreamHandler) {
	s.handler = sh
	close(s.ready)
}
