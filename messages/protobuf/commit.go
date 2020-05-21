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
	"golang.org/x/xerrors"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
	"github.com/hyperledger-labs/minbft/usig"
)

type commit struct {
	pbMsg *pb.Commit
	prep  messages.Prepare
	ui    *usig.UI
}

func newCommit(r uint32, prep messages.Prepare) *commit {
	return &commit{
		pbMsg: &pb.Commit{
			ReplicaId: r,
			Prepare:   pbPrepareFromAPI(prep),
		},
		prep: prep,
	}
}

func newCommitFromPb(pbMsg *pb.Commit) (*commit, error) {
	prep, err := newPrepareFromPb(pbMsg.GetPrepare())
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal embedded Prepare: %w", err)
	}
	ui := new(usig.UI)
	if err := ui.UnmarshalBinary(pbMsg.GetUi()); err != nil {
		return nil, xerrors.Errorf("cannot unmarshal UI: %w", err)
	}
	return &commit{pbMsg: pbMsg, prep: prep, ui: ui}, nil
}

func (m *commit) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *commit) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (m *commit) Prepare() messages.Prepare {
	return m.prep
}

func (m *commit) UI() *usig.UI {
	return m.ui
}

func (m *commit) SetUI(ui *usig.UI) {
	m.ui = ui
	m.pbMsg.Ui = usig.MustMarshalUI(ui)
}

func (commit) ImplementsReplicaMessage() {}
func (commit) ImplementsPeerMessage()    {}
func (commit) ImplementsCommit()         {}
