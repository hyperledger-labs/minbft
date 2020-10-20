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
	"strconv"

	"github.com/a8m/envsubst"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	authen "github.com/hyperledger-labs/minbft/sample/authentication"
)

const (
	defNrReplicas     = 3
	defReplicaKeySpec = "ECDSA"

	defReplicaSecParam = 256
	defNrClients       = 1
	defClientKeySpec   = "ECDSA"
	defClientSecParam  = 256

	defUsigEnclaveFile = "libusig.signed.so"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate [numberReplicas [numberClients [usigEnclaveFile]]]",
	Short: "Generate sample keystore file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			nrReplicas, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse number of replicas "+
					"from positional argument: %s", err)
			}
			viper.Set("replicas.number", nrReplicas)
		}
		if len(args) > 1 {
			nrClients, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("failed to parse number of clients "+
					"from positional argument: %s", err)
			}
			viper.Set("clients.number", nrClients)
		}
		if len(args) > 2 {
			viper.Set("usig.enclaveFile", args[2])
		}

		return generateKeys()
	},
	Args: cobra.MaximumNArgs(3),
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().IntP("num-replicas", "r",
		defNrReplicas, "number of replicas")
	must(viper.BindPFlag("replicas.number",
		generateCmd.Flags().Lookup("num-replicas")))

	generateCmd.Flags().String("replica-key-spec",
		defReplicaKeySpec, "keyspec for replica")
	must(viper.BindPFlag("replicas.keyspec",
		generateCmd.Flags().Lookup("replica-key-spec")))

	generateCmd.Flags().Int("replica-sec-param",
		defReplicaSecParam, "replica security param")
	must(viper.BindPFlag("replicas.secparam",
		generateCmd.Flags().Lookup("replica-sec-param")))

	generateCmd.Flags().IntP("num-clients", "c",
		defNrClients, "number of clients")
	must(viper.BindPFlag("clients.number",
		generateCmd.Flags().Lookup("num-clients")))

	generateCmd.Flags().String("client-key-spec",
		defClientKeySpec, "keyspec for client")
	must(viper.BindPFlag("clients.keyspec",
		generateCmd.Flags().Lookup("client-key-spec")))

	generateCmd.Flags().Int("client-sec-param",
		defClientSecParam, "client security param")
	must(viper.BindPFlag("clients.secparam",
		generateCmd.Flags().Lookup("client-sec-param")))

	generateCmd.Flags().StringP("usig-enclave-file", "u",
		defUsigEnclaveFile, "USIG enclave file")
	must(viper.BindPFlag("usig.enclaveFile",
		generateCmd.Flags().Lookup("usig-enclave-file")))
}

func generateKeys() error {
	usigEnclaveFile, err := envsubst.String(viper.GetString("usig.enclaveFile"))
	if err != nil {
		return fmt.Errorf("failed to parse USIG enclave filename: %s", err)
	}
	opts := &authen.TestnetKeyOpts{
		NumberReplicas:  viper.GetInt("replicas.number"),
		ReplicaKeySpec:  viper.GetString("replicas.keyspec"),
		ReplicaSecParam: viper.GetInt("replicas.secparam"),

		NumberClients:  viper.GetInt("clients.number"),
		ClientKeySpec:  viper.GetString("clients.keyspec"),
		ClientSecParam: viper.GetInt("clients.secparam"),

		UsigEnclaveFile: usigEnclaveFile,
	}

	outFileName := viper.GetString("output")
	fmt.Println("Using output file:", outFileName)

	out, err := os.Create(outFileName)
	if err != nil {
		return fmt.Errorf("failed to open output file: %s", err)
	}

	if err := authen.GenerateTestnetKeys(out, opts); err != nil {
		return fmt.Errorf("failed to generate keys: %s", err)
	}

	return nil
}
