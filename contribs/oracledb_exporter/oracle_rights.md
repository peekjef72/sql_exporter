
# ORACLE Rights

The default oracle_standard_basis.yml config file contains SQL queries to these objects:
* `SYS.v_$parameter`

* `SYS.v_$instance`

* `SYS.v_$database`

* `SYS.v_$session`

* `SYS.v_$sysstat`

* `SYS.v_$sesstat`

* `SYS.v_$metric`

* `SYS.v_$undostat`

* `SYS.v_$session_event`

* `SYS.v_$waitclassmetric`

* `SYS.v_$system_wait_class`

* `SYS.DBA_DATA_FILES`

* `SYS.DBA_FREE_SPACE`

* `SYS.DBA_TEMP_FILES`

* `SYS.DBA_LMT_FREE_SP`

* `SYS.DBA_TEMP_FREE_SPACE`

* `SYS.DBA_TABLESPACES`

* `SYS.v_$tablespace`

* `SYS.v_$sysmetric`

* `SYS.v_$log_history`

module PERFSTAT must be installed on instance to collect metrics.

* `PERFSTAT.STATS$SYSTEM_EVENT`

* `PERFSTAT.STATS$BG_EVENT_SUMMARY`

* `PERFSTAT.STATS$MEA_STAT_INSTANCE`


The default oracle_pdbs_basis.yml config file contains SQL queries to the same objects except PERFSTAT elements and :

* `SYS.v$system_event`

# Conclusion
To allow your user to collect metrics with this file you have to setup these rights.

Script example for an already existing db user.

## Standard

```sql
GRANT SELECT ON SYS.v_$parameter TO user_name;
GRANT SELECT ON SYS.v_$instance TO user_name;
GRANT SELECT ON SYS.v_$database TO user_name;
GRANT SELECT ON SYS.v_$session TO user_name;
GRANT SELECT ON SYS.v_$sysstat TO user_name;
GRANT SELECT ON SYS.v_$sesstat TO user_name;
GRANT SELECT ON SYS.v_$metric TO user_name;
GRANT SELECT ON SYS.v_$undostat TO user_name;
GRANT SELECT ON SYS.v_$session_event TO user_name;
GRANT SELECT ON SYS.v_$waitclassmetric TO user_name;
GRANT SELECT ON SYS.v_$system_wait_class TO user_name;
GRANT SELECT ON SYS.DBA_DATA_FILES TO user_name;
GRANT SELECT ON SYS.DBA_FREE_SPACE TO user_name;
GRANT SELECT ON SYS.DBA_TEMP_FILES TO user_name;
GRANT SELECT ON SYS.DBA_LMT_FREE_SPACE user_name;
GRANT SELECT ON SYS.DBA_TEMP_FREE_SPACE TO user_name;
GRANT SELECT ON SYS.DBA_TABLESPACES TO user_name;
GRANT SELECT ON SYS.v_$tablespace TO user_name;
GRANT SELECT ON SYS.v_$sysmetric TO user_name;
GRANT SELECT ON SYS.v_$log_history TO user_name;

GRANT SELECT ON "PERFSTAT"."STATS$SYSTEM_EVENT" TO user_name;
GRANT SELECT ON "PERFSTAT"."STATS$BG_EVENT_SUMMARY" TO user_name;
GRANT SELECT ON "PERFSTAT"."MEA_STAT_INSTANCE" TO user_name;
```

## PDBs

```sql
GRANT SELECT ON SYS.v_$parameter TO user_name;
GRANT SELECT ON SYS.v_$instance TO user_name;
GRANT SELECT ON SYS.v_$database TO user_name;
GRANT SELECT ON SYS.v_$session TO user_name;
GRANT SELECT ON SYS.v_$sysstat TO user_name;
GRANT SELECT ON SYS.v_$sesstat TO user_name;
GRANT SELECT ON SYS.v_$metric TO user_name;
GRANT SELECT ON SYS.v_$undostat TO user_name;
GRANT SELECT ON SYS.v_$session_event TO user_name;
GRANT SELECT ON SYS.v_$waitclassmetric TO user_name;
GRANT SELECT ON SYS.v_$system_wait_class TO user_name;
GRANT SELECT ON SYS.DBA_DATA_FILES TO user_name;
GRANT SELECT ON SYS.DBA_FREE_SPACE TO user_name;
GRANT SELECT ON SYS.DBA_TEMP_FILES TO user_name;
GRANT SELECT ON SYS.DBA_LMT_FREE_SPACE user_name;
GRANT SELECT ON SYS.DBA_TEMP_FREE_SPACE TO user_name;
GRANT SELECT ON SYS.DBA_TABLESPACES TO user_name;
GRANT SELECT ON SYS.v_$tablespace TO user_name;
GRANT SELECT ON SYS.v_$sysmetric TO user_name;
GRANT SELECT ON SYS.v_$log_history TO user_name;
GRANT SELECT ON SYS.v_$system_event TO user_name;
```
