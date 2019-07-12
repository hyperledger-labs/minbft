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
//
// ConnectReplica method assigns a replica instance, given its ID.
type ReplicaConnector interface {
	api.ReplicaConnector
	ConnectReplica(id uint32, replica api.ConnectionHandler)
}

// ConnectManyReplicas helps to assign multiple replica instances. It
// invokes ConnectReplica on the specified connector for each replica
// in the supplied map indexed by replica ID.
func ConnectManyReplicas(conn ReplicaConnector, replicas map[uint32]api.ConnectionHandler) {
	for id, r := range replicas {
		conn.ConnectReplica(id, r)
	}
}

type connector struct {
	replicas []api.ConnectionHandler
	ready    []chan bool
}

// New creates an instance of ReplicaConnector
func New(n int) ReplicaConnector {
	ready := make([]chan bool, n)
	for i := 0; i < n; i++ {
		ready[uint32(i)] = make(chan bool)
	}
	return &connector{
		replicas: make([]api.ConnectionHandler, n),
		ready:    ready,
	}
}

// ReplicaMessageStreamHandler returns the instance previously
// assigned to replica ID.
func (c *connector) ReplicaMessageStreamHandler(replicaID uint32) api.MessageStreamHandler {
	if replicaID >= uint32(len(c.replicas)) {
		return nil
	}
	return &messageStreamHandler{replicaID, c}
}

func (c *connector) ConnectReplica(replicaID uint32, replica api.ConnectionHandler) {
	c.replicas[replicaID] = replica
	close(c.ready[replicaID])
}

// messageStreamHandler is a local representation of the replica for
// the purpose of message exchange.
type messageStreamHandler struct {
	replicaID uint32
	connector *connector
}

// HandleMessageStream implements actual communication with other replica.
func (h *messageStreamHandler) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		<-h.connector.ready[h.replicaID]
		replica := h.connector.replicas[h.replicaID]
		for msg := range replica.HandleMessageStream(in) {
			out <- msg
		}
	}()

	return out
}
