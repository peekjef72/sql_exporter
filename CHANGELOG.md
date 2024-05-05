# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->

## 0.8.2 2024-05-05
* add proxy mode : allow to scrap remote servers using connection string as target
* add hanadb exporter and standard metrics.
* add basic encryption for user/passwords used to connect to database: password are not in plain text in configuration file.

## 0.8.1 2022-10-02
* added multi-targets
* add db2
* remove multi drivers compilations: use tags instead to have specific exporter for each one supported (mssql, db2, oracle)
* add contribs for standard statistics (mssql, db2)

## 0.5 
* forked from [mgit-at/sqlexporter](https://github.com/mgit-at/sql_exporter/blob/master/README.md)