// Copyright (c) 2020 NEC Laboratories Europe GmbH.
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

package protobuf

import (
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type hello struct {
	pbMsg *pb.Hello
}

func newHello(r uint32) *hello {
	return &hello{pbMsg: &pb.Hello{
		ReplicaId: r,
	}}
}

func newHelloFromPb(pbMsg *pb.Hello) *hello {
	return &hello{pbMsg: pbMsg}
}

func (m *hello) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *hello) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (hello) ImplementsReplicaMessage() {}
func (hello) ImplementsPeerMessage()    {}
func (hello) ImplementsHello()          {}
