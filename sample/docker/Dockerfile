FROM ubuntu:18.04 AS build
COPY --from=golang:1.14.4-buster /usr/local/go /usr/local/go/
ENV PATH /usr/local/go/bin:$PATH
ENV SGX_SDK_URL https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/ubuntu18.04-server/sgx_linux_x64_sdk_2.9.101.2.bin
RUN apt-get update && apt-get install -y --no-install-recommends \
		build-essential \
		ca-certificates \
		pkg-config \
		python \
		wget \
	&& rm -rf /var/lib/apt/lists/* \
	&& wget -q -O sgx_linux_x64_sdk.bin $SGX_SDK_URL \
	&& chmod +x sgx_linux_x64_sdk.bin \
	&& echo -e "no\n/opt/intel" | ./sgx_linux_x64_sdk.bin
COPY . minbft/
ENV SGX_MODE SIM
RUN cd minbft \
	&& . /opt/intel/sgxsdk/environment \
	&& make prefix=/opt/minbft install

FROM ubuntu:18.04
RUN apt-get update && apt-get install -y --no-install-recommends \
		libssl1.1 \
	&& rm -rf /var/lib/apt/lists/*
COPY --from=build /opt/intel/sgxsdk /opt/intel/sgxsdk/
WORKDIR /opt/minbft
COPY --from=build /opt/minbft ./
ENV PATH /opt/minbft/bin:$PATH
ENV LD_LIBRARY_PATH /opt/minbft/lib:$LD_LIBRARY_PATH
COPY sample/config/consensus.yaml sample/peer/peer.yaml ./
RUN sed -i 's/:800\([0-2]\)/replica\1:8000/' consensus.yaml
COPY sample/docker/docker-entrypoint.sh /usr/local/bin/
VOLUME ["/data"]
EXPOSE 8000
ENTRYPOINT ["docker-entrypoint.sh"]
