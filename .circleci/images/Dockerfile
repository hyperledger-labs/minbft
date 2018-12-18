FROM circleci/buildpack-deps:bionic-curl

RUN sudo apt-get update \
  && sudo apt-get install -y \
    pkg-config

RUN GO_URL="https://dl.google.com/go/go1.10.4.linux-amd64.tar.gz" \
  && curl $GO_URL | sudo tar -C /usr/local -xzf -
COPY go.sh /etc/profile.d/

RUN SGX_SDK_URL="https://download.01.org/intel-sgx/linux-2.3.1/ubuntu18.04/sgx_linux_x64_sdk_2.3.101.46683.bin" \
  && sudo apt-get install -y \
    build-essential \
    python \
  && curl $SGX_SDK_URL -o /tmp/sgx_linux_x64_sdk.bin \
  && chmod +x /tmp/sgx_linux_x64_sdk.bin \
  && echo -e "no\n/opt/intel" | sudo /tmp/sgx_linux_x64_sdk.bin \
  && rm -f /tmp/sgx_linux_x64_sdk.bin \
  && sudo ln -s /opt/intel/sgxsdk/environment /etc/profile.d/intel-sgxsdk.sh

COPY install_latest_gometalinter.sh /tmp/install_latest_gometalinter.sh
RUN sudo apt-get install -y jq \
  && bash /tmp/install_latest_gometalinter.sh

COPY gometalinter.sh /etc/profile.d
