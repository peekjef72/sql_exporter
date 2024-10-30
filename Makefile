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
PASSWD_ENCRYPT := $(GOPATH)/bin/passwd_encrypt
pkgs    = $(shell $(GO) list ./... | grep -v /vendor/)

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(shell pwd)
BASE_DIR            ?= $(shell dirname $(BIN_DIR))
DOCKER_IMAGE_NAME   ?= mssql-exporter
DOCKER_IMAGE_TAG    ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))

all: promu build

style:
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

test:
	@echo ">> running tests"
	@$(GO) test -short -race $(pkgs)

build-mssql: promu passwd_encrypt
	@echo ">> building MSSQL binaries"
	@$(PROMU) build

build-oracledb: promu passwd_encrypt
	@echo ">> building ORACLE binaries"
	@. $(PREFIX)/.env_oracle && $(PROMU) build --prefix $(PREFIX) --config=.promu-oracle.yml
	# @$(PROMU) build --config=.promu-oracle.yml

build-db2: promu passwd_encrypt
	@echo ">> building DB2 binaries"
	@. $(PREFIX)/.env_db2 && $(PROMU) build --config=.promu-db2.yml

build-hanasql: promu passwd_encrypt
	@echo ">> building HANASQL binaries"
	@$(PROMU) build --config=.promu-hana.yml

build: build-mssql build-db2 build-hanasql build-oracledb

# @mv $(BIN_DIR)/contribs/mssql_exporter $(BIN_DIR)/config
# @cp $(PASSWD_ENCRYPT) $(BIN_DIR)
# @mv $(BIN_DIR)/cmd/mssql_exporter $(BIN_DIR)
# @git remote set-url origin "https://github.com/peekjef72/mssql_exporter.git"
# @$(PROMU) tarball --config=.promu.yml
# @git remote set-url origin "https://github.com/peekjef72/sql_exporter.git"
# @mv $(BIN_DIR)/config $(BIN_DIR)/contribs/mssql_exporter
# @rm $(BIN_DIR)/passwd_encrypt
# @mv $(BIN_DIR) $(BIN_DIR)/cmd/mssql_exporter 
tarball-mssql: promu passwd_encrypt build-mssql
	@echo ">> building mssql release tarball"
	@$(shell ./build_tarball.sh mssql_exporter)


tarball-db2: promu passwd_encrypt build-db2
	@echo ">> building db2 release tarball"
	@$(shell ./build_tarball.sh db2_exporter)

tarball-oracledb: promu passwd_encrypt build-oracledb
	@echo ">> building oracledb release tarball"
	@$(shell ./build_tarball.sh oracledb_exporter)

tarball-hanasql: promu passwd_encrypt build-hanasql 
	@echo ">> building HANASQL release tarball"
	@$(shell ./build_tarball.sh hanasql_exporter)

# tarball: tarball-mssql tarball-db2 tarball-hana #tarball-oracle

tarball-all: build
	@echo ">> build release tarball"
	@$(shell ./build_tarball.sh all)

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:
	$(GO) install github.com/prometheus/promu@latest

passwd_encrypt:
	$(GO) install github.com/peekjef72/passwd_encrypt@latest

.PHONY: all style format build test vet tarball docker promu
