// Copyright © 2018 NEC Solution Innovators, Ltd.
//
// Authors: Naoya Horiguchi <n-horiguchi@ah.jp.nec.com>
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

type options struct{}

// Option represents function type to set options.
type Option func(*options)

func newOptions(opts ...Option) options {
	opt := options{}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}
