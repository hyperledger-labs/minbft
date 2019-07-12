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

type common struct {
	replicas map[uint32]api.ConnectionHandler
}

type clientSide struct {
	*common
}

type replicaSide struct {
	*common
}

// NewClientSide creates a new instance of ReplicaConnector to use at
// client side, i.e. initiate client-to-replica connections.
func NewClientSide() ReplicaConnector {
	return &clientSide{
		common: newCommon(),
	}
}

// NewReplicaSide creates a new instance of ReplicaConnector to use at
// replica side, i.e. initiate replica-to-replica connections.
func NewReplicaSide() ReplicaConnector {
	return &replicaSide{
		common: newCommon(),
	}
}

func (c *clientSide) ReplicaMessageStreamHandler(id uint32) api.MessageStreamHandler {
	replica := c.replicas[id]
	if replica == nil {
		return nil
	}

	return replica.ClientMessageStreamHandler()
}

func (c *replicaSide) ReplicaMessageStreamHandler(id uint32) api.MessageStreamHandler {
	replica := c.replicas[id]
	if replica == nil {
		return nil
	}

	return replica.PeerMessageStreamHandler()
}

func newCommon() *common {
	return &common{
		replicas: make(map[uint32]api.ConnectionHandler),
	}
}

func (c *common) AssignReplica(id uint32, replica api.ConnectionHandler) {
	c.replicas[id] = replica
}
