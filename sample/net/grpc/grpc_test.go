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

package grpc

import (
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hyperledger-labs/minbft/sample/net/grpc/connector"
	"github.com/hyperledger-labs/minbft/sample/net/grpc/proto"
	"github.com/hyperledger-labs/minbft/sample/net/grpc/server"
)

const (
	nrReplicas uint32 = 3
	nrMessages        = 5
)

func TestReplicaMessageStreamHandler(t *testing.T) {
	msgs := makeTestMessages(nrReplicas, nrMessages)
	servers, addrs := startLoopServers(nrReplicas)
	defer stopLoopServers(servers)

	replicaConnector := connector.New()
	err := replicaConnector.ConnectManyReplicas(addrs, grpc.WithInsecure(), grpc.WithBlock())
	require.NoError(t, err)

	inChannels := prepareInChannels(msgs)
	outChannels := startMessageStreams(t, replicaConnector, inChannels)

	_, err = replicaConnector.ReplicaMessageStreamHandler(nrReplicas)
	assert.Error(t, err, "ReplicaMessageStreamHandler must fail with unassigned replica ID")

	checkOutChannels(t, msgs, outChannels)
}

func TestReplicaServer(t *testing.T) {
	replica := new(loopReplica)
	addr := replica.start()
	defer replica.stop()

	msgs := makeTestMessages(1, nrMessages)
	replicaConnector := connector.New()
	err := replicaConnector.ConnectReplica(uint32(0), addr, grpc.WithInsecure(), grpc.WithBlock())
	require.NoError(t, err)

	inChannels := prepareInChannels(msgs)
	outChannels := startMessageStreams(t, replicaConnector, inChannels)

	checkOutChannels(t, msgs, outChannels)
}

func makeTestMessages(nrReplicas uint32, nrMessages int) (msgs map[uint32][]string) {
	msgs = make(map[uint32][]string)
	for i := uint32(0); i < nrReplicas; i++ {
		msgs[i] = make([]string, nrMessages)
		for j := range msgs[i] {
			msgs[i][j] = fmt.Sprintf("Message %d for replica %d", j, i)
		}
	}
	return
}

func prepareInChannels(msgs map[uint32][]string) (inChannels map[uint32]chan []byte) {
	inChannels = make(map[uint32]chan []byte)

	for i, msgList := range msgs {
		in := make(chan []byte, len(msgList))
		for _, msg := range msgList {
			in <- []byte(msg)
		}
		close(in)
		inChannels[i] = in
	}
	return
}

func checkOutChannels(t *testing.T, msgs map[uint32][]string, outChannels map[uint32]<-chan []byte) {
	for i, out := range outChannels {
		for _, msg := range msgs[i] {
			assert.Equal(t, msg, string(<-out))
		}
		_, more := <-out
		assert.False(t, more)
	}
}

func startMessageStreams(t *testing.T, replicaConnector *connector.ReplicaConnector, inChannels map[uint32]chan []byte) (outChannels map[uint32]<-chan []byte) {
	outChannels = make(map[uint32]<-chan []byte)

	for i, in := range inChannels {
		streamHandler, err := replicaConnector.ReplicaMessageStreamHandler(i)
		require.NoError(t, err)
		require.NotNil(t, streamHandler)

		out := streamHandler.HandleMessageStream(in)
		require.NotNil(t, out)
		outChannels[i] = out
	}
	return
}

func startLoopServers(nrReplicas uint32) (servers map[uint32]*loopChatServer, addrs map[uint32]string) {
	addrs = make(map[uint32]string)
	servers = make(map[uint32]*loopChatServer)
	for i := uint32(0); i < nrReplicas; i++ {
		servers[i] = &loopChatServer{}
		addrs[i] = servers[i].start()
	}
	return
}

func stopLoopServers(servers map[uint32]*loopChatServer) {
	for _, srv := range servers {
		srv.stop()
	}
}

type loopChatServer struct {
	grpcServer *grpc.Server
}

func (s *loopChatServer) Chat(stream proto.Channel_ChatServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}

func (s *loopChatServer) start() (addr string) {
	s.grpcServer, addr = startServer(s)
	return
}

func (s *loopChatServer) stop() {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
		s.grpcServer = nil
	}
}

type loopReplica struct {
	grpcServer *grpc.Server
}

func (r *loopReplica) HandleMessageStream(in <-chan []byte) <-chan []byte {
	// Buffer a couple of messages to create a bit of concurrency
	out := make(chan []byte, 2)
	go func() {
		for msg := range in {
			out <- msg
		}
		close(out)
	}()
	return out
}

func (r *loopReplica) start() (addr string) {
	r.grpcServer, addr = startServer(server.New(r))
	return
}

func (r *loopReplica) stop() {
	if r.grpcServer != nil {
		r.grpcServer.Stop()
		r.grpcServer = nil
	}
}

func startServer(channelServer proto.ChannelServer) (grpcServer *grpc.Server, addr string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr = lis.Addr().String()
	grpcServer = grpc.NewServer()
	proto.RegisterChannelServer(grpcServer, channelServer)
	go func() {
		err := grpcServer.Serve(lis)
		if err != nil {
			panic(err)
		}
	}()
	return
}
