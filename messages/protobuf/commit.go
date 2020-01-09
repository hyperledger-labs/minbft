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
	"github.com/golang/protobuf/proto"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type commit struct {
	pbMsg pb.Commit
}

func newCommit() *commit {
	return &commit{}
}

func (m *commit) init(r uint32, prep messages.Prepare) {
	req := prep.Request()

	m.pbMsg = pb.Commit{Msg: &pb.Commit_M{
		ReplicaId: r,
		PrimaryId: prep.ReplicaID(),
		View:      prep.View(),
		Request:   pb.RequestFromAPI(req),
		PrimaryUi: prep.UIBytes(),
	}}
}

func (m *commit) set(pbMsg *pb.Commit) {
	m.pbMsg = *pbMsg
}

func (m *commit) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&pb.Message{Type: &pb.Message_Commit{Commit: &m.pbMsg}})
}

func (m *commit) ReplicaID() uint32 {
	return m.pbMsg.Msg.GetReplicaId()
}

func (m *commit) Prepare() messages.Prepare {
	prep := newPrepare()
	prep.set(&pb.Prepare{
		Msg: &pb.Prepare_M{
			ReplicaId: m.pbMsg.Msg.GetPrimaryId(),
			View:      m.pbMsg.Msg.GetView(),
			Request:   m.pbMsg.Msg.GetRequest(),
		},
		ReplicaUi: m.pbMsg.Msg.GetPrimaryUi(),
	})
	return prep
}

func (m *commit) CertifiedPayload() []byte {
	return pb.MarshalOrPanic(m.pbMsg.Msg)
}

func (m *commit) UIBytes() []byte {
	return m.pbMsg.ReplicaUi
}

func (m *commit) SetUIBytes(uiBytes []byte) {
	m.pbMsg.ReplicaUi = uiBytes
}

func (commit) ImplementsReplicaMessage() {}
func (commit) ImplementsCommit()         {}
