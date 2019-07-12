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

// Package connector implements generally useful replica connector
// functionality.
package connector

import "github.com/hyperledger-labs/minbft/api"

// ReplicaConnector represents a generic implementation of
// connectivity interface. It simply dispatches API method calls to
// corresponding local replica representations. Replica
// representations should be assigned with SetReplica method before it
// can be used.
//
// AssignReplica method assigns a replica representation to the
// replica identifier.
type ReplicaConnector interface {
	api.ReplicaConnector
	AssignReplica(id uint32, replica api.ConnectionHandler)
}

type connector struct {
	replicas map[uint32]api.ConnectionHandler
}

// NewReplicaConnector creates a new instance of ReplicaConnector.
func New() ReplicaConnector {
	return &connector{
		replicas: make(map[uint32]api.ConnectionHandler),
	}
}

func (c *connector) ReplicaMessageStreamHandler(id uint32) api.MessageStreamHandler {
	return c.replicas[id]
}

func (c *connector) AssignReplica(id uint32, replica api.ConnectionHandler) {
	c.replicas[id] = replica
}
