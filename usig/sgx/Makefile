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

.DEFAULT_GOAL := all
.PHONY: all build enclave untrusted check test clean help

all: build check
build: enclave untrusted
test: check

help:
	@echo 'Usage: make [target] [variables]'
	@echo ''
	@echo 'Generic targets:'
	@echo '  all (default)       - Build and test all'
	@echo '  build               - Build all'
	@echo '  enclave             - Build enclave'
	@echo '  untrusted           - Build untrusted code'
	@echo '  check|test          - Run all tests'
	@echo '  clean               - Remove all build artifacts'
	@echo ''
	@echo 'Variables'
	@echo '  SGX_MODE=HW|SIM     - Build mode: hardware or simulation'
	@echo '  SGX_DEBUG=0|1       - Debug mode: disabled or enabled'
	@echo '  SGX_PRERELEASE=0|1  - Pre-release mode: disabled or enabled'
	@echo '  V=0|1               - Verbose build: disabled or enabled'
	@echo ''
	@echo 'Intel(R) SGX SDK convention for build configuraitons is followed:'
	@echo '  - Hardware Mode, Debug build:'
	@echo '      make'
	@echo '  - Hardware Mode, Pre-release build:'
	@echo '      make SGX_PRERELEASE=1 SGX_DEBUG=0'
	@echo '  - Hardware Mode, Release build:'
	@echo '      make SGX_DEBUG=0'
	@echo '  - Simulation Mode, Debug build:'
	@echo '      make SGX_MODE=SIM'
	@echo '  - Simulation Mode, Pre-release build:'
	@echo '      make SGX_MODE=SIM SGX_PRERELEASE=1 SGX_DEBUG=0'
	@echo '  - Simulation Mode, Release build:'
	@echo '      make SGX_MODE=SIM SGX_DEBUG=0'

# Build mode
#   HW  - Hardware mode (default)
#   SIM - Simulation mode
# NB: Must be consistent with `make help` output
SGX_MODE ?= HW

# Three configuration modes - Debug, Pre-release, Release
#   Debug       - Macro DEBUG enabled (default)
#   Pre-release - Macro NDEBUG and EDEBUG enabled
#   Release     - Macro NDEBUG enabled
# NB: Must be consistent with `make help` output
SGX_DEBUG ?= 1
ifeq ($(SGX_DEBUG), 1)
  DEBUG_CPPFLAGS        := -DDEBUG -UNDEBUG -UEDEBUG
  ENCLAVE_DISABLE_DEBUG := 0
else ifeq ($(SGX_PRERELEASE), 1)
  DEBUG_CPPFLAGS        := -DNDEBUG -DEDEBUG -UDEBUG
  ENCLAVE_DISABLE_DEBUG := 0
else
  DEBUG_CPPFLAGS        := -DNDEBUG -UEDEBUG -UDEBUG
  ENCLAVE_DISABLE_DEBUG := 1
endif

ifeq ($(SGX_SDK),)
  SGX_SDK := /opt/intel/sgxsdk
  $(warning "SGX_SDK is unset; guessing: $(SGX_SDK) ...")
endif

# Verbose mode
#   V=1 - print all the command executed
#   V=0 - print only abbreviation of executed commands and suppress
#         output from some commands
V ?= 0
ifeq ($(V),0)
  # Print only an abbreviation of the command executed in a receipt.
  # Usage: $(call abbrev-cmd, COMMAND, ABBREVIATION)
  abbrev-cmd = $(if $(2), \
                    @echo $(2) && $(1), \
                    @$(1))
  # Inhibit command output, optionally error output as well
  # Usage: $(call silent-cmd, COMMAND, [0|1])
  # The second parameter specifies if error output should be inhibited.
  silent-cmd = $(1) >/dev/null $(if $(2), 2>&1)
else
  abbrev-cmd = $(1)
  silent-cmd = $(1)
endif

ifeq ($(SGX_MODE), HW)
  SGX_CONFIG := hw
