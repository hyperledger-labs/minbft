// Copyright (c) 2021 NEC Laboratories Europe GmbH.
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

type newView struct {
	pbMsg  *pb.NewView
	nvCert messages.NewViewCert
	ui     *usig.UI
}

func newNewView(r uint32, nv uint64, nvCert messages.NewViewCert) *newView {
	return &newView{
		pbMsg: &pb.NewView{
			ReplicaId: r,
			NewView:   nv,
			NvCert:    pbNewViewCertFromAPI(nvCert),
		},
		nvCert: nvCert,
	}
}

func newNewViewFromPb(pbMsg *pb.NewView) (*newView, error) {
	ui := new(usig.UI)
	if err := ui.UnmarshalBinary(pbMsg.GetUi()); err != nil {
		return nil, xerrors.Errorf("cannot unmarshal UI: %w", err)
	}
	nvCert, err := newViewCertFromPb(pbMsg.GetNvCert())
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal new-view certificate: %w", err)
	}
	return &newView{pbMsg: pbMsg, nvCert: nvCert, ui: ui}, nil
}

func (m *newView) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *newView) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (m *newView) NewView() uint64 {
	return m.pbMsg.GetNewView()
}

func (m *newView) NewViewCert() messages.NewViewCert {
	return m.nvCert
}

func (m *newView) UI() *usig.UI {
	return m.ui
}

func (m *newView) SetUI(ui *usig.UI) {
	m.ui = ui
	m.pbMsg.Ui = usig.MustMarshalUI(ui)
}

func pbNewViewCertFromAPI(cert messages.NewViewCert) []*pb.ViewChange {
	pbCert := make([]*pb.ViewChange, 0, len(cert))
	for _, m := range cert {
		pbCert = append(pbCert, pbViewChangeFromAPI(m))
	}
	return pbCert
}

func newViewCertFromPb(pbCert []*pb.ViewChange) (messages.NewViewCert, error) {
	cert := make(messages.NewViewCert, 0, len(pbCert))
	for _, m := range pbCert {
		vc, err := newViewChangeFromPb(m)
		if err != nil {
			return nil, err
		}
		cert = append(cert, vc)
	}
	return cert, nil
}

func (newView) ImplementsReplicaMessage() {}
func (newView) ImplementsPeerMessage()    {}
func (newView) ImplementsNewView()        {}
