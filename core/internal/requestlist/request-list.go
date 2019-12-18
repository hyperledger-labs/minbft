// Copyright (c) 2018-2019 NEC Laboratories Europe GmbH.
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

// Package requestlist provides functionality to maintain a set of
// Request messages.
package requestlist

import (
	"sync"

	messages "github.com/hyperledger-labs/minbft/messages/protobuf"
)

// List defines methods to manipulate a set of Request messages. The
// set is only capable of keeping a single message per client. All
// methods are safe to invoke concurrently.
//
// Add method adds Request message to the set. Any previously added
// Request from the same client will be replaced.
//
// Remove method removes Request message of the specified client from
// the set, if any.
//
// All method returns all messages currently in the set.
type List interface {
	Add(req *messages.Request)
	Remove(clientID uint32)
	All() []*messages.Request
}

type msgMap map[uint32]*messages.Request

type list struct {
	sync.RWMutex
	messages msgMap
}

// New creates a new instance of List interface.
func New() List {
	return &list{
		messages: make(msgMap),
	}
}

func (l *list) Add(req *messages.Request) {
	l.Lock()
	defer l.Unlock()

	l.messages[req.ClientID()] = req
}

func (l *list) Remove(clientID uint32) {
	l.Lock()
	defer l.Unlock()

	delete(l.messages, clientID)
}

func (l *list) All() []*messages.Request {
	l.RLock()
	defer l.RUnlock()

	all := make([]*messages.Request, 0, len(l.messages))
	for _, m := range l.messages {
		all = append(all, m)
	}

	return all
}
