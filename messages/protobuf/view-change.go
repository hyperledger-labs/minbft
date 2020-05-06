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

type viewChange struct {
	pbMsg  *pb.ViewChange
	log    messages.MessageLog
	vcCert messages.ViewChangeCert
	ui     *usig.UI
}

func newViewChange(r uint32, nv uint64, log messages.MessageLog, vcCert messages.ViewChangeCert) *viewChange {
	return &viewChange{
		pbMsg: &pb.ViewChange{
			ReplicaId: r,
			NewView:   nv,
			Log:       pbMessageLogFromAPI(log),
			VcCert:    pbViewChangeCertFromAPI(vcCert),
		},
		log:    log,
		vcCert: vcCert,
	}
}

func newViewChangeFromPb(pbMsg *pb.ViewChange) (*viewChange, error) {
	ui := new(usig.UI)
	if err := ui.UnmarshalBinary(pbMsg.GetUi()); err != nil {
		return nil, xerrors.Errorf("cannot unmarshal UI: %w", err)
	}
	log, err := messageLogFromPb(pbMsg.GetLog())
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal message log: %w", err)
	}
	vcCert, err := viewChangeCertFromPb(pbMsg.GetVcCert())
	if err != nil {
		return nil, xerrors.Errorf("cannot unmarshal view-change certificate: %w", err)
	}
	return &viewChange{pbMsg: pbMsg, log: log, vcCert: vcCert, ui: ui}, nil
}

func (m *viewChange) MarshalBinary() ([]byte, error) {
	return marshalMessage(m.pbMsg)
}

func (m *viewChange) ReplicaID() uint32 {
	return m.pbMsg.GetReplicaId()
}

func (m *viewChange) NewView() uint64 {
	return m.pbMsg.GetNewView()
}

func (m *viewChange) MessageLog() messages.MessageLog {
	return m.log
}

func (m *viewChange) ViewChangeCert() messages.ViewChangeCert {
	return m.vcCert
}

func (m *viewChange) UI() *usig.UI {
	return m.ui
}

func (m *viewChange) SetUI(ui *usig.UI) {
	m.ui = ui
	m.pbMsg.Ui = usig.MustMarshalUI(ui)
}

func pbMessageLogFromAPI(log messages.MessageLog) []*pb.Message {
	pbLog := make([]*pb.Message, 0, len(log))
	for _, m := range log {
		pbLog = append(pbLog, pb.WrapMessage(pbMessageFromAPI(m)))
	}
	return pbLog
}

func messageLogFromPb(pbLog []*pb.Message) (messages.MessageLog, error) {
	log := make(messages.MessageLog, 0, len(pbLog))
	for _, pbMsg := range pbLog {
		typedMsg, err := typedMessageFromPb(pbMsg)
		if err != nil {
			return nil, err
		}
		m, ok := typedMsg.(messages.CertifiedMessage)
		if !ok {
			return nil, xerrors.New("unexpected message type")
		}
		log = append(log, m)
	}
	return log, nil
}

func pbViewChangeCertFromAPI(cert messages.ViewChangeCert) []*pb.ReqViewChange {
	pbCert := make([]*pb.ReqViewChange, 0, len(cert))
	for _, m := range cert {
		pbCert = append(pbCert, pbReqViewChangeFromAPI(m))
	}
	return pbCert
}

func viewChangeCertFromPb(pbCert []*pb.ReqViewChange) (messages.ViewChangeCert, error) {
	cert := make(messages.ViewChangeCert, 0, len(pbCert))
	for _, m := range pbCert {
		rvc, err := newReqViewChangeFromPb(m)
		if err != nil {
			return nil, err
		}
		cert = append(cert, rvc)
	}
	return cert, nil
}

func (viewChange) ImplementsReplicaMessage() {}
func (viewChange) ImplementsPeerMessage()    {}
func (viewChange) ImplementsViewChange()     {}
