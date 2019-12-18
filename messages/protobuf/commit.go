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
)

type commit struct {
	messages.IsCommit
	Commit
}

func newCommit() *commit {
	return &commit{}
}

func (m *commit) init(r uint32, prep messages.Prepare) {
	req := prep.Request()

	m.Commit = Commit{Msg: &Commit_M{
		ReplicaId: r,
		PrimaryId: prep.ReplicaID(),
		View:      prep.View(),
		Request:   pbRequestFromAPI(req),
		PrimaryUi: prep.UIBytes(),
	}}
}

func (m *commit) set(pbMsg *Commit) {
	m.Commit = *pbMsg
}

func (m *commit) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&Message{Type: &Message_Commit{Commit: &m.Commit}})
}

func (m *commit) ReplicaID() uint32 {
	return m.Msg.GetReplicaId()
}

func (m *commit) Prepare() messages.Prepare {
	prep := newPrepare()
	prep.set(&Prepare{
		Msg: &Prepare_M{
			ReplicaId: m.Msg.GetPrimaryId(),
			View:      m.Msg.GetView(),
			Request:   m.Msg.GetRequest(),
		},
		ReplicaUi: m.Msg.GetPrimaryUi(),
	})
	return prep
}

func (m *commit) CertifiedPayload() []byte {
	return MarshalOrPanic(m.Msg)
}

func (m *commit) UIBytes() []byte {
	return m.ReplicaUi
}

func (m *commit) SetUIBytes(uiBytes []byte) {
	m.ReplicaUi = uiBytes
}
