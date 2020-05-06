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
