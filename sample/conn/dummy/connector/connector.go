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

// Package connector provides a mechanism for connecting replica
// instances in the same process. Useful for testing.
package connector

import (
	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/sample/conn/common/replicastub"

	common "github.com/hyperledger-labs/minbft/sample/conn/common/connector"
)

// ReplicaConnector allows to connect replica instances within the
// same process directly.
//
// AssignReplicaStub method associates the supplied replica ID with
// the replica stub.
type ReplicaConnector interface {
	api.ReplicaConnector
	AssignReplicaStub(id uint32, stub replicastub.ReplicaStub)
}

type connector struct {
	common.ReplicaConnector
}

// NewClientSide creates a new instance of ReplicaConnector to use at
// client side, i.e. initiate client-to-replica connections.
func NewClientSide() ReplicaConnector {
	return &connector{common.NewClientSide()}
}

// NewReplicaSide creates a new instance of ReplicaConnector to use at
// replica side, i.e. initiate replica-to-replica connections.
func NewReplicaSide() ReplicaConnector {
	return &connector{common.NewReplicaSide()}
}

func (c *connector) AssignReplicaStub(id uint32, stub replicastub.ReplicaStub) {
	c.AssignReplica(id, stub)
}
