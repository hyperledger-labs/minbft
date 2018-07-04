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
	return uint64(rand.Int())
}

// randReplicaID return a random replica ID
func randReplicaID(n uint32) uint32 {
	return uint32(rand.Intn(int(n)))
}

// primaryID returns primary replica ID
func primaryID(n uint32, view uint64) uint32 {
	return uint32(view % uint64(n))
}

// randBackupID returns random backup replica ID
func randBackupID(n uint32, view uint64) uint32 {
	return randOtherReplicaID(primaryID(n, view), n)
}

// randOtherReplicaID returns an ID of some other replica
func randOtherReplicaID(id, n uint32) uint32 {
	offset := rand.Intn(int(n)-2) + 1
	return (id + uint32(offset)) % n
}
