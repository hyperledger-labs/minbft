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

type prepare struct {
	Prepare
}

func newPrepare() *prepare {
	return &prepare{}
}

func (m *prepare) init(r uint32, v uint64, req messages.Request) {
	m.Prepare = Prepare{Msg: &Prepare_M{
		ReplicaId: r,
		View:      v,
		Request:   pbRequestFromAPI(req),
	}}
}

func (m *prepare) set(pbMsg *Prepare) {
	m.Prepare = *pbMsg
}

func (m *prepare) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&Message{Type: &Message_Prepare{Prepare: &m.Prepare}})
}

func (m *prepare) ReplicaID() uint32 {
	return m.Msg.GetReplicaId()
}

func (m *prepare) View() uint64 {
	return m.Msg.GetView()
}

func (m *prepare) Request() messages.Request {
	req := newRequest()
	req.set(m.Msg.GetRequest())
	return req
}

func (m *prepare) CertifiedPayload() []byte {
	return MarshalOrPanic(m.Msg)
}

func (m *prepare) UIBytes() []byte {
	return m.ReplicaUi
}

func (m *prepare) SetUIBytes(uiBytes []byte) {
	m.ReplicaUi = uiBytes
}

func (prepare) ImplementsReplicaMessage() {}
func (prepare) ImplementsPrepare()        {}
