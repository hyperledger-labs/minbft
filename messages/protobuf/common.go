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

	"google.golang.org/protobuf/proto"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

func typedMessageFromPb(pbMsg *pb.Message) (messages.Message, error) {
	switch t := pbMsg.Typed.(type) {
	case *pb.Message_Hello:
		return newHelloFromPb(t.Hello)
	case *pb.Message_Request:
		return newRequestFromPb(t.Request)
	case *pb.Message_Reply:
		return newReplyFromPb(t.Reply)
	case *pb.Message_Prepare:
		return newPrepareFromPb(t.Prepare)
	case *pb.Message_Commit:
		return newCommitFromPb(t.Commit)
	case *pb.Message_ReqViewChange:
		return newReqViewChangeFromPb(t.ReqViewChange)
	default:
		return nil, xerrors.New("unknown message type")
	}
}

func pbMessageFromAPI(m messages.Message) proto.Message {
	switch m := m.(type) {
	case messages.Request:
		return pbRequestFromAPI(m)
	case messages.Reply:
		return pbReplyFromAPI(m)
	case messages.Prepare:
		return pbPrepareFromAPI(m)
	case messages.Commit:
		return pbCommitFromAPI(m)
	case messages.ReqViewChange:
		return pbReqViewChangeFromAPI(m)
	default:
		panic("unknown message type")
	}
}

func pbRequestFromAPI(m messages.Request) *pb.Request {
	if m, ok := m.(*request); ok {
		return m.pbMsg
	}
	return pb.RequestFromAPI(m)
}

func pbReplyFromAPI(m messages.Reply) *pb.Reply {
	if m, ok := m.(*reply); ok {
		return m.pbMsg
	}
	return pb.ReplyFromAPI(m)
}

func pbPrepareFromAPI(m messages.Prepare) *pb.Prepare {
	if m, ok := m.(*prepare); ok {
		return m.pbMsg
	}
	return pb.PrepareFromAPI(m)
}

func pbCommitFromAPI(m messages.Commit) *pb.Commit {
	if m, ok := m.(*commit); ok {
		return m.pbMsg
	}
	return pb.CommitFromAPI(m)
}

func pbReqViewChangeFromAPI(m messages.ReqViewChange) *pb.ReqViewChange {
	if m, ok := m.(*reqViewChange); ok {
		return m.pbMsg
	}
	return pb.ReqViewChangeFromAPI(m)
}
