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
	"fmt"

	"github.com/hyperledger-labs/minbft/api"
)

// ReplicaConnector allows to connect replica instances in the same
// process directly. It is useful for testing.
type ReplicaConnector struct {
	replicas map[uint32]api.MessageStreamHandler
}

var _ api.ReplicaConnector = (*ReplicaConnector)(nil)

// New creates an instance of ReplicaConnector
func New() *ReplicaConnector {
	return &ReplicaConnector{
		replicas: make(map[uint32]api.MessageStreamHandler),
	}
}

// ReplicaMessageStreamHandler returns the instance previously
// assigned to replica ID.
func (c *ReplicaConnector) ReplicaMessageStreamHandler(replicaID uint32) (api.MessageStreamHandler, error) {
	replica, ok := c.replicas[replicaID]
	if !ok {
		return nil, fmt.Errorf("No message stream handler registered for replica %d", replicaID)
	}
	return replica, nil
}

// ConnectReplica initializes a messageStreamHandler instance.
func (c *ReplicaConnector) ConnectReplica(replicaID uint32, ready chan bool) {
	c.replicas[replicaID] = &messageStreamHandler{replicaID, ready, nil}
}

// ConnectManyReplicas assigns messageStreamHandlers for a set of replica IDs.
func (c *ReplicaConnector) ConnectManyReplicas(replicas map[uint32]chan bool) {
	for id, r := range replicas {
		c.ConnectReplica(id, r)
	}
}

// SetReady sets replica instance to replica connector.
func (c *ReplicaConnector) SetReady(replicaID uint32, replica api.MessageStreamHandler) error {
	r, ok := c.replicas[replicaID]
	if !ok {
		return fmt.Errorf("messageStreamHandler not available for replica %d", replicaID)
	}
	r.(*messageStreamHandler).setReplica(replica)
	return nil
}

// messageStreamHandler is a local representation of the replica for
// the purpose of message exchange.
type messageStreamHandler struct {
	replicaID uint32
	ready     chan bool
	replica   api.MessageStreamHandler
}

// SetReady sets a replica instance and notifies that we are now ready to
// start handling messages from h.replica.HandleMessageStream.
func (h *messageStreamHandler) setReplica(replica api.MessageStreamHandler) {
	h.replica = replica
	h.ready <- true
}

// HandleMessageStream implements actual communication with other replica.
func (h *messageStreamHandler) HandleMessageStream(in <-chan []byte) <-chan []byte {
	out := make(chan []byte)

	go func() {
		<-h.ready
		for msg := range h.replica.HandleMessageStream(in) {
			out <- msg
		}
	}()

	return out
}
