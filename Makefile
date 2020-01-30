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

INSTALL := install
INSTALL_PROGRAM := $(INSTALL)
INSTALL_DATA := $(INSTALL) -m 644

builddir := sample/build

prefix ?= sample
bindir := $(prefix)/bin
libdir := $(prefix)/lib

.PHONY: all build install uninstall clean check test lint generate

all: build check
test: check

usig-target-list := usig-help usig-all usig-build usig-clean		\
                    usig-check usig-test usig-enclave usig-untrusted
.PHONY: $(usig-target-list)

.PHONY: help
help:
	@echo 'Usage: make [target]...'
	@echo ''
	@echo 'Generic targets:'
	@echo '  all (default)    - Build and test all'
	@echo '  build            - Build all'
	@echo '  install          - Build and install artifacts'
	@echo '  uninstall        - Uninstall artifacts'
	@echo '  clean            - Remove all build artifacts'
	@echo '  check|test       - Run all tests'
	@echo '  lint             - Run code quality checks'
	@echo '  generate         - Generate dependent files'
	@echo ''
	@echo 'Specific targets:'
	@echo '  usig-*           - Make USIG target, where target is one of:'
	@echo '                     $(usig-target-list)'

build: prerequisite-check usig-build
	go build -o $(builddir)/keytool ./sample/authentication/keytool
	go build -o $(builddir)/peer ./sample/peer

install: build
	$(INSTALL_PROGRAM) -D $(builddir)/keytool $(bindir)/keytool
	$(INSTALL_PROGRAM) -D $(builddir)/peer $(bindir)/peer
	$(INSTALL_DATA) -D usig/sgx/shim/libusig_shim.so $(libdir)/libusig_shim.so
	$(INSTALL_DATA) -D usig/sgx/enclave/libusig.signed.so $(libdir)/libusig.signed.so

uninstall:
	rm -f $(bindir)/keytool
	rm -f $(bindir)/peer
	rm -f $(libdir)/libusig.signed.so

clean: usig-clean
	rm -rf $(builddir)

check: usig-build usig-check
	go test -short -race ./...

lint:
	golangci-lint run ./...

generate:
	go generate ./...

prerequisite-check:
	@bash tools/prerequisite-check.sh

$(usig-target-list):
	$(MAKE) -C usig/sgx $(patsubst usig-%,%,$@)