else
  SGX_CONFIG := sim
endif
ifeq ($(SGX_PRERELEASE), 1)
  SGX_CONFIG := $(SGX_CONFIG)_prerelease
endif
ifeq ($(SGX_DEBUG), 1)
  SGX_CONFIG := $(SGX_CONFIG)_debug
endif
# Build configuration is captured in this file name
SGX_CONFIG_FILE := .config_$(SGX_CONFIG)

$(SGX_CONFIG_FILE):
ifneq ($(wildcard .config_*),)
	$(call abbrev-cmd, \
	       $(MAKE) --no-print-directory clean NO_RECONF=1 && \
	       rm -f .config_*)
endif
	$(call abbrev-cmd, \
	       touch $@, \
	       "CONF   $(SGX_CONFIG)")

ifneq ($(NO_RECONF),1)
  # Including an empty file doesn't harm, but forces make to execute a
  # rule targeting this file, i.e. clean up after configuration change
  -include $(SGX_CONFIG_FILE)
endif

SGX_INCLUDE_PATH := $(SGX_SDK)/include
SGX_LIBRARY_PATH := $(SGX_SDK)/lib64
SGX_BIN_PATH     := $(SGX_SDK)/bin/x64
SGX_SIGNER       := $(SGX_BIN_PATH)/sgx_sign
SGX_EDGER8R      := $(SGX_BIN_PATH)/sgx_edger8r

COMMON_CFLAGS   := -m64 -std=c99 -Wall -Wextra -Werror
COMMON_CPPFLAGS := $(DEBUG_CPPFLAGS) -I$(SGX_INCLUDE_PATH) -MMD -MP
COMMON_LDFLAGS  := -L$(SGX_LIBRARY_PATH)
COMMON_LDLIBS   :=

ifeq ($(SGX_DEBUG), 1)
  COMMON_CFLAGS += -O0 -ggdb
else
  COMMON_CFLAGS += -O2
endif

ifneq ($(SGX_MODE), HW)
  COMMON_CPPFLAGS  += -DSGX_SIM_MODE
  SGX_TRTS_LIB     := sgx_trts_sim
  SGX_TSERVICE_LIB := sgx_tservice_sim
else
  SGX_TRTS_LIB     := sgx_trts
  SGX_TSERVICE_LIB := sgx_tservice
endif

%.o: %.c
	$(call abbrev-cmd, \
	       $(CC) $(CPPFLAGS) $(CFLAGS) -c $< -o $@, \
	       "CC     $@")

ENCLAVE_NAME         := usig
ENCLAVE_TARGET       := enclave/lib$(ENCLAVE_NAME).so
ENCLAVE_SIGNED       := $(ENCLAVE_TARGET:.so=.signed.so)
ENCLAVE_LDS          := $(ENCLAVE_TARGET:.so=.lds)
ENCLAVE_EDL          := enclave/$(ENCLAVE_NAME).edl
ENCLAVE_CONFIG       := enclave/$(ENCLAVE_NAME).config.xml
ENCLAVE_KEY          := enclave/$(ENCLAVE_NAME).key
ENCLAVE_PUB          := enclave/$(ENCLAVE_NAME).pub
ENCLAVE_PROXY_SOURCE := enclave/$(ENCLAVE_NAME)_t.c
ENCLAVE_PROXY_HEADER := $(ENCLAVE_PROXY_SOURCE:.c=.h)

ENCLAVE_SOURCES := $(ENCLAVE_PROXY_SOURCE) enclave/usig.c
ENCLAVE_OBJECTS := $(ENCLAVE_SOURCES:.c=.o)
ENCLAVE_DEPS    := $(ENCLAVE_SOURCES:.c=.d)

