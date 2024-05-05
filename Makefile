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


all: promu build

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

build-mssql: promu
	@echo ">> building MSSQL binaries"
	@$(PROMU) build --prefix $(PREFIX) -v

build-ora: promu 
	@echo ">> building ORACLE binaries"
	@$(PROMU) build --prefix $(PREFIX) --config=.promu-oracle.yml

build-db2: promu 
	@echo ">> building DB2 binaries"
	@$(PROMU) build --prefix $(PREFIX) --config=.promu-db2.yml

build-hana: promu 
	@echo ">> building HANASQL binaries"
	@$(PROMU) build --prefix $(PREFIX) --config=.promu-hana.yml

build: build-mssql build-db2 build-hana #build-ora

tarball-mssql: mssql_exporter
	@echo ">> building mssql release tarball"
	@mv $(BIN_DIR)/contribs/mssql_exporter $(BIN_DIR)/config
	@git remote set-url origin "https://github.com/peekjef72/mssql_exporter.git"
	@$(PROMU) tarball --config=.promu.yml
	@git remote set-url origin "https://github.com/peekjef72/sql_exporter.git"
	@mv $(BIN_DIR)/config $(BIN_DIR)/contribs/mssql_exporter

tarball-db2: build-db2
	@echo ">> building db2 release tarball"
	@mv $(BIN_DIR)/contribs/db2_exporter $(BIN_DIR)/config
	@git remote set-url origin "https://github.com/peekjef72/db2_exporter.git"
	@$(PROMU) tarball --config=.promu-db2.yml
	@git remote set-url origin "https://github.com/peekjef72/sql_exporter.git"
	@mv $(BIN_DIR)/config $(BIN_DIR)/contribs/db2_exporter

tarball-ora: build-ora
	@echo ">> building oracledb release tarball"
	@mv $(BIN_DIR)/contribs/oracledb_exporter $(BIN_DIR)/config
	@git remote set-url origin "https://github.com/peekjef72/oracledb_exporter.git"
	@$(PROMU) tarball --config=.promu-oracle.yml
	@git remote set-url origin "https://github.com/peekjef72/sql_exporter.git"
	@mv $(BIN_DIR)/config $(BIN_DIR)/contribs/oracledb_exporter

tarball-hana: build-hana 
	@echo ">> building HANASQL release tarball"
	@mv $(BIN_DIR)/contribs/hanasql_exporter $(BIN_DIR)/config
	@git remote set-url origin "https://github.com/peekjef72/hanasql_exporter.git"
	@$(PROMU) tarball --config=.promu-hana.yml
	@git remote set-url origin "https://github.com/peekjef72/sql_exporter.git"
	@mv $(BIN_DIR)/config $(BIN_DIR)/contribs/hanasql_exporter

tarball: tarball-mssql tarball-db2 tarball-hana #tarball-ora

docker:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		$(GO) install github.com/prometheus/promu@v0.13.0


.PHONY: all style format build test vet tarball docker promu
