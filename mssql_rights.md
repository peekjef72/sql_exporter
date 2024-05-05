
# MSSQL Rights

The default mssql_standart.collector.yml config file contains SQL queries to these objects:
* select `SERVERPROPERTY()`: 
usetAll users can query the server properties.

* `@@MAX_CONNECTIONS` : no right specified

* `sys.sysprocesses`: requires VIEW SERVER STATE permission on the server.

* `sys.dm_os_performance_counters`: requires VIEW SERVER STATE permission on the server.

* `sys.dm_os_sys_memory`: requires VIEW SERVER STATE permission on the server.

* `master.sys.databases`: requires VIEW ANY DATABASE at server level

* `sys.master_files`: requires VIEW ANY DEFINITION

* `sys.dm_exec_query_stats`: requires VIEW SERVER STATE (maybe VIEW DATABASE STATE)

* `sys.dm_exec_plan_attributes`: requires VIEW SERVER STATE (maybe VIEW DATABASE STATE)

* `msdb.dbo.sysjobs`, `msdb.dbo.sysjobservers`: require 
    * GRANT SELECT ON OBJECT::msdb.dbo.sysjobs TO user
    * GRANT SELECT ON OBJECT::msdb.dbo.sysjobservers TO user

# Conclusion
To allow your user to collect metrics with this file you have to setup these rights.

Script example for an already existing windows user.

```sql
use [master]
GO
CREATE LOGIN [your_user] FROM WINDOWS WITH DEFAULT_DATABASE=[master]
GO 
GRANT VIEW SERVER STATE TO [your_user]
GO 
use msdb
GO
grant select on msdb.dbo.sysjobs to [your_user];
grant select on msdb.dbo.sysjobservers to [your_user];
GO
```
##