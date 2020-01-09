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
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type reply struct {
	pbMsg *pb.Reply
}

func newReply(r, cl uint32, seq uint64, res []byte) *reply {
	return &reply{pbMsg: &pb.Reply{Msg: &pb.Reply_M{
		ReplicaId: r,
		ClientId:  cl,
		Seq:       seq,
		Result:    res,
	}}}
}

func newReplyFromPb(pbMsg *pb.Reply) *reply {
	return &reply{pbMsg: pbMsg}
}

func (m *reply) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *reply) ReplicaID() uint32 {
	return m.pbMsg.GetMsg().GetReplicaId()
}

func (m *reply) ClientID() uint32 {
	return m.pbMsg.GetMsg().GetClientId()
}

func (m *reply) Sequence() uint64 {
	return m.pbMsg.GetMsg().GetSeq()
}

func (m *reply) Result() []byte {
	return m.pbMsg.GetMsg().GetResult()
}

func (m *reply) SignedPayload() []byte {
	return pb.AuthenBytesFromReply(m.pbMsg)
}

func (m *reply) Signature() []byte {
	return m.pbMsg.Signature
}

func (m *reply) SetSignature(signature []byte) {
	m.pbMsg.Signature = signature
}

func (reply) ImplementsReplicaMessage() {}
func (reply) ImplementsReply()          {}
