CC = gcc
SGX_SDK ?= "/opt/intel/sgxsdk"

check-sgx-capability: check-sgx-capability.c
	$(CC) -I$(SGX_SDK)/include -L$(SGX_SDK)/lib64 -o $@ $? -lsgx_capable

check: check-sgx-capability
	LD_LIBRARY_PATH=$(SGX_SDK)/lib64:$(LD_LIBRARY_PATH) ./check-sgx-capability

clean:
	rm -f check-sgx-capability
