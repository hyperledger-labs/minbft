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

	"github.com/golang/protobuf/proto"

	"github.com/hyperledger-labs/minbft/messages"
	"github.com/hyperledger-labs/minbft/messages/protobuf/pb"
)

type impl struct{}

func NewImpl() messages.MessageImpl {
	return &impl{}
}

func (*impl) NewFromBinary(data []byte) (messages.Message, error) {
	pbMsg := &pb.Message{}
	if err := proto.Unmarshal(data, pbMsg); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal message wrapper: %w", err)
	}

	switch t := pbMsg.Type.(type) {
	case *pb.Message_Request:
		return newRequestFromPb(t.Request), nil
	case *pb.Message_Prepare:
		return newPrepareFromPb(t.Prepare), nil
	case *pb.Message_Commit:
		return newCommitFromPb(t.Commit), nil
	case *pb.Message_Reply:
		return newReplyFromPb(t.Reply), nil
	default:
		return nil, xerrors.New("unknown message type")
	}
}

func (*impl) NewRequest(cl uint32, seq uint64, op []byte) messages.Request {
	return newRequest(cl, seq, op)
}

func (*impl) NewPrepare(r uint32, v uint64, req messages.Request) messages.Prepare {
	return newPrepare(r, v, req)
}

func (*impl) NewCommit(r uint32, prep messages.Prepare) messages.Commit {
	return newCommit(r, prep)
}

func (*impl) NewReply(r, cl uint32, seq uint64, res []byte) messages.Reply {
	return newReply(r, cl, seq, res)
}
