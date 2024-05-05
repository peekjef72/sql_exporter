go build -ldflags "-X github.com/prometheus/common/version.Version=0.8.1 -X github.com/prometheus/common/version.Revision=91693a054d7316ba635deadf855f5cd9eab57a9e -X github.com/prometheus/common/version.Branch=multidb_devs -X github.com/prometheus/common/version.BuildDate=20230904-09:03:15 -X github.com/prometheus/common/version.BuildUser=peekjef72@pc_collab"  -tags "netgo,usergo,static,mssql" -o "mssql_exporter" .


## oracle
=> change oci8.pc to adapt the include and lib directories according to where the client is installed.
set variable PKG_CONFIG_PATH to local directory where oci8.pc stands
    PKG_CONFIG_PATH=/home/users/d107684/go/src/sql_exporter
export PKG_CONFIG_PATH
then
```bash
$GOBIN/promu -vc .promu-oracle.yml build
```
to retrieve the constant values (version, build...) and report them it the command

```text
go build -o "oracledb_exporter" -ldflags "-X github.com/prometheus/common/version.Version=0.8.2 -X github.com/prometheus/common/version.Revision=0fa6905364eecb575c52c69d8f1cf1618682191d -X github.com/prometheus/common/version.Branch=multidb_devs -X github.com/prometheus/common/version.BuildDate=2024-05-03T15:18:26.209Z -X github.com/prometheus/common/version.BuildUser=d107684@dal-v-survdadc " -tags "netgo,usergo,static,oracle" .
```