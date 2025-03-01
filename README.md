# Prometheus SQL Exporter

Exporter for [Prometheus](https://prometheus.io) that can collect multiple type of sql servers.

As examples 4 configurations for exporters are provided (see contribs):

* [mssql](contribs/mssql_exporter/)
* [db2](contribs/db2_exporter/)
* [oracle](contribs/oracle_exporter/)
* [hana](contribs/hanasql_exporter/)

This exporter was [free/sql_exporter](https://github.com/free/sql_exporter) before version 0.5.
In actual version the exporter is compiled via tag for specific sql server. The advantage is to have only one logic for configuration and deployement.

## Overview

<figure>
    <img src="contribs/mssql_exporter/screenshots/mssql_dashboard_general.PNG" alt="overview MSSQL">
    <figcaption style="font-style: italic; text-align: center;">MSSQL dashboard overview</figcaption>
</figure>

SQL Exporter is a configuration driven exporter that exposes metrics gathered from MSSQL Servers, for use by the Prometheus monitoring system. Out of the box, it provides support for Microsoft SQL Server, IBM DB2, HANADB, and Oracle but any DBMS for which a Go driver is available may be monitored after rebuilding the binary with the DBMS driver included.

The exporter is multi targets, meaning that you can set several target servers configuration identified each by name, then Prometheus can scratch these targets by adding the parameter target into the url. It can also works with a default target configuration and authentication models.

The collected metrics and the queries that produce them are entirely configuration defined. **No SQL query are hard coded inside the exporter**. SQL queries are grouped into
collectors -- logical groups of queries, e.g. *query stats* or *I/O stats*, mapped to the metrics they populate.
This means you can quickly and easily set up custom collectors to measure data quality, whatever that might mean in your specific case.

Per the Prometheus philosophy, scrapes are synchronous (metrics are collected on every `/metrics` poll) but, in order to keep load at reasonable levels, minimum collection intervals may optionally be set per collector, producing cached
metrics when queried more frequently than the configured interval.

## building

### mssql or hanasql

mssql_exporter and hanasql_exporter can be compiled staticaly.

```bash
make build-mssql build-hanasql
```

pre-requirements:

* gcc (installed via your prefered package manager)

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

  for RH 10, libcrypt.so.1 is required and may need to install libxcrypt-compat:

  ```bash
  dnf install libxcrypt-compat
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
  export IBM_DB_HOME=/home/<user>/db2/clidriver
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

```bash
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

Then use this file:

```bash
. .env_oracle
make build-oracledb
```

## Usage

Usage is the same for all sql_exporters, but will be explained only for mssql_exporter.

Get Prometheus MSSQL Exporter as a [packaged release](https://github.com/jfpik/sql_exporter/releases/latest) or
build it yourself (see above.)

then run it from the command line:

```shell
$ ./mssql_exporter
```

Use the `--help` flag to get help information.

```shell
$ ./mssql_exporter --help
usage: mssql_exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try --help-long and --help-man).
      --config.data-source-name=CONFIG.DATA-SOURCE-NAME  
                                 Data source name to override the value in the configuration file with.
      --web.telemetry-path="/metrics"  
                                 Path under which to expose collector's internal metrics.
  -c, --config.file="config/config.yml"  
                                 mssql_exporter Exporter configuration file.
  -d, --[no-]debug               debug connection checks.
  -n, --[no-]dry-run             check exporter configuration file and try to collect a target then exit.
  -t, --target=TARGET            In dry-run mode specify the target name, else ignored.
  -m, --model="default"          In dry-run mode specify the model name to build the dynamic target, else ignored.
  -a, --auth_key=AUTH_KEY        In dry-run mode specify the auth_key to use, else ignored.
  -o, --collector=COLLECTOR      Specify the collector name restriction to collect, replace the collector_names set for each
                                 target.
      --[no-]web.systemd-socket  Use systemd socket activation listeners instead of port listeners (Linux only).
      --web.listen-address=:9399 ...  
                                 Addresses on which to expose metrics and web interface. Repeatable for multiple addresses.
                                 Examples: `:9100` or `[::1]:9100` for http, `vsock://:9100` for vsock
      --web.config.file=""       Path to configuration file that can enable TLS or authentication. See:
                                 https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt, json]
  -V, --[no-]version             Show application version.

```

## Configuration

SQL Exporter is deployed alongside the DB server it collects metrics from. If both the exporter and the DB
server are on the same host, they will share the same failure domain: they will usually be either both up and running
or both down. When the database is unreachable, `/metrics` responds with HTTP code 500 Internal Server Error, causing
Prometheus to record `up=0` for that scrape. Only metrics defined by collectors are exported on the `/metrics` endpoint.
SQL Exporter process metrics are exported at `/sql_exporter_metrics`.

The configuration examples listed here only cover the core elements.
You will find ready to use "standard" DBMS-specific collector definitions in the
[`examples`](https://github.com/peekjef72/sql_exporter/tree/master/contribs) directory. You may contribute your own collector
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

DB | SQL Exporter expected DSN | Driver sees
:---|:---|:---
DB2 | `db2:////<hostname>:<port>?user%20id=<login>&password=<password>&database=<database>&protocol=...`<br>or<br>`db2://DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;` | _
Hanasql | `hdb:////<hostname>:<port>?user%20id=<login>&password=<password>&database=<database>&protocol=...`<br>or<br>`hdb://DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;` | optionnal parameters: <ul><li>databaseName=&lt;dbname&gt;<li> defaultSchema=&lt;schema&gt; <li>timeout=&lt;timeout_seconds&gt;<li>pingInterval=&lt;intervanl_seconds&gt;<li>TLSRootCAFile=&lt;file&gt;<li>TLSServerName=&lt;file&gt;<li>TLSInsecureSkipVerify=&lt;file&gt;</ul>
Oracle | `oracle://<host>:<port>/<sid>?user_id=<login>&password=<password>&params=<VAL>`<br>or<br>`oci:///user:passw@host:port/dbname?params=<VAL>`<br>or<br>`oracle://DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>; optional=<value>` | optionnal parameters: <ul><li>loc=&lt;time.location&gt; default time.UTC<br><li>isolation=&lt;READONLY&#124;SERIALIZABLE&#124;DEFAULT&gt;<li>questionph=&lt;enableQuestionPlaceHolders&gt; true&#124;false<li>prefetch_rows=&lt;u_int&gt; default 0<li>prefetch_memory=&lt;u_int&gt; default 4096<li>as=&lt;sysdba&#124;sysasm&#124;sysoper default empty.<li>stmt_cache_size=<u_int>default 0</ul>
SQL Server | `sqlserver://<hostname>:<port>/<instance>?user%20id=<login>&password=<password>&database=<database>&protocol=...`<br>or<br>`sqlserver://DATABASE=<database>; HOSTNAME=<hostname>; PORT=<port>; PROTOCOL=<protocol>; UID=<login>; PWD=<password>;` | *unchanged*
<strike>PostgreSQL</strike> | <strike>`postgres://user:passw@host:port/dbname`</strike> | <strike>*unchanged*</strike>

### User authentication / password encryption

If you don't want to write the users' password in clear text in config file (targets files on the exporter), you can encrypt them with a shared password.

How it works:

* choose a shared password (passphrase) of 16 24 or 32 bytes length and store it your in your favorite password keeper (keepass for me).
* use passwd_encrypt tool:

    ```bash
    ./passwd_encrypt 
    give the key: must be 16 24 or 32 bytes long
    enter key: 0123456789abcdef 
    enter password: mypassword
    Encrypting...
    Encrypted message hex: CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=
    $
    ```

* set the user password in the target file part:

    ```yaml
    name: <target_name>
    # doublequotes are mandatory because of ":" in string
    data_source_name: "<driver>://<hostname>:<port>/<instance>?database=<database>?protocol=TCP&isolation=READONLY"
    auth_config:
      user: <user>
      # password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
      password: /encrypted/qtj1GrR3HcqtJFoBAnEIXlQYQtcptu4COs1Q3A85A5z6vv5HXEC4n0aXWQI=
    collectors:
      - <collectors_name>
    ```

* set the shared passphrase in prometheus config (either job or node file)

  * prometheus jobs with target files:

    ```yaml
    #--------- Start prometheus <driver> exporter  ---------#
    - job_name: "<driver>"
        metrics_path: /metrics
        file_sd_configs:
          - files: [ "/etc/prometheus/<driver>_nodes/*.yml" ]
        relabel_configs:
          - source_labels: [__address__]
            target_label: __param_target
          - source_labels: [__tmp_source_host]
            target_label: __address__

    #--------- End prometheus <driver> exporter ---------#
    ```

    ```yaml
    - targets: [ "<target_name>" ]
      labels:
        # if you have activated password encrypted passphrass
        __param_auth_key: 0123456789abcdef
        host: "<target_name>_fullqualified.domain.name"
        # custom labels…
        environment: "DEV"
    ```

## Loging level

You can change the log.level online by sending a signal USR2 to the process. It will increase and cycle into levels each time a si
gnal is received.

```shell
kill -USR2 pid
```

Usefull if something is wrong and you want to have detailled log only for a small interval.

You can also set the loglevel using API endpoint /loglevel

* GET /loglevel : to retrieve the current level
* POST /loglevel : to cycle and increase the current loglevel
* POST /loglevel/\<level\> : to set level to \<level\>

## Reload

You can tell the exporter to reload its configuration by sending a signal HUP to the process or send a POST request to /reload endpoint.

## Exporter HTTP server

The exporter http server has a default landing page that permit to access :

* "/health" : a simple heartbeat page that return "OK" if exporter is UP
* "/config": expose defined configuration of the exporter
* "/targets": expose all known targets (locally defined or dynamically defined). Password are masked.
* "/targets/&lt;target&gt;": obtain configuration for target &lt;target&gt; or 404 Not found if doesn't exist. Password are masked.
* "/status": expose exporter version, process start time
* "/debug": expose exporter debug/profiling metrics
* "/sql_exporter_metrics": exporter internal prometheus metrics
* "/metrics": expose target's metrics.
* "/loglevel": GET exposes exporter current log level. POST /loglevel increases by one the current level (cycling). POST /loglevel/[level] set the new [level].
* "/reload": method POST only: tells the exporter to reload the configuration.

Reponse can be set to json by supplying a header "Accept: application/json" in the request.

### Prometheus scrapping

Prometheus scraps a target by geting the url /metrics or *metric_path if you redefine it by command line argument.

The entrypoint "/metrics" accepts the following argument:

* target=&lt;name&gt; [mandatory]: define the target to use:
  * a "name" defined locally in the exporter configuration.
  * a "definition" of target, that represents a data_source_name uri. In this case the target definition is based on the model parameter value, and if authentication is not set in the data_source_name, it should use the auth_name defined in configuration. If password  is encrypted, the shared key used to decipher must be speficied in auth_key.
* model=&lt;model&gt; (default="default")
* auth_name=&lt;auth_name&gt; the authentication parameters to use to connect with data_source_name
* auth_key=&lt;auth_key&gt; the shared key used to decipher encrypted password.
* health=&lt;true&gt; alter scraping behavior: only return the target connection status metrics; Use to determine if the connection to target is OK or not 1|0.
* collector=&lt;collector_name&gt;[&amp;collector=&lt;coll_name2&gt;&amp;...] alter scraping behavior; collect specific collectors list, instead of the default defined for the target; usefull to build a specific job with custom metrics with a different scraping interval by example.
