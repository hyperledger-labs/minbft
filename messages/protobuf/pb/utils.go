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

package pb

import (
	"google.golang.org/protobuf/proto"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/usig"
)

func MarshalOrPanic(m proto.Message) []byte {
	bytes, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bytes
}

func WrapMessage(m proto.Message) *Message {
	switch m := m.(type) {
	case *Hello:
		return &Message{Typed: &Message_Hello{Hello: m}}
	case *Request:
		return &Message{Typed: &Message_Request{Request: m}}
	case *Reply:
		return &Message{Typed: &Message_Reply{Reply: m}}
	case *Prepare:
		return &Message{Typed: &Message_Prepare{Prepare: m}}
	case *Commit:
		return &Message{Typed: &Message_Commit{Commit: m}}
	case *ReqViewChange:
		return &Message{Typed: &Message_ReqViewChange{ReqViewChange: m}}
	case *ViewChange:
		return &Message{Typed: &Message_ViewChange{ViewChange: m}}
	default:
		panic("unknown message type")
	}
}

func HelloFromAPI(h messages.Hello) *Hello {
	return &Hello{
		ReplicaId: h.ReplicaID(),
	}
}

func MessageFromAPI(m messages.Message) proto.Message {
	switch m := m.(type) {
	case messages.Request:
		return RequestFromAPI(m)
	case messages.Reply:
		return ReplyFromAPI(m)
	case messages.Prepare:
		return PrepareFromAPI(m)
	case messages.Commit:
		return CommitFromAPI(m)
	case messages.ReqViewChange:
		return ReqViewChangeFromAPI(m)
	case messages.ViewChange:
		return ViewChangeFromAPI(m)
	default:
		panic("unknown message type")
	}
}

func RequestFromAPI(req messages.Request) *Request {
	return &Request{
		ClientId:  req.ClientID(),
		Seq:       req.Sequence(),
		Operation: req.Operation(),
		Signature: req.Signature(),
	}
}

func ReplyFromAPI(reply messages.Reply) *Reply {
	return &Reply{
		ReplicaId: reply.ReplicaID(),
		ClientId:  reply.ClientID(),
		Seq:       reply.Sequence(),
		Result:    reply.Result(),
		Signature: reply.Signature(),
	}
}

func PrepareFromAPI(prep messages.Prepare) *Prepare {
	return &Prepare{
		ReplicaId: prep.ReplicaID(),
		View:      prep.View(),
		Request:   RequestFromAPI(prep.Request()),
		Ui:        usig.MustMarshalUI(prep.UI()),
	}
}

func CommitFromAPI(comm messages.Commit) *Commit {
	return &Commit{
		ReplicaId: comm.ReplicaID(),
		Prepare:   PrepareFromAPI(comm.Prepare()),
		Ui:        usig.MustMarshalUI(comm.UI()),
	}
}

func ReqViewChangeFromAPI(rvc messages.ReqViewChange) *ReqViewChange {
	return &ReqViewChange{
		ReplicaId: rvc.ReplicaID(),
		NewView:   rvc.NewView(),
		Signature: rvc.Signature(),
	}
}

func ViewChangeFromAPI(vc messages.ViewChange) *ViewChange {
	return &ViewChange{
		ReplicaId: vc.ReplicaID(),
		NewView:   vc.NewView(),
		Log:       messageLogFromAPI(vc.MessageLog()),
		VcCert:    viewChangeCertFromAPI(vc.ViewChangeCert()),
		Ui:        usig.MustMarshalUI(vc.UI()),
	}
}

func messageLogFromAPI(log messages.MessageLog) []*Message {
	pbLog := make([]*Message, 0, len(log))
	for _, m := range log {
		pbLog = append(pbLog, WrapMessage(MessageFromAPI(m)))
	}
	return pbLog
}

func viewChangeCertFromAPI(cert messages.ViewChangeCert) []*ReqViewChange {
	pbCert := make([]*ReqViewChange, 0, len(cert))
	for _, m := range cert {
		pbCert = append(pbCert, ReqViewChangeFromAPI(m))
	}
	return pbCert
}
