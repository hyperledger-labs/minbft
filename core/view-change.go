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

package minbft

// viewProvider returns the current active view number.
//
// If there is a view change operation in progress, it waits for the
// replica to switch back to the normal mode. No view change will
// begin until the returned release function is invoked. It is safe to
// invoke concurrently.
type viewProvider func() (view uint64, release func())

// viewWaiter waits for a view to become the current active view.
//
// True is returned once the replica operates in normal mode and the
// supplied view number is the current active view. In that case, no
// view change will begin until the returned release function is
// invoked. False is returned as soon as it is detected that the
// supplied view number will never become active. It is safe to invoke
// concurrently.
type viewWaiter func(view uint64) (ok bool, release func())
