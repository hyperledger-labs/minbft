// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

// Package connector implements ReplicaConnector interface to connect
// replica instances in the same processes. Useful for testing.
package connector

import (
	"github.com/hyperledger-labs/minbft/api"
)

// ReplicaConnector allows to connect replica instances in the same
// process directly. It is useful for testing.
type ReplicaConnector struct {
	replicas map[uint32]chan api.MessageStreamHandler
}

var _ api.ReplicaConnector = (*ReplicaConnector)(nil)

// New creates an instance of ReplicaConnector
func New(n int) *ReplicaConnector {
	replicas := make(map[uint32]chan api.MessageStreamHandler)
	for i := 0; i < n; i++ {
		replicas[uint32(i)] = make(chan api.MessageStreamHandler)
	}
	return &ReplicaConnector{replicas: replicas}
}

// ReplicaMessageStreamHandler returns the instance previously
// assigned to replica ID.
func (c *ReplicaConnector) ReplicaMessageStreamHandler(replicaID uint32) (api.MessageStreamHandler, error) {
	return &messageStreamHandler{replicaID, c}, nil
}

// ConnectReplica assigns a replica instance to replica ID.
func (c *ReplicaConnector) ConnectReplica(replicaID uint32, replica api.MessageStreamHandler) {
	c.replicas[replicaID] <- replica
}

// ConnectManyReplicas assigns replica instances to replica IDs
// according to the supplied map indexed by replica ID.
func (c *ReplicaConnector) ConnectManyReplicas(replicas map[uint32]api.MessageStreamHandler) {
	for id, r := range replicas {
		c.ConnectReplica(id, r)
	}
}

// messageStreamHandler is a local representation of the replica for
// the purpose of message exchange.
type messageStreamHandler struct {
	replicaID uint32
	connector *ReplicaConnector
}

// HandleMessageStream implements actual communication with other replica.
func (h *messageStreamHandler) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		replica := <-h.connector.replicas[h.replicaID]
		for msg := range replica.HandleMessageStream(in) {
			out <- msg
		}
	}()

	return out
}
