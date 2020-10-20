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

// Package connector implements ReplicaConnector interface using gRPC
// as a mechanism to exchange messages with replicas over the network
package connector

import (
	"fmt"

	"google.golang.org/grpc"

	"github.com/hyperledger-labs/minbft/api"
	common "github.com/hyperledger-labs/minbft/sample/conn/common/connector"
	pb "github.com/hyperledger-labs/minbft/sample/conn/grpc/proto"
)

// ReplicaConnector implements connectivity API using gRPC as a
// network communication mechanism.
//
// ConnectReplica method establishes a connection to a replica by its
// gRPC target address.
type ReplicaConnector interface {
	api.ReplicaConnector
	ConnectReplica(replicaID uint32, target string, dialOpts ...grpc.DialOption) error
}

// ConnectManyReplicas helps to establish connections to multiple
// replicas. It invokes ConnectReplica on the specified connector for
// each replica in the supplied map. The map holds gRPC target
// addresses indexed by replica ID.
func ConnectManyReplicas(conn ReplicaConnector, targets map[uint32]string, dialOpts ...grpc.DialOption) error {
	for id, target := range targets {
		err := conn.ConnectReplica(id, target, dialOpts...)
		if err != nil {
			return err
		}
	}
	return nil
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

// ConnectReplica establishes a connection to a replica by its gRPC
// target (address).
func (c *connector) ConnectReplica(replicaID uint32, target string, dialOpts ...grpc.DialOption) error {
	connection, err := grpc.Dial(target, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to dial replica: %s", err)
	}

	replica := &replica{
		rpcClient: pb.NewChannelClient(connection),
		id:        replicaID,
	}

	c.AssignReplica(replicaID, replica)

	return nil
}
