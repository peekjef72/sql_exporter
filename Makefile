# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO     := go
GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU  := $(GOPATH)/bin/promu
pkgs    = $(shell $(GO) list ./... | grep -v /vendor/)

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(shell pwd)
BASE_DIR            ?= $(shell dirname $(BIN_DIR))
MSSQL_DIR			?= $(BASE_DIR)/mssql_exporter
DB2_DIR				?= $(BASE_DIR)/db2_exporter
DOCKER_IMAGE_NAME   ?= mssql-exporter
DOCKER_IMAGE_TAG    ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))


all: promu build build-db2

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test:
	@echo ">> running tests"
	@$(GO) test -short -race $(pkgs)

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

build: promu
	@echo ">> building MSSQL binaries"
	@$(PROMU) build --prefix $(PREFIX) -v

build-ora: promu
	@echo ">> building ORACLE binaries"
	@$(PROMU) build --prefix $(PREFIX) --config=.promu-oracle.yml

build-db2: promu
	@echo ">> building DB2 binaries"
	@$(PROMU) build --prefix $(PREFIX) --config=.promu-db2.yml

tarball: promu
	@echo ">> building release tarball"
	@mv $(BIN_DIR)/examples/mssql_targets $(BIN_DIR)/examples/targets
	@echo ">>> renaming directory to $(MSSQL_DIR)"
	@ln -s $(BIN_DIR) $(MSSQL_DIR)
	@cd $(MSSQL_DIR) && $(PROMU) tarball --prefix $(MSSQL_DIR) $(MSSQL_DIR)
	@mv $(BIN_DIR)/examples/targets $(BIN_DIR)/examples/mssql_targets
	@rm $(MSSQL_DIR)

tarball-db2: promu
	@echo ">> building DB2 release tarball"
	@mv $(BIN_DIR)/examples/db2_targets $(BIN_DIR)/examples/targets
	@echo ">>> renaming directory to $(DB2_DIR)"
	@ln -s $(BIN_DIR) $(DB2_DIR)
	@cd $(DB2_DIR) && $(PROMU) tarball --prefix $(DB2_DIR) --config=.promu-db2.yml $(DB2_DIR)
	@mv $(BIN_DIR)/examples/targets $(BIN_DIR)/examples/db2_targets
	@rm $(DB2_DIR)

tarball-ora: promu
	@echo ">> building ORACLE release tarball"
	@mv $(BIN_DIR)/examples/oracle_targets $(BIN_DIR)/examples/targets
	@echo ">>> renaming directory to $(ORACLE_DIR)"
	@ln -s $(BIN_DIR) $(ORACLE_DIR)
	@cd $(ORACLE_DIR) && $(PROMU) tarball --prefix $(ORACLE_DIR) --config=.promu-oracle.yml $(ORACLE_DIR)
	@mv $(BIN_DIR)/examples/targets $(BIN_DIR)/examples/oracle_targets
	@rm $(ORACLE_DIR)

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		$(GO) install github.com/prometheus/promu@v0.13.0


.PHONY: all style format build test vet tarball docker promu
