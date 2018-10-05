[![CircleCI](https://circleci.com/gh/hyperledger-labs/minbft/tree/master.svg?style=svg)](https://circleci.com/gh/hyperledger-labs/minbft/tree/master)
[![GoDoc](https://godoc.org/github.com/hyperledger-labs/minbft?status.svg)](https://godoc.org/github.com/hyperledger-labs/minbft)
[![Go Report Card](https://goreportcard.com/badge/github.com/hyperledger-labs/minbft)](https://goreportcard.com/report/github.com/hyperledger-labs/minbft)

# MinBFT #

  * [Status](#status)
  * [What is MinBFT](#what-is-minbft)
  * [Why MinBFT](#why-minbft)
  * [Concepts](#concepts)
  * [Requirements](#requirements)
      * [Operating System](#operating-system)
      * [Golang](#golang)
      * [Intel® SGX SDK](#intel-sgx-sdk)
  * [Getting Started](#getting-started)
      * [Building](#building)
      * [Running Example](#running-example)
      * [Code Structure](#code-structure)
  * [Roadmap](#roadmap)
  * [Contributing](#contributing)
  * [License](#license)

## Status ##

This project is in experimental development stage. It is **not**
suitable for any kind of production use. Interfaces and implementation
may change significantly during the life cycle of the project.

## What is MinBFT ##

MinBFT is a pluggable software component that allows to achieve
Byzantine fault-tolerant consensus with fewer consenting nodes and
less communication rounds comparing to the conventional BFT protocols.
The component is implemented based
on [Efficient Byzantine Fault-Tolerance paper][minbft-paper], a BFT
protocol that leverages secure hardware capabilities of the
participant nodes.

This project is an implementation of MinBFT consensus protocol. The
code is written in Golang, except the TEE part which is written in C,
as an Intel® SGX enclave.

[minbft-paper]: https://www.researchgate.net/publication/260585535_Efficient_Byzantine_Fault-Tolerance

## Why MinBFT ##

Byzantine fault-tolerant (BFT) protocols are able to achieve high
transaction throughput in permission-based consensus networks with
static configuration of connected nodes. However, existing components
such as Practical Byzantine Fault Tolerance (PBFT) consensus plugin
still incur high communication cost. Given the pervasive Trusted
Execution Environments (TEE) on commodity platforms, we see a strong
motivation to deploy more efficient BFT protocols that leverage the
TEE applications as trust anchors even on faulty nodes.

TEEs are pervasive nowadays and supported by many commodity platforms.
For instance, Intel® SGX is being deployed on many PCs and servers,
while mobile platforms are mostly supported by Arm Trustzone. TEEs
rely on secure hardware to provide data protection and code isolation
from a compromised hosting system. Here, we propose such a consensus
component that implements MinBFT, a BFT protocol that leverages TEEs
to prevent message equivocation, and thus reducing the required number
of consenting nodes as well as communication rounds given the same
fault-tolerance threshold. More specifically, it requires only `2f+1`
consenting nodes in order to tolerate `f` faulty nodes (or tolerate up
to half of faulty nodes); meanwhile, committing a message requires
only 2 rounds of communication among the nodes instead of 3 rounds as
in PBFT.

We hope that by evaluating this consensus component under the existing
blockchain frameworks, the community will benefit from availability to
leverage it in different practical use cases.

## Concepts ##

The consensus process in MinBFT is very similar to PBFT. Consenting
nodes (i.e., nodes who vote for ordering the transactions) compose a
fully-connected network. There is a leader (often referred to as
primary node) among the nodes who first prepares an incoming request
message to the other nodes by suggesting a sequence number for the
request in broadcasted `PREPARE` messages. The other nodes verify the
`PREPARE` messages and subsequently broadcast a `COMMIT` message in
the network. Finally, the nodes who have received `f+1` (where `f` is
fault-tolerance, a number of tolerated faulty nodes) consistent
`COMMIT` messages execute the request locally and update the
underlying service state. If the leader is perceived as faulty, a view
change procedure follows to change the leader node.

Note that the signatures for each `PREPARE` and `COMMIT` messages are
generated by USIG (Unique Sequential Identifier Generator) service,
the tamper-proof part of the consenting nodes. The sequence numbers
are also assigned to the messages by USIG with the help of a unique,
monotonic, and sequential counter protected by TEE. The signature,
also known as USIG certificate, certifies the counter assigned to a
particular message. The USIG certificate combined with the counter
value comprises a UI (unique identifier) of the message. Since the
monotonic counter prevents a faulty node from sending conflicting
messages to different nodes and provides correct information during
view changes, MinBFT requires less communication rounds and total
number of consenting nodes than PBFT.

For more detailed description of the protocol, refer to [Efficient
Byzantine Fault-Tolerance paper][minbft-paper].

## Requirements ##

### Operating System ###

The project has been tested on Ubuntu 16.04.4 LTS (Xenial Xerus).
Additional required packages can be installed as follows:

```sh
sudo apt-get install build-essential pkg-config
```

### Golang ###

`go1.10` is used to build this project. For installation instructions
please visit [this page](https://golang.org/doc/install). Please make
sure to export `GOPATH` environment variable, e.g. by adding the
following line to `~/.profile`:

```sh
export GOPATH="$HOME/go"
```

### Intel® SGX SDK ###

The Intel® SGX enclave implementation has been tested with Intel® SGX
SDK for Linux version 2.3.1. For installation instuctions please visit
[installation guide][sgx-install]. A conventional directory to install
the SDK is `/opt/intel/`. Please do not forget to source
`/opt/intel/sgxsdk/environment` file in your shell. Alternatively, one
can add the following line to `~/.profile`:

```sh
. /opt/intel/sgxsdk/environment
```

and create `/etc/ld.so.conf.d/sgx-sdk.conf` file with the following
content:

```
/opt/intel/sgxsdk/sdk_libs
```

When using a machine with no SGX support, only SGX simulation mode is
supported. In that case, please be sure to export the following
environment variable, e.g. by modifying `~/.profile` file:

```sh
export SGX_MODE=SIM
```

[sgx-install]: https://01.org/intel-softwareguard-extensions/documentation/intel-software-guard-extensions-installation-guide

## Getting Started ##

The project source code should be placed in
`$GOPATH/src/github.com/hyperledger-labs/minbft/` directory. All
following commands are supposed to be run from that directory.

### Building ###

The project can be build by issuing the following command:

```sh
make build
```

The binaries produced are placed under `sample/bin/` directory.

### Running Example ###

Running the example requires some set up. Please make sure the project
has been successfully built and `sample/bin/keytool` and
`sample/bin/peer` binaries were produced. Those binaries can be
supplied with options through a configuration file, environment
variables, or command-line arguments. More information about available
options can be queried by invoking those binaries with `help`
argument. Sample configuration files can be found in
`sample/authentication/keytool/` and `sample/peer/` directories
respectively.

#### Generating Keys ####

The following command are to be run from `sample` directory.

```sh
cd sample
```

Sample key set file for testing can be generate by using `keytool`
command. This command produces a key set file suitable for running the
example on a local machine:

```sh
bin/keytool generate
```

This invocation will create a sample key set file named `keys.yaml`
containing 3 key pairs for replicas and 1 key pair for a client by
default.

#### Consensus Options Configuration ####

Consensus options can be set up by means of a configuration file. A
sample consensus configuration file can be used as an example:

```sh
cp config/consensus.yaml ./
```

#### Peer Configuration ####

Peer configuration can be supplied in a configuration file. Selected
options can be modified through command line arguments of `peer`
binary. A sample configuration can be used as an example:

```sh
cp peer/peer.yaml ./
```

#### Running Replicas ####

To start up an example consensus network of replicas on a local
machine, invoke the following commands:

```sh
bin/peer run 0 &
bin/peer run 1 &
bin/peer run 2 &
```

This will start the replica nodes as 3 separate OS processes in
background using the configuration files prepared in previous steps.

#### Submitting Requests ####

Requests can be submitted for ordering and execution to the example
consensus network using the same `peer` binary and configuration files
for convenience. It is better to issue the following commands in
another terminal so that the output messages do not intermix:

```sh
bin/peer request First request
bin/peer request Second request
bin/peer request Another request
```

These commands should produce the following output showing the result
of ordering and execution of the submitted requests:

```
Reply: {"Height":1,"PrevBlockHash":null,"Payload":"Rmlyc3QgcmVxdWVzdA=="}
Reply: {"Height":2,"PrevBlockHash":"DuAGbE1hVQCvgi+R0E5zWaKSlVYFEo3CjlRj9Eik5h4=","Payload":"U2Vjb25kIHJlcXVlc3Q="}
Reply: {"Height":3,"PrevBlockHash":"963Kn659GbtX35MZYzguEwSH1UvF2cRYo6lNpIyuCUE=","Payload":"QW5vdGhlciByZXF1ZXN0"}
```

The output shows the submitted requests being ordered and executed by
a sample blockchain service. The service executes request by simply
appending a new block for each request to the trivial blockchain
maintained by the service.

#### Tear Down ####

The following command can be used to terminate running replica
processes and release the occupied TCP ports:

```sh
killall peer
```

### Code Structure ###

The code divided into core consensus protocol implementation and
sample implementation of external components required to interact with
the core. The following directories contain the code:

  * `api/` - definition of API between core and external components
  * `client/` - implementation of client-side part of the protocol
  * `core/` - implementation of core consensus protocol
  * `usig/` - implementation of USIG, tamper-proof component
  * `messages/` - definition of the protocol messages
  * `sample/` - sample implementation of external interfaces
    * `authentication/` - generation and verification of
                          authentication tags
      * `keytool/` - tool to generate sample key set file
    * `net/` - network connectivity
    * `config/` - consensus configuration provider
    * `requestconsumer/` - service executing ordered requests
    * `peer/` - CLI application to run a replica/client instance

## Roadmap ##

The following features of MinBFT protocol has been implemented:

  * _Normal case operation_: minimal ordering and execution of
    requests as long as primary replica is not faulty
  * _SGX USIG_: implementation of USIG service as Intel® SGX enclave

The following features are considered to be implemented:

  * _View change operation_: provide liveness in case of faulty
    primary replica
  * _Garbage collection and checkpoints_: generation and handling of
    `CHECKPOINT` messages, log pruning, high and low water marks
  * _USIG enclave attestation_: support to remotely attest USIG
    Intel® SGX enclave
  * _Faulty node recovery_: support to retrieve missing log entries
    and synchronize service state from other replicas
  * _Request batching_: reducing latency and increasing throughput by
    combining outstanding requests for later processing
  * _Asynchronous requests_: enabling parallel processing of requests
  * _MAC authentication_: using MAC in place of digital signature in
    USIG to reduce message size
  * _Read-only requests_: optimized processing of read-only requests
  * _Speculative request execution_: reducing processing delay by
    tentatively executing requests
  * _Documentation improvement_: comprehensive documentation
  * _Testing improvement_: comprehensive unit- and integration tests
  * _Benchmarks_: measuring performance

## Contributing ##

Any kind of feedback is highly appreciated. Questions, suggestions,
bug reports, change requests etc are welcome. Please [file a GitHub
issue](https://github.com/hyperledger-labs/minbft/issues/new) or [submit
a pull request](https://github.com/hyperledger-labs/minbft/compare), as
appropriate.

## License ##

Source code files are licensed under
the [Apache License, Version 2.0](LICENSE).

Documentation files are licensed under
the [Creative Commons Attribution 4.0 International License][cc-40].

[cc-40]: http://creativecommons.org/licenses/by/4.0/
