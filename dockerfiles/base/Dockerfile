# Copyright (c) 2018 NEC Laboratories Europe GmbH.
#
# Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM ubuntu:xenial

# URL to download Intel SGX SDK installer from
ARG sgx_sdk_url="https://download.01.org/intel-sgx/linux-2.1.1/ubuntu64-server/sgx_linux_x64_sdk_2.1.101.42529.bin"
# Golang version to install
ARG golang_ver=1.9.4

# Install required packages
RUN apt-get update -q && \
    apt-get install -qy --no-install-recommends ca-certificates \
            build-essential python wget git make pkg-config && \
    rm -rf /var/cache/apt /var/lib/apt

# Install Intel SGX SDK
ENV SGX_SDK=/opt/intel/sgxsdk
RUN set -eux; \
    wget "${sgx_sdk_url}" -O sgx_installer.bin; \
    chmod +x sgx_installer.bin; \
    echo -e "no\n$(dirname $SGX_SDK)" | \
    ./sgx_installer.bin; \
    rm sgx_installer.bin; \
    set +u; . $SGX_SDK/environment; set -u; \
    cp -r $SGX_SDK/SampleCode /tmp/SampleCode; \
    cd /tmp/SampleCode/LocalAttestation; \
    make SGX_MODE=SIM; \
    ./app; \
    rm -r /tmp/SampleCode
ENV PATH=$PATH:$SGX_SDK/bin:$SGX_SDK/bin/x64
ENV PKG_CONFIG_PATH=$PKG_CONFIG_PATH:$SGX_SDK/pkgconfig
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$SGX_SDK/sdk_libs

# Install Golang
RUN set -eux; \
    wget "https://golang.org/dl/go${golang_ver}.linux-amd64.tar.gz" -O golang.tar.gz; \
    tar -C /usr/local -xaf golang.tar.gz; \
    rm golang.tar.gz; \
    export PATH="/usr/local/go/bin:$PATH"; \
    go version
ENV GOPATH /opt/go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R +rwx "$GOPATH"
WORKDIR $GOPATH
