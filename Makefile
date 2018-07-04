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

.PHONY: all build clean check test lint generate

all: build check
test: check

usig-target-list := usig-help usig-all usig-build usig-clean		\
                    usig-check usig-test usig-enclave usig-untrusted	\
                    usig-go-wrapper
.PHONY: $(usig-target-list)

.PHONY: help
help:
	@echo 'Usage: make [target]...'
	@echo ''
	@echo 'Generic targets:'
	@echo '  all (default)    - Build and test all'
	@echo '  build            - Build all'
	@echo '  clean            - Remove all build artifacts'
	@echo '  check|test       - Run all tests'
	@echo '  lint             - Run code quality checks'
	@echo '  protos           - Generate protobuf messages'
	@echo '  mocks            - Generate the gomock files'
	@echo '  generate         - Generate dependent files'
	@echo ''
	@echo 'Specific targets:'
	@echo '  usig-*           - Make USIG target, where target is one of:'
	@echo '                     $(usig-target-list)'

build: usig-build | bin
	go build -o sample/bin/peer ./sample/peer
	go build -o sample/bin/keytool ./sample/authentication/keytool

clean: usig-clean
	rm -rf bin

check: usig-build usig-check
	go test -short -race ./...

lint: usig-go-wrapper
	gometalinter --vendor --enable="gofmt" --enable="misspell" --exclude='.+[.]pb[.]go' ./...

generate:
	go generate ./...

$(usig-target-list):
	$(MAKE) -C usig/sgx $(patsubst usig-%,%,$@)

bin:
	@mkdir -p bin
