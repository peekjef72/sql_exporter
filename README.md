# Prometheus SQL Exporter 
Exporter for [Prometheus](https://prometheus.io) that can collect multiple type of sql servers.

As examples 4 configurations for exporters are provided (see contribs):
 * [mssql](contribs/mssql_exporter/)
 * [db2](contribs/db2_exporter/)
 * [oracle](contribs/oracle_exporter/)
 * [hana](contribs/hanasql_exporter/)

## Overview

<figure>
    <img src="contribs/mssql_exporter/screenshots/mssql_dashboard_general.PNG" alt="overview MSSQL">
    <figcaption style="font-style: italic; text-align: center;">MSSQL dashboard overview</figcaption>
</figure>

SQL Exporter is a configuration driven exporter that exposes metrics gathered from MSSQL Servers, for use by the Prometheus monitoring system. Out of the box, it provides support for Microsoft SQL Server, IBM DB2, HANADB but any DBMS for which a Go driver is available may be monitored after rebuilding the binary with the DBMS driver included.

The exporter is multi targets, meaning that you can set several target servers configuration identified each by name, then Prometheus can scratch these targets by adding the parameter target into the url. It can also works with a default target configuration and authentication models 

The collected metrics and the queries that produce them are entirely configuration defined. **No SQL query are hard coded inside the exporter**. SQL queries are grouped into
collectors -- logical groups of queries, e.g. *query stats* or *I/O stats*, mapped to the metrics they populate.
This means you can quickly and easily set up custom collectors to measure data quality, whatever that might mean in your specific case.

Per the Prometheus philosophy, scrapes are synchronous (metrics are collected on every `/metrics` poll) but, in order to keep load at reasonable levels, minimum collection intervals may optionally be set per collector, producing cached
metrics when queried more frequently than the configured interval.

## building

### msuql or hanasql
mssql_exporter and hanasql_exporter can be compiled staticaly.
```bash
make build-mssql build-hana
```

### db2

db2_exporter can't be compiled staticaly.
ctdriver must be installed first for compilation and for **usage**.
see [go_ibm_db/INSTALL.md](https://github.com/ibmdb/go_ibm_db/blob/master/INSTALL.md)

Here a small summary for linux:
* download the cli :
  ```bash
  mkdir $HOME/db2
  cd $HOME/db2
  curl --output linuxx64_odbc_cli.tar.gz https://public.dhe.ibm.com/ibmdl/export/pub/software/data/db2/drivers/odbc_cli/linuxx64_odbc_cli.tar.gz
  tar xzf linuxx64_odbc_cli.tar.gz
  export IBM_DB_HOME=/home/<user>/db2/clidriver
  export CGO_CFLAGS=-I$IBM_DB_HOME/include
  export CGO_LDFLAGS=-L$IBM_DB_HOME/lib
  export LD_LIBRARY_PATH=$IBM_DB_HOME/lib:$LD_LIBRARY_PATH
  ```
  If you have root access you can set path to DB2 dynamic library via ld.so.conf:

  ```bash
  vi /etc/ld.so.conf.d/db2_odbc.conf
  ```

  ```text
  /home/jfpik/db2/clidriver/lib
  ```

  ```bash
  ldconfig
  ```
  if you plan to recompile db2_exporter several times, you can build an env file:

  ```bash
  vi .env_db2
  ```

  ```text
  export IBM_DB_HOME=/home/jfpi<user>k/db2/clidriver
  export CGO_CFLAGS="-I $IBM_DB_HOME/include"
  export CGO_LDFLAGS="-L $IBM_DB_HOME/lib"
  export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$IBM_DB_HOME/lib

  GO111MODULE=on
  GOSUMDB=off
  GOFLAGS="-tags=db2"
  ```

  Then use this file:

    ```bash
    . .env_db2
    make build-db2
    ```

for others urls check [setup.go](https://github.com/ibmdb/go_ibm_db/blob/master/installer/setup.go)

### OracleDB

oracledb_exporter can't be compiled staticaly too. Oracle Instant client must be installed first on system.
Download and install an oracle instant client: recommanded on linux oracle-instantclient19.23-basiclite-19.23.0.0.0 (oracle-instantclient19.6-basiclite-19.6.0.0.0-1.x86_64.rpm
 and oracle-instantclient19.23-devel-19.23.0.0.0-1.x86_64.rpm)

```
curl --output ~/Downloads/oracle-instantclient19.23-basic-19.23.0.0.0-1.x86_64.rpm https://yum.oracle.com/repo/OracleLinux/OL8/oracle/instantclient/x86_64/getPackage/oracle-instantclient19.23-basic-19.23.0.0.0-1.x86_64.rpm
curl --output ~/Downloads/oracle-instantclient19.23-devel-19.23.0.0.0-1.x86_64.rpm https://yum.oracle.com/repo/OracleLinux/OL8/oracle/instantclient/x86_64/getPackage/oracle-instantclient19.23-devel-19.23.0.0.0-1.x86_64.rpm
```

then install the download package, and update library path
```bash
dnf install file:///home/jfpik/Downloads/oracle-instantclient19.23-basic-19.23.0.0.0-1.x86_64.rpm 
dnf install file:///home/jfpik/Downloads/oracle-instantclient19.23-devel-19.23.0.0.0-1.x86_64.rpm

ldconfig
```

check oci8.pc and .promu-oracle.yml file to adapt version or path with installed rpm.

## Usage

Get Prometheus MSSQL Exporter as a [packaged release](https://github.com/free/mssql_exporter/releases/latest) or
build it yourself:

```
$ go install github.com/free/sql_exporter/cmd/sql_exporter
```

then run it from the command line:

```
$ mssql_exporter
```

Use the `--help` flag to get help information.

```
$ ./mssql_exporter --help
usage: mssql_exporter [<flags>]

Flags:
  -h, --help               Show context-sensitive help (also try --help-long and --help-man).
      --config.data-source-name=CONFIG.DATA-SOURCE-NAME  
                           Data source name to override the value in the configuration file with.
      --web.listen-address=":9399"  
                           The address to listen on for HTTP requests.
      --web.telemetry-path="/metrics"  
                           Path under which to expose collector's internal metrics.
  -c, --config.file="config/config.yml"  
                           mssql_exporter Exporter configuration file.
  -d, --debug              debug connection checks.
  -n, --dry-run            check exporter configuration file and try to collect a target then exit.
  -t, --target=TARGET      In dry-run mode specify the target name, else ignored.
      --log.level=info     Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt  Output format of log messages. One of: [logfmt, json]
  -V, --version            Show application version.
```

## Configuration

SQL Exporter is deployed alongside the DB server it collects metrics from. If both the exporter and the DB
server are on the same host, they will share the same failure domain: they will usually be either both up and running
or both down. When the database is unreachable, `/metrics` responds with HTTP code 500 Internal Server Error, causing
Prometheus to record `up=0` for that scrape. Only metrics defined by collectors are exported on the `/metrics` endpoint.
SQL Exporter process metrics are exported at `/sql_exporter_metrics`.

The configuration examples listed here only cover the core elements. For a comprehensive and comprehensively documented
configuration file check out 
[`documentation/sql_exporter.yml`](https://github.com/free/sql_exporter/tree/master/documentation/sql_exporter.yml).
You will find ready to use "standard" DBMS-specific collector definitions in the
[`examples`](https://github.com/free/sql_exporter/tree/master/examples) directory. You may contribute your own collector
definitions and metric additions if you think they could be more widely useful, even if they are merely different takes
on already covered DBMSs.

**`./mssql_exporter.yml`**

```yaml
# Global settings and defaults.
global:
  # Subtracted from Prometheus' scrape_timeout to give us some headroom and prevent Prometheus from
  # timing out first.
  scrape_timeout_offset: 500ms
  # Minimum interval between collector runs: by default (0s) collectors are executed on every scrape.
  min_interval: 0s
  # Maximum number of open connections to any one target. Metric queries will run concurrently on
  # multiple connections.
  max_connections: 3
  # Maximum number of idle connections to any one target.
  max_idle_connections: 3

# The target to monitor and the collectors to execute on it.
targets:
  # list of target to collect
  - target:
    name: MY_INSTANCE
    # Data source name always has a URI schema that matches the driver name. In some cases (e.g. MySQL)
    # the schema gets dropped or replaced to match the driver expected DSN format.
    data_source_name: 'sqlserver://prom_user:prom_password@dbserver1.example.com:1433'

    # Collectors (referenced by name) to execute on the target.
    collectors: [mssql_standard]

  # or specify each target in a configuration file with same format than for a target
  - targets_files: [ "targets/*.yml" ]

# Collector definition files.
collector_files: 
  - "*.collector.yml"
```

### Collectors

Collectors may be defined inline, in the exporter configuration file, under `collectors`, or they may be defined in
separate files and referenced in the exporter configuration by name, making them easy to share and reuse.

The collector definition below generates gauge metrics of the form `pricing_update_time{market="US"}`.

**`./pricing_data_freshness.collector.yml`**

```yaml
# This collector will be referenced in the exporter configuration as `pricing_data_freshness`.
collector_name: pricing_data_freshness

# A Prometheus metric with (optional) additional labels, value and labels populated from one query.
metrics:
  - metric_name: pricing_update_time
    type: gauge
    help: 'Time when prices for a market were last updated.'
    key_labels:
      # Populated from the `market` column of each row.
      - Market
    static_labels:
      # Arbitrary key/value pair
      portfolio: income
    values: [LastUpdateTime]
    query: |
      SELECT Market, max(UpdateTime) AS LastUpdateTime
      FROM MarketPrices
      GROUP BY Market
```
### target file

```yaml
name: "target_name"
data_source_name: "sqlserver://nowhere:1434/instance_2?user%20id=domain\\user&password={Xöe8;vhmbr4yYEL0~Ybfg}&database=myDatabase"

# Collectors (referenced by name) to execute on the target.
collectors:
  - mssql_standard

```
### Data Source Names

To keep things simple and yet allow fully configurable database connections to be set up, SQL Exporter uses DSNs (like
`sqlserver://prom_user:prom_password@dbserver1.example.com:1433`) to refer to database instances. However, because the
Go `sql` library does not allow for automatic driver selection based on the DSN (i.e. an explicit driver name must be
specified) SQL Exporter uses the schema part of the DSN (the part before the `://`) to determine which driver to use.

Unfortunately, while this works out of the box with the [MS SQL Server](https://github.com/denisenkom/go-mssqldb) and
[PostgreSQL](github.com/lib/pq) drivers, the [MySQL driver](github.com/go-sql-driver/mysql) DSNs format does not include
a schema and the [Clickhouse](github.com/kshvakov/clickhouse) one uses `tcp://`. So SQL Exporter does a bit of massaging
of DSNs for the latter two drivers in order for this to work:

DB | SQL Exporter expected DSN | Driver sees
:---|:---|:---
Oracle | `oracle://<host>:<port>/<sid>?user_id=<login>&password=<password>&params=<VAL>`<br>or<br>`oci8:///user:passw@host:port/dbname?params=<VAL>`| optionnal parameters: <ul><li>loc=&lt;time.location&gt; default time.UTC<br><li>isolation=&lt;READONLY&#124;SERIALIZABLE&#124;DEFAULT&gt;<li>questionph=&lt;enableQuestionPlaceHolders&gt; true&#124;false<li>prefetch_rows=&lt;u_int&gt; default 0<li>prefetch_memory=&lt;u_int&gt; default 4096<li>as=&lt;sysdba&#124;sysasm&#124;sysoper default empty.<li>stmt_cache_size=<u_int>default 0</ul>
<strike>MySQL</strike> | <strike>`mysql://user:passw@protocol(host:port)/dbname` | `user:passw@protocol(host:port)/dbname`
PostgreSQL | `postgres://user:passw@host:port/dbname` | *unchanged*
SQL Server | `sqlserver://user:passw@host:port/instance` | *unchanged*

## Why It Exists

SQL Exporter started off as an exporter for Microsoft SQL Server, for which no reliable exporters exist. But what is
the point of a configuration driven SQL exporter, if you're going to use it along with 2 more exporters with wholly
different world views and configurations, because you also have MySQL and PostgreSQL instances to monitor?

A couple of alternative database agnostic exporters are available -- https://github.com/justwatchcom/sql_exporter and
https://github.com/chop-dbhi/prometheus-sql -- but they both do the collection at fixed intervals, independent of
Prometheus scrapes. This is partly a philosophical issue, but practical issues are not all that difficult to imagine:
jitter; duplicate data points; or collected but not scraped data points. The control they provide over which labels get
applied is limited, and the base label set spammy. And finally, configurations are not easily reused without
copy-pasting and editing across jobs and instances.
