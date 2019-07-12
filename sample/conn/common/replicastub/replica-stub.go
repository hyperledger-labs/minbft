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

func (s *replicaStub) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)

		<-s.ready
		for msg := range s.replica.HandleMessageStream(in) {
			out <- msg
		}
	}()

	return out
}

func (s *replicaStub) AssignReplica(replica api.ConnectionHandler) {
	s.replica = replica
	close(s.ready)
}
