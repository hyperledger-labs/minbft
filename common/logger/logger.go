// Copyright © 2018 NEC Solution Innovators, Ltd.
// Copyright © 2020 NEC Laboratories Europe GmbH.
//
// Authors: Naoya Horiguchi <n-horiguchi@ah.jp.nec.com>
//          Konstantin Munichev <konstantin.munichev@neclab.eu>
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

package logger

import (
	"fmt"
	"os"

	"github.com/op/go-logging"
)

const (
	module           = "minbft"
	defaultLogPrefix = `%{color}[%{module}] %{time:15:04:05.000} %{longfunc} ▶ %{level:.4s} %{id:03x}%{color:reset}`
)

// Logger provides a basic functionality for any logger implementation
type Logger interface {
	Debug(args ...interface{})
	Debugf(template string, args ...interface{})
	Info(args ...interface{})
	Infof(template string, args ...interface{})
	Warning(args ...interface{})
	Warningf(template string, args ...interface{})
	Error(args ...interface{})
	Errorf(template string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(template string, args ...interface{})
}

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

func makeLogger(id uint32, who string, opts options) Logger {
	logger := logging.MustGetLogger(module)
	logFormatString := fmt.Sprintf("%s %s %d: %%{message}", defaultLogPrefix, who, id)
	stringFormatter := logging.MustStringFormatter(logFormatString)
	backend := logging.NewLogBackend(opts.logFile, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, stringFormatter)
	formattedLoggerBackend := logging.AddModuleLevel(backendFormatter)

	logger.SetBackend(formattedLoggerBackend)

	formattedLoggerBackend.SetLevel(opts.logLevel, module)

	return logger
}

// NewReplicaLogger creates new replica's logger
func NewReplicaLogger(id uint32, opts ...Option) Logger {
	return makeLogger(id, "Replica", newOptions(opts...))
}

// NewClientLogger creates new client's logger
func NewClientLogger(id uint32, opts ...Option) Logger {
	return makeLogger(id, "Client", newOptions(opts...))
}
