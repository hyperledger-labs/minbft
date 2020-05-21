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

type prepare struct {
	pbMsg *pb.Prepare
	req   messages.Request
}

func newPrepare(r uint32, v uint64, req messages.Request) *prepare {
	return &prepare{
		pbMsg: &pb.Prepare{
			ReplicaId: r,
			View:      v,
			Request:   pbRequestFromAPI(req),
		},
		req: req,
	}
}

func newPrepareFromPb(pbMsg *pb.Prepare) *prepare {
	req := newRequestFromPb(pbMsg.GetRequest())
	return &prepare{pbMsg: pbMsg, req: req}
}

func (m *prepare) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *prepare) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (m *prepare) View() uint64 {
	return m.pbMsg.GetView()
}

func (m *prepare) Request() messages.Request {
	return m.req
}

func (m *prepare) UIBytes() []byte {
	return m.pbMsg.Ui
}

func (m *prepare) SetUIBytes(uiBytes []byte) {
	m.pbMsg.Ui = uiBytes
}

func (prepare) ImplementsReplicaMessage() {}
func (prepare) ImplementsPeerMessage()    {}
func (prepare) ImplementsPrepare()        {}

func pbPrepareFromAPI(m messages.Prepare) *pb.Prepare {
	if m, ok := m.(*prepare); ok {
		return m.pbMsg
	}

	return pb.PrepareFromAPI(m)
}
