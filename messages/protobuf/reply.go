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
)

type reply struct {
	Reply
}

func newReply() *reply {
	return &reply{}
}

func (m *reply) init(r, cl uint32, seq uint64, res []byte) {
	m.Reply = Reply{Msg: &Reply_M{
		ReplicaId: r,
		ClientId:  cl,
		Seq:       seq,
		Result:    res,
	}}
}

func (m *reply) set(pbMsg *Reply) {
	m.Reply = *pbMsg
}

func (m *reply) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&Message{Type: &Message_Reply{Reply: &m.Reply}})
}

func (m *reply) ReplicaID() uint32 {
	return m.Msg.GetReplicaId()
}

func (m *reply) ClientID() uint32 {
	return m.Msg.GetClientId()
}

func (m *reply) Sequence() uint64 {
	return m.Msg.GetSeq()
}

func (m *reply) Result() []byte {
	return m.Msg.GetResult()
}

func (m *reply) SignedPayload() []byte {
	return MarshalOrPanic(m.Msg)
}

func (m *reply) Signature() []byte {
	return m.Reply.Signature
}

func (m *reply) SetSignature(signature []byte) {
	m.Reply.Signature = signature
}

func (reply) ImplementsReplicaMessage() {}
func (reply) ImplementsReply()          {}
