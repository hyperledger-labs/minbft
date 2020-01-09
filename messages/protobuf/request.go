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

	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type request struct {
	pbMsg pb.Request
}

func newRequest() *request {
	return &request{}
}

func (m *request) init(cl uint32, seq uint64, op []byte) {
	m.pbMsg = pb.Request{Msg: &pb.Request_M{
		ClientId: cl,
		Seq:      seq,
		Payload:  op,
	}}
}

func (m *request) set(pbMsg *pb.Request) {
	m.pbMsg = *pbMsg
}

func (m *request) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&pb.Message{Type: &pb.Message_Request{Request: &m.pbMsg}})
}

func (m *request) ClientID() uint32 {
	return m.pbMsg.Msg.GetClientId()
}

func (m *request) Sequence() uint64 {
	return m.pbMsg.Msg.GetSeq()
}

func (m *request) Operation() []byte {
	return m.pbMsg.Msg.GetPayload()
}

func (m *request) SignedPayload() []byte {
	return pb.MarshalOrPanic(m.pbMsg.Msg)
}

func (m *request) Signature() []byte {
	return m.pbMsg.Signature
}

func (m *request) SetSignature(signature []byte) {
	m.pbMsg.Signature = signature
}

func (request) ImplementsClientMessage() {}
func (request) ImplementsRequest()       {}
