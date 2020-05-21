// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
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

package minbft

import (
	"math/rand"
)

// randN returns a random number of replicas for testing
func randN() uint32 {
	return uint32(rand.Intn(256-3) + 3)
}

// randView returns a random view for testing
func randView() uint64 {
	// Using uint32 gives enough room to increment the view number
	// without overflowing uint64.
	return uint64(rand.Uint32())
}

// randOtherView returns a distinct random view
func randOtherView(view uint64) uint64 {
	for {
		otherView := randView()
		if otherView != view {
			return otherView
		}
	}
}

// primaryID returns primary replica ID
func primaryID(n uint32, view uint64) uint32 {
	return uint32(view % uint64(n))
}

// viewForPrimary returns a random view given its primary ID
func viewForPrimary(n uint32, id uint32) uint64 {
	otherView := randView()
	return otherView - otherView%uint64(n) + uint64(id)
}

// randReplicaID return random replica ID
func randReplicaID(n uint32) uint32 {
	return uint32(rand.Intn(int(n)))
}

// randOtherReplicaID returns an ID of some other replica
func randOtherReplicaID(id, n uint32) uint32 {
	offset := rand.Intn(int(n)-1) + 1
	return (id + uint32(offset)) % n
}

// randBytes returns a slice of random bytes
func randBytes() []byte {
	return []byte{byte(rand.Int())} // nolint:gosec
}
