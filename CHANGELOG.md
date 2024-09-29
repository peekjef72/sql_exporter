# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
## 0.9.0 / not release
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