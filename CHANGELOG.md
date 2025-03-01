# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
## 0.9.2 / 2025-02-25
- fixed: label set uppercase on config: converted to lower case, both in config and in query results.
- fixed: panic when label name set for value is not found in query results.
- fixed: allow spaces between operator and pattern in collector list for target (e.g.: - ~ oracle_standard.*)
- fixed: add log message when error found during parsing of target or collector files.
- added: new parameter for /metric endpoint: collector. allow to collect target only for that collector list (&collector=name1&collector=nameX...)
- fixed: now constant labels set for targets are used.
- upgrade to go 1.24

## 0.9.1 / 2024-12-14

- add for dynamic target a validation of the dsn format, so that invalid one are rejected.
- add collector_status metric labeled with collector name.
- add config parameters in configuration file in global section for:
    - web.listen-address (priority to config file over command line argument --web.listen-address)
    - log.level (priority to config file over command line argument --log.level)
    - up_help allow user to replace default help message for metric help (default is "if the target is reachable 1, else 0 if the scrape failed")
    - scrape_duration_help same for scrap duration metric (default is "How long it took to scrape the target in seconds")
    - collector_status_help same for collector status (default is "collector scripts status 0: error - 1: ok - 2: Invalid login 3: Timeout")
- fixed bug with encrypted password and no auth_key provided.

## 0.9.0 / 2024-10-30

- removed passwd_encrypt tool source code from httpapi_exporter: created a new stand-alone package [passwd_encrypt](https://github.com/peekjef72/passwd_encrypt). Passwd_encrypt is still installed when building and added to the released archiv.
- updated prometheus/exporter-toolkit to 0.13.0 (log => log/slog)
- renamed entrypoint /healthz to /health : response format depends on "accept" header (application/json, text/plain, text/html default)
- updated entrypoint /status, /loglevel /targets /config: response format depends on "accept" header (application/json, text/plain, text/html default)
- added cmd line --model_name to perform test with model and uri in dry-run mode
- added out format for passwd_encrypt that can be cut/pasted into config file.
- added GET /loglevel to retrieve current level, add POST /loglevel[/level] to set loglevel to level directly 
- loglevel link in landing page
- fixed typos
- added oracledb building and metrics (standard and pdbs)

## 0.8.2 / 2024-05-05
* add proxy mode : allow to scrap remote servers using connection string as target
* add hanadb exporter and standard metrics.
* add basic encryption for user/passwords used to connect to database: password are not in plain text in configuration file.

## 0.8.1 / 2022-10-02
* added multi-targets
* add db2
* remove multi drivers compilations: use tags instead to have specific exporter for each one supported (mssql, db2, oracle)
* add contribs for standard statistics (mssql, db2)

## 0.5 
* forked from [mgit-at/sqlexporter](https://github.com/mgit-at/sql_exporter/blob/master/README.md)