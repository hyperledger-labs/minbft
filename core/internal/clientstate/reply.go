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
	"sync"

	"github.com/hyperledger-labs/minbft/messages"
)

type replyState struct {
	sync.RWMutex

	// Last replied request ID
	lastRepliedSeq uint64

	// Last Reply message
	reply messages.Reply

	// Channel to close and recreate when new Reply added
	waitChan chan struct{}
}

func newReplyState() *replyState {
	return &replyState{
		waitChan: make(chan struct{}),
	}
}

func (s *replyState) AddReply(reply messages.Reply) {
	seq := reply.Sequence()

	s.Lock()
	defer s.Unlock()

	if seq <= s.lastRepliedSeq {
		return
	}

	s.reply = reply
	s.lastRepliedSeq = seq

	defer close(s.waitChan)
	s.waitChan = make(chan struct{})
}

func (s *replyState) ReplyChannel(seq uint64, cancel <-chan struct{}) <-chan messages.Reply {
	out := make(chan messages.Reply, 1)

	go func() {
		defer close(out)

		for {
			s.RLock()
			lastRepliedSeq := s.lastRepliedSeq
			reply := s.reply
			waitChan := s.waitChan
			s.RUnlock()

			if seq <= lastRepliedSeq {
				if reply.Sequence() == seq {
					out <- reply
				}
				return
			}

			select {
			case <-waitChan:
			case <-cancel:
				return
			}
		}
	}()

	return out
}
