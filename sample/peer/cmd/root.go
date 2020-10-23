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

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperledger-labs/minbft/common/logger"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

const (
	name         = "peer"
	cfgEnvPrefix = name
	defCfgFile   = name + ".yaml"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   name,
	Short: "Application to run MinBFT peer",
	Long: `
This application is a sample implementation of CLI showing how to use
a MinBFT core package.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config",
		defCfgFile, "config file")
	must(viper.BindPFlag("config",
		rootCmd.PersistentFlags().Lookup("config")))

	rootCmd.PersistentFlags().String("consensus-conf",
		defConsensusCfgFile, "consensus configuration file")
	must(viper.BindPFlag("consensusConf",
		rootCmd.PersistentFlags().Lookup("consensus-conf")))

	rootCmd.PersistentFlags().String("keys", defKeysFile, "keyset file")
	must(viper.BindPFlag("keys",
		rootCmd.PersistentFlags().Lookup("keys")))

	rootCmd.PersistentFlags().String("logging-level", "", "logging level")
	must(viper.BindPFlag("logging.level",
		rootCmd.PersistentFlags().Lookup("logging-level")))

	rootCmd.PersistentFlags().String("logging-file", "", "logging file")
	must(viper.BindPFlag("logging.file",
		rootCmd.PersistentFlags().Lookup("logging-file")))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetEnvPrefix(cfgEnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetConfigFile(cfgFile)
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func getLoggingOptions() ([]logger.Option, error) {
	opts := []logger.Option{}

	if viper.GetString("logging.level") != "" {
		logLevel, err := logging.LogLevel(viper.GetString("logging.level"))
		if err != nil {
			return nil, fmt.Errorf("failed to set logging level: %s", err)
		}
		opts = append(opts, logger.WithLogLevel(logLevel))
	}

	if viper.GetString("logging.file") != "" {
		logFile, err := os.OpenFile(viper.GetString("logging.file"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open logging file: %s", err)
		}
		opts = append(opts, logger.WithLogFile(logFile))
	}

	return opts, nil
}
