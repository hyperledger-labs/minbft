// Copyright © 2018 NEC Laboratories Europe GmbH.
//
// Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
//
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

	"github.com/hyperledger-labs/minbft/api"
	"github.com/hyperledger-labs/minbft/client"
	authen "github.com/hyperledger-labs/minbft/sample/authentication"
	"github.com/hyperledger-labs/minbft/sample/config"
	"github.com/hyperledger-labs/minbft/sample/conn/grpc/connector"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

// requestCmd represents the request command
var requestCmd = &cobra.Command{
	Use:   "request [request]",
	Short: "Submit request to replicas",
	Long: `
Submit a request to the consensus network, wait for it to be processed
and output the result. The request is a simple text string.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := []byte(strings.Join(args, " "))

		res, err := request(req)
		if err != nil {
			return err
		}

		fmt.Println("Reply:", string(res))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(requestCmd)

	requestCmd.Flags().Int("id", 0, "ID of the client")
	must(viper.BindPFlag("client.id",
		requestCmd.Flags().Lookup("id")))
}

type clientStack struct {
	api.Authenticator
	api.ReplicaConnector
}

func request(req []byte) ([]byte, error) {
	id := uint32(viper.GetInt("client.id"))

	keysFile, err := os.Open(viper.GetString("keys"))
	if err != nil {
		return nil, fmt.Errorf("Failed to open keyset file: %s", err)
	}

	auth, err := authen.New([]api.AuthenticationRole{api.ClientAuthen}, id, keysFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to create authenticator: %s", err)
	}

	cfg := config.New()
	cfg.LoadConfig(viper.GetString("consensusConf"))

	peerAddrs := make(map[uint32]string)
	for _, p := range cfg.Peers() {
		peerAddrs[uint32(p.ID)] = p.Addr
	}

	conn := connector.New()

	// XXX: The connection destination should be authenticated;
	// grpc.WithInsecure() option is passed here for simplicity.
	err = connector.ConnectManyReplicas(conn, peerAddrs, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to peers: %s", err)
	}

	client, err := client.New(id, cfg.N(), cfg.F(), clientStack{auth, conn})
	if err != nil {
		return nil, fmt.Errorf("Failed to create client instance: %s", err)
	}

	return <-client.Request(req), nil
}
