FROM circleci/buildpack-deps:bionic-curl

RUN sudo apt-get update \
  && sudo apt-get install -y \
    pkg-config

RUN GO_URL="https://dl.google.com/go/go1.10.4.linux-amd64.tar.gz" \
  && curl $GO_URL | sudo tar -C /usr/local -xzf - \
  && echo 'export GOPATH="$HOME/go"' | sudo tee -a /etc/profile > /dev/null \
  && echo 'export PATH="/usr/local/go/bin:$PATH"' | sudo tee -a /etc/profile > /dev/null

RUN SGX_SDK_URL="https://download.01.org/intel-sgx/linux-2.3.1/ubuntu18.04/sgx_linux_x64_sdk_2.3.101.46683.bin" \
  && sudo apt-get install -y \
    build-essential \
    python \
  && curl $SGX_SDK_URL -o /tmp/sgx_linux_x64_sdk.bin \
  && chmod +x /tmp/sgx_linux_x64_sdk.bin \
  && echo -e "no\n/opt/intel" | sudo /tmp/sgx_linux_x64_sdk.bin \
  && rm -f /tmp/sgx_linux_x64_sdk.bin \
  && echo 'source /opt/intel/sgxsdk/environment' | sudo tee -a /etc/profile > /dev/null

RUN GOMETALINTER_URL="https://github.com/alecthomas/gometalinter/releases/download/v2.0.11/gometalinter-2.0.11-linux-amd64.tar.gz" \
  && curl -L $GOMETALINTER_URL | sudo tar -C /opt -xzf - \
  && echo 'export PATH="/opt/gometalinter-2.0.11-linux-amd64:$PATH"' | sudo tee -a /etc/profile > /dev/null

RUN echo 'export PATH="$GOPATH/bin:$PATH"' | sudo tee -a /etc/profile > /dev/null
