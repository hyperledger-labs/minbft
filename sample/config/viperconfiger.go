// Copyright (c) 2018 NEC Laboratories Europe GmbH.
//
// Authors: Wenting Li <wenting.li@neclab.eu>
//          Sergey Fedorov <sergey.fedorov@neclab.eu>
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

package config

import (
	"io"
	"log"
	"math"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// ViperConfiger implements Configer through viper
type ViperConfiger struct {
	config *viper.Viper
}

// New creates a new instance of viper configer
func New() *ViperConfiger {
	return &ViperConfiger{}
}

// Peer represents an entry in the list of peers in configuration
type Peer struct {
	ID   int
	Addr string
}

// LoadConfig parses the config file
//
// (Example config file template)
//
//  protocol:
//      "n": 3
//      f: 1
//      checkpointPeriod: 10
//      logsize: 20
//
//      timeout:
//          request: 2s
//          prepare: 1s
//          viewchange: 3s
//  peers:
//      - id: 0
//        addr: ":8000"
//      - id: 1
//        addr: ":8001"
//      - id: 2
//        addr: ":8002"
//
func (c *ViperConfiger) LoadConfig(filePath string) {
	confPath, confName := filepath.Split(filePath)
	ext := filepath.Ext(confName)
	name := confName[:len(confName)-len(ext)]

	if len(ext) < 2 || len(name) == 0 {
		panic("Invalid config file name (should specify file extension for parsing).")
	}

	config := viper.New()
	config.SetConfigName(name)
	config.SetConfigType(ext[1:]) // exclude the char '.'
	config.AddConfigPath(confPath)
	config.AddConfigPath("./") // support relative path
	err := config.ReadInConfig()
	if err != nil {
		panic("Failed to parse config file: " + err.Error())
	}

	c.config = config
}

// ReadConfig reads configuration from the specified io.Reader parsing
// it according to the specified type supported by
// github.com/spf13/viper package, e.g. "yaml"
func (c *ViperConfiger) ReadConfig(in io.Reader, cfgType string) error {
	config := viper.New()
	config.SetConfigType(cfgType)
	err := config.ReadConfig(in)
	if err != nil {
		return err
	}
	c.config = config
	return nil
}

// IsInitialized returns true if the config file is correctly parsed
func (c *ViperConfiger) IsInitialized() bool {
	return nil != c.config
}

//============ Helpers ============

// getUint32 checks the boundary of the config value and returns uint32
func (c *ViperConfiger) getUint32(key string) uint32 {
	n := c.config.GetInt64(key)
	if n < int64(0) || n > math.MaxUint32 {
		panic("interger overflow: " + key)
	}
	return uint32(n)
}

// getTimeDuration returns time duration of the config value
func (c *ViperConfiger) getTimeDuration(key string) time.Duration {
	return c.config.GetDuration(key)
}

//============ Configer Interface =============

// N returns the total number of replicas
func (c *ViperConfiger) N() uint32 {
	return c.getUint32("protocol.n")
}

// F returns the number of tolerated faulty replicas
func (c *ViperConfiger) F() uint32 {
	return c.getUint32("protocol.f")
}

// CheckpointPeriod returns the checkpoint period
func (c *ViperConfiger) CheckpointPeriod() uint32 {
	return c.getUint32("protocol.checkpointPeriod")
}

// Logsize returns the maximum size of the log
func (c *ViperConfiger) Logsize() uint32 {
	return c.getUint32("protocol.logsize")
}

// TimeoutRequest returns the per-request timeout for view change
func (c *ViperConfiger) TimeoutRequest() time.Duration {
	return c.getTimeDuration("protocol.timeout.request")
}

// TimeoutPrepare returns the timeout to forward REQUEST message
func (c *ViperConfiger) TimeoutPrepare() time.Duration {
	return c.getTimeDuration("protocol.timeout.prepare")
}

// TimeoutViewChange returns the timeout to receive NEW-VIEW message
func (c *ViperConfiger) TimeoutViewChange() time.Duration {
	return c.getTimeDuration("protocol.timeout.viewchange")
}

// Peers returns a list peers
func (c *ViperConfiger) Peers() []Peer {
	peers := []Peer{}
	err := c.config.UnmarshalKey("peers", &peers)
	if err != nil {
		log.Print("Failed to unmarshal peers:", err)
		return nil
	}
	return peers
}
