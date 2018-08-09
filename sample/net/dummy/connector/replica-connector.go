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

// ConnectReplica assigns a replica instance to replica ID.
func (c *ReplicaConnector) ConnectReplica(replicaID uint32, replica api.MessageStreamHandler) {
	c.replicas[replicaID] = replica
}

// ConnectManyReplicas assigns replica instances to replica IDs
// according to the supplied map indexed by replica ID.
func (c *ReplicaConnector) ConnectManyReplicas(replicas map[uint32]api.MessageStreamHandler) {
	for id, r := range replicas {
		c.ConnectReplica(id, r)
	}
}
