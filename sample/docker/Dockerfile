FROM ubuntu:20.04 AS build
COPY --from=golang:1.15-buster /usr/local/go /usr/local/go/
ENV PATH /usr/local/go/bin:$PATH
RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates \
		curl \
		pkg-config
COPY . minbft/
ENV SGX_MODE SIM
RUN cd minbft \
	&& ./tools/install-sgx-sdk.sh \
	&& . /opt/intel/sgxsdk/environment \
	&& make prefix=/opt/minbft install

FROM ubuntu:20.04
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
