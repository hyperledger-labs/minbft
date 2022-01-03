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

package testing

import (
	"testing"

	"github.com/hyperledger-labs/minbft/messages"
)

func DoTestMessageImpl(t *testing.T, impl messages.MessageImpl) {
	t.Run("Hello", func(t *testing.T) {
		DoTestHello(t, impl)
	})
	t.Run("Request", func(t *testing.T) {
		DoTestRequest(t, impl)
	})
	t.Run("Prepare", func(t *testing.T) {
		DoTestPrepare(t, impl)
	})
	t.Run("Commit", func(t *testing.T) {
		DoTestCommit(t, impl)
	})
	t.Run("Reply", func(t *testing.T) {
		DoTestReply(t, impl)
	})
	t.Run("ReqViewChange", func(t *testing.T) {
		DoTestReqViewChange(t, impl)
	})
	t.Run("ViewChange", func(t *testing.T) {
		DoTestViewChange(t, impl)
	})
	t.Run("NewView", func(t *testing.T) {
		DoTestNewView(t, impl)
	})
}
