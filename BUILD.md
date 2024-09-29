go build -ldflags "-X github.com/prometheus/common/version.Version=0.8.1 -X github.com/prometheus/common/version.Revision=91693a054d7316ba635deadf855f5cd9eab57a9e -X github.com/prometheus/common/version.Branch=multidb_devs -X github.com/prometheus/common/version.BuildDate=20230904-09:03:15 -X github.com/prometheus/common/version.BuildUser=peekjef72@pc_collab"  -tags "netgo,usergo,static,mssql" -o "mssql_exporter" .


## mssql

build an environment definition file with :
- mssql tag enabled

e.g.: .env_mssql

```shell
GO111MODULE=on
GOSUMDB=off
GOFLAGS="-tags=mssql"

```

load env and play make to build mssql_exporter

```shell
. .env_mssql
export GOENV=.env_mssql
make build-mssql
```

## oracle

requirement:
oracle instant client v19 downloaded on oracle site (rpm or pkg).
- oracle-instantclient19.6-basiclite

build an environment definition file with :
- oracle tag enabled
- CGO_CGLAGS set to oracle include stand
- CGO_LDFLAGS set to oracle dynamic libraries stand
- PKG_CONFIG_PATH set to where oci8.pc stands

e.g.: .env_oracle

```shell
GO111MODULE=on
GOSUMDB=off
GOFLAGS="-tags=oracle"
CGO_CFLAGS=-I/usr/include/oracle/19.16/client64/
CGO_LDFLAGS=-L/usr/lib/oracle/19.16/client64/lib
PKG_CONFIG_PATH=/home/users/XXXXX/go/src/sql_exporter
```

load env and play make to build oracle_exporter

```shell
. .env_oracle
export GOENV=.env_oracle
make build-ora
```

## db2

Pre-requirements: clibdrivers

build an environment definition file with :
- db2 tag enabled
- CGO_CGLAGS set to db2 include stand
- CGO_LDFLAGS set to db2 dynamic libraries stand

e.g.: .env_db2

```shell
GO111MODULE=on
GOSUMDB=off
GOFLAGS="-tags=db2"
CGO_CFLAGS=-I/opt/db2_exporter/lib/clidriver/include
CGO_LDFLAGS=-L/opt/db2_exporter/lib/clidriver/lib

```

load env and play make to build db2_exporter

```shell
. .env_db2
export GOENV=.env_db2
make build-db2
```

## hanasql

build an environment definition file with :
- hana tag enabled

e.g.: .env_hana

```shell
GO111MODULE=on
GOSUMDB=off
GOFLAGS="-tags=hana"

```

load env and play make to build hanasql_exporter

```shell
. .env_hana
export GOENV=.env_hana
make build-hana
```



