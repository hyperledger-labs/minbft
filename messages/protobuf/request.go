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

type request struct {
	pbMsg *pb.Request
}

func newRequest(cl uint32, seq uint64, op []byte) *request {
	return &request{pbMsg: &pb.Request{
		ClientId:  cl,
		Seq:       seq,
		Operation: op,
	}}
}

func newRequestFromPb(pbMsg *pb.Request) (*request, error) {
	return &request{pbMsg: pbMsg}, nil
}

func (m *request) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *request) ClientID() uint32 {
	return m.pbMsg.GetClientId()
}

func (m *request) Sequence() uint64 {
	return m.pbMsg.GetSeq()
}

func (m *request) Operation() []byte {
	return m.pbMsg.GetOperation()
}

func (m *request) Signature() []byte {
	return m.pbMsg.Signature
}

func (m *request) SetSignature(signature []byte) {
	m.pbMsg.Signature = signature
}

func (request) ImplementsClientMessage() {}
func (request) ImplementsRequest()       {}

func pbRequestFromAPI(m messages.Request) *pb.Request {
	if m, ok := m.(*request); ok {
		return m.pbMsg
	}

	return pb.RequestFromAPI(m)
}
