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

package protobuf

import (
	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type commit struct {
	pbMsg *pb.Commit
}

func newCommit(r uint32, prep messages.Prepare) *commit {
	return &commit{pbMsg: &pb.Commit{
		ReplicaId: r,
		Prepare:   pbPrepareFromAPI(prep),
	}}
}

func newCommitFromPb(pbMsg *pb.Commit) *commit {
	return &commit{pbMsg: pbMsg}
}

func (m *commit) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *commit) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (m *commit) Prepare() messages.Prepare {
	return newPrepareFromPb(m.pbMsg.GetPrepare())
}

func (m *commit) UIBytes() []byte {
	return m.pbMsg.Ui
}

func (m *commit) SetUIBytes(uiBytes []byte) {
	m.pbMsg.Ui = uiBytes
}

func (commit) ImplementsReplicaMessage() {}
func (commit) ImplementsPeerMessage()    {}
func (commit) ImplementsCommit()         {}
