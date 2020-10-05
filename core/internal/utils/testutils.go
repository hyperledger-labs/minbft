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

package utils

import (
	"math/rand"
)

// RandN returns a random number of replicas for testing
func RandN() uint32 {
	return uint32(rand.Intn(256-3) + 3)
}

// RandView returns a random view for testing
func RandView() uint64 {
	// Using uint32 gives enough room to increment the view number
	// without overflowing uint64.
	return uint64(rand.Uint32())
}

// RandOtherView returns a distinct random view
func RandOtherView(view uint64) uint64 {
	for {
		otherView := RandView()
		if otherView != view {
			return otherView
		}
	}
}

// PrimaryID returns primary replica ID
func PrimaryID(n uint32, view uint64) uint32 {
	return uint32(view % uint64(n))
}

// ViewForPrimary returns a random view given its primary ID
func ViewForPrimary(n uint32, id uint32) uint64 {
	otherView := RandView()
	return otherView - otherView%uint64(n) + uint64(id)
}

// RandReplicaID return random replica ID
func RandReplicaID(n uint32) uint32 {
	return uint32(rand.Intn(int(n)))
}

// RandOtherReplicaID returns an ID of some other replica
func RandOtherReplicaID(id, n uint32) uint32 {
	offset := rand.Intn(int(n)-1) + 1
	return (id + uint32(offset)) % n
}

// RandBytes returns a slice of random bytes
func RandBytes() []byte {
	return []byte{byte(rand.Int())} // nolint:gosec
}