ENCLAVE_ARTIFACTS := $(ENCLAVE_TARGET) $(ENCLAVE_SIGNED) $(ENCLAVE_CONFIG) \
                     $(ENCLAVE_PROXY_SOURCE) $(ENCLAVE_PROXY_HEADER) \
	             $(ENCLAVE_KEY) $(ENCLAVE_PUB) \
		     $(ENCLAVE_OBJECTS) $(ENCLAVE_DEPS)

ENCLAVE_CFLAGS   := $(COMMON_CFLAGS) -fvisibility=hidden -fpie -fstack-protector
ENCLAVE_CPPFLAGS := $(COMMON_CPPFLAGS) -nostdinc -I$(SGX_INCLUDE_PATH)/tlibc
ENCLAVE_LDFLAGS  := $(COMMON_LDFLAGS) -Wl,-pie -Wl,--no-undefined \
                    -nostdlib -nodefaultlibs -nostartfiles \
                    -Bstatic -Bsymbolic -Wl,--export-dynamic \
                    -Wl,--version-script=$(ENCLAVE_LDS) \
                    -Wl,-eenclave_entry -Wl,--defsym,__ImageBase=0 \
                    -Wl,-fuse-ld=gold -Wl,--rosegment
ENCLAVE_LDLIBS   := $(COMMON_LDLIBS) -Wl,--whole-archive \
                    -l$(SGX_TRTS_LIB) -Wl,--no-whole-archive \
                    -Wl,--start-group -lsgx_tstdc -lsgx_tcrypto \
                    -l$(SGX_TSERVICE_LIB) -Wl,--end-group

-include $(ENCLAVE_DEPS)

$(ENCLAVE_OBJECTS): CFLAGS   := $(ENCLAVE_CFLAGS)
$(ENCLAVE_OBJECTS): CPPFLAGS := $(ENCLAVE_CPPFLAGS)

$(ENCLAVE_TARGET): LDFLAGS := $(ENCLAVE_LDFLAGS)
$(ENCLAVE_TARGET): LDLIBS  := $(ENCLAVE_LDLIBS)
$(ENCLAVE_TARGET): $(ENCLAVE_OBJECTS)
	$(call abbrev-cmd, \
	       $(CC) $(LDFLAGS) $^ -o $@ $(LDLIBS), \
	       "LINK   $@")

$(ENCLAVE_PROXY_SOURCE) $(ENCLAVE_PROXY_HEADER): $(ENCLAVE_EDL)
	$(call abbrev-cmd, \
	       $(SGX_EDGER8R) --trusted-dir $(@D) --trusted $<, \
	       "GEN    $@")

$(ENCLAVE_CONFIG): %.xml: %.xml.in
	$(call abbrev-cmd, \
	       sed -e 's@\$$DISABLE_DEBUG@$(ENCLAVE_DISABLE_DEBUG)@' $< >$@, \
	       "GEN    $@")

lib%.signed.so: lib%.so %.key %.config.xml
	$(call abbrev-cmd, \
	       $(call silent-cmd, \
	              $(SGX_SIGNER) sign \
	                            -key $*.key \
	                            -enclave $< \
	                            -config $*.config.xml \
	                            -out $@, \
	              1), \
	       "GEN    $@")

$(ENCLAVE_KEY):
	$(call abbrev-cmd, \
	       $(call silent-cmd, \
	              openssl genrsa -out $@ -3 3072, \
	              1), \
	       "GEN    $@")

$(ENCLAVE_PUB): %.pub: %.key
	$(call abbrev-cmd, \
	       $(call silent-cmd, \
	              openssl rsa -out $@ -in $< -pubout, \
	              1), \
	       "GEN    $@")

enclave: $(ENCLAVE_SIGNED)

UNTRUSTED_NAME         := usig_shim
UNTRUSTED_TARGET       := shim/lib$(UNTRUSTED_NAME).so
UNTRUSTED_PROXY_SOURCE := shim/$(ENCLAVE_NAME)_u.c
UNTRUSTED_PROXY_HEADER := $(UNTRUSTED_PROXY_SOURCE:.c=.h)

