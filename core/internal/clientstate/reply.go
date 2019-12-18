// Copyright (c) 2018 NEC Laboratories Europe GmbH.
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

package clientstate

import (
	"fmt"
	"sync"

	"github.com/hyperledger-labs/minbft/messages"
)

type replyState struct {
	sync.Mutex

	// Last replied request ID
	lastRepliedSeq uint64

	// Last Reply message
	reply messages.Reply

	// Channels to close when new Reply added
	replyAdded []chan<- struct{}
}

func newReplyState() *replyState {
	return &replyState{}
}

func (s *replyState) AddReply(reply messages.Reply) error {
	seq := reply.Sequence()

	s.Lock()
	defer s.Unlock()

	if seq <= s.lastRepliedSeq {
		return fmt.Errorf("old request ID")
	}

	s.reply = reply
	s.lastRepliedSeq = seq

	for _, ch := range s.replyAdded {
		close(ch)
	}
	s.replyAdded = nil

	return nil
}

func (s *replyState) ReplyChannel(seq uint64) <-chan messages.Reply {
	out := make(chan messages.Reply, 1)

	go func() {
		defer close(out)

		s.Lock()
		for s.lastRepliedSeq < seq {
			s.waitForReplyLocked()
		}
		reply := s.reply
		s.Unlock()

		if reply.Sequence() == seq {
			out <- reply
		}
	}()

	return out
}

func (s *replyState) waitForReplyLocked() {
	replyAdded := make(chan struct{})
	s.replyAdded = append(s.replyAdded, replyAdded)

	s.Unlock()
	<-replyAdded
	s.Lock()
}
