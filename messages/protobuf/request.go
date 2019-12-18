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

type request struct {
	messages.IsRequest
	Request
}

func newRequest() *request {
	return &request{}
}

func (m *request) init(cl uint32, seq uint64, op []byte) {
	m.Request = Request{Msg: &Request_M{
		ClientId: cl,
		Seq:      seq,
		Payload:  op,
	}}
}

func (m *request) set(pbMsg *Request) {
	m.Request = *pbMsg
}

func (m *request) MarshalBinary() ([]byte, error) {
	return proto.Marshal(&Message{Type: &Message_Request{Request: &m.Request}})
}

func (m *request) ClientID() uint32 {
	return m.Msg.GetClientId()
}

func (m *request) Sequence() uint64 {
	return m.Msg.GetSeq()
}

func (m *request) Operation() []byte {
	return m.Msg.GetPayload()
}

func (m *request) SignedPayload() []byte {
	return MarshalOrPanic(m.Msg)
}

func (m *request) Signature() []byte {
	return m.Request.Signature
}

func (m *request) SetSignature(signature []byte) {
	m.Request.Signature = signature
}