UNTRUSTED_SOURCES := $(UNTRUSTED_PROXY_SOURCE) shim/shim.c
UNTRUSTED_OBJECTS := $(UNTRUSTED_SOURCES:.c=.o)
UNTRUSTED_DEPS    := $(UNTRUSTED_SOURCES:.c=.d)

UNTRUSTED_ARTIFACTS := $(UNTRUSTED_TARGET) $(UNTRUSTED_OBJECTS) $(UNTRUSTED_DEPS) \
	               $(UNTRUSTED_PROXY_SOURCE) $(UNTRUSTED_PROXY_HEADER)


UNTRUSTED_CFLAGS   := $(COMMON_CFLAGS) -fPIC
UNTRUSTED_CPPFLAGS := $(COMMON_CPPFLAGS) -pthread
UNTRUSTED_LDFLAGS  := $(COMMON_LDFLAGS) -fPIC -pthread
UNTRUSTED_LDLIBS   := -lpthread

ifneq ($(SGX_MODE), HW)
  UNTRUSTED_LDLIBS += -lsgx_urts_sim
else
  UNTRUSTED_LDLIBS += -lsgx_urts
endif

-include $(UNTRUSTED_DEPS)

$(UNTRUSTED_OBJECTS): CFLAGS   := $(UNTRUSTED_CFLAGS)
$(UNTRUSTED_OBJECTS): CPPFLAGS := $(UNTRUSTED_CPPFLAGS)

$(UNTRUSTED_TARGET): LDFLAGS := $(UNTRUSTED_LDFLAGS) -shared
$(UNTRUSTED_TARGET): LDLIBS  := $(UNTRUSTED_LDLIBS)
$(UNTRUSTED_TARGET): $(UNTRUSTED_OBJECTS)
	$(call abbrev-cmd, \
	       $(CC) $(LDFLAGS) $^ -o $@ $(LDLIBS), \
	       "LINK   $@")

$(UNTRUSTED_PROXY_SOURCE) $(UNTRUSTED_PROXY_HEADER): $(ENCLAVE_EDL)
	$(call abbrev-cmd, \
	       $(SGX_EDGER8R) --untrusted-dir $(@D) --untrusted $<, \
	       "GEN    $@")

untrusted: $(UNTRUSTED_TARGET)

TEST_TARGET  := test/usig_test
TEST_SOURCES := test/usig_test.c
TEST_OBJECTS := $(TEST_SOURCES:.c=.o)
TEST_DEPS    := $(TEST_SOURCES:.c=.d)

TEST_ARTIFACTS := $(TEST_TARGET) $(TEST_OBJECTS) $(TEST_DEPS)

-include $(TEST_DEPS)

$(TEST_OBJECTS): CFLAGS   := $(UNTRUSTED_CFLAGS)
$(TEST_OBJECTS): CPPFLAGS := $(UNTRUSTED_CPPFLAGS) -I$(CURDIR)/shim

$(TEST_TARGET): LDFLAGS := $(UNTRUSTED_LDFLAGS)
$(TEST_TARGET): LDLIBS  := $(UNTRUSTED_LDLIBS)
$(TEST_TARGET): $(TEST_OBJECTS) $(UNTRUSTED_TARGET)
	$(call abbrev-cmd, \
	       $(CC) $(LDFLAGS) $^ -o $@ $(LDLIBS), \
	       "LINK   $@")

.PHONY: usig-test
usig-test: untrusted $(TEST_TARGET)

check: usig-test enclave
	$(call abbrev-cmd, \
	       ./$(TEST_TARGET) $(ENCLAVE_SIGNED), \
	       "RUN    $(TEST_TARGET)")

ARTIFACTS := $(ENCLAVE_ARTIFACTS) $(UNTRUSTED_ARTIFACTS) \
             $(TEST_ARTIFACTS)

clean:
	$(call abbrev-cmd, \
	       rm -f $(ARTIFACTS), \
	       "CLEAN")
