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

import (
	"os"

	logging "github.com/op/go-logging"
)

type options struct {
	logLevel logging.Level
	logFile  *os.File
}

// Option represents function type to set options.
type Option func(*options)

func newOptions(opts ...Option) options {
	opt := options{
		logLevel: logging.DEBUG,
		logFile:  os.Stdout,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

// WithLogLevel sets logging level
func WithLogLevel(l logging.Level) Option {
	return func(opts *options) {
		opts.logLevel = l
	}
}

// WithLogFile sets file path of logging file
func WithLogFile(f *os.File) Option {
	return func(opts *options) {
		opts.logFile = f
	}
}
