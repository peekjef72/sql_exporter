// package db2_exporter
package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Query wraps a sql.Stmt and all the metrics populated from it. It helps extract keys and values from result rows.
type Query struct {
	config         *QueryConfig
	metricFamilies []*MetricFamily
	// columnTypes maps column names to the column type expected by metrics: key (string) or value (float64).
	columnTypes columnTypeMap
	fieldTypes  fieldTypeMap
	logContext  []interface{}
	logger      log.Logger

	conn *sql.DB
	stmt *sql.Stmt
}

type columnType int
type columnTypeMap map[string]columnType

const (
	columnTypeKey   = 1
	columnTypeValue = 2
)

type fieldType int
type fieldTypeMap map[string]fieldType

const (
	fieldTypeTime   = 1
	fieldTypeString = 2
	fieldTypeNumber = 3
)

// NullTime is an alias for sql.NullTime data type to catch NULL in result
type NullTime sql.NullTime

// Scan implements the Scanner interface for NullInt64
func (ni *NullTime) Scan(value interface{}) error {
	var i sql.NullTime
	if err := i.Scan(value); err != nil {
		return err
	}
	// if nil the make Valid false
	if reflect.TypeOf(value) == nil {
		*ni = NullTime{i.Time, false}
	} else {
		*ni = NullTime{i.Time, true}
	}
	return nil
}

// convert timestamp to number of milliseconds unix epoch
func (ni *NullTime) ToFloat() float64 {
	if !ni.Valid {
		return 0
	}
	return float64(ni.Time.UnixMilli())
}

// NewQuery returns a new Query that will populate the given metric families.
func NewQuery(
	logContext []interface{},
	logger log.Logger,
	qc *QueryConfig,
	metricFamilies ...*MetricFamily) (*Query, error) {
	logContext = append(logContext, "query", qc.Name)

	columnTypes := make(columnTypeMap)
	fieldTypes := make(fieldTypeMap)

	for _, mf := range metricFamilies {
		for _, kcol := range mf.config.KeyLabels {
			kcol = strings.ToLower(kcol)
			if err := setColumnType(logContext, kcol, columnTypeKey, columnTypes); err != nil {
				return nil, err
			}
		}
		for _, vcol := range mf.config.Values {
			vcol = strings.ToLower(vcol)
			if err := setColumnType(logContext, vcol, columnTypeValue, columnTypes); err != nil {
				return nil, err
			}
		}
	}

	q := Query{
		config:         qc,
		metricFamilies: metricFamilies,
		columnTypes:    columnTypes,
		fieldTypes:     fieldTypes,
		logContext:     logContext,
		logger:         logger,
	}
	return &q, nil
}

// setColumnType stores the provided type for a given column, checking for conflicts in the process.
func setColumnType(logContext []interface{}, columnName string, ctype columnType, columnTypes columnTypeMap) error {
	previousType, found := columnTypes[columnName]
	if found {
		if previousType != ctype {
			logContext = append(logContext, "errmsg", fmt.Sprintf("column %q used both as key and value", columnName))
			return fmt.Errorf("%s", logContext...)
		}
	} else {
		columnTypes[columnName] = ctype
	}
	return nil
}

//
// func (q *Query) CloseTmp(rows *sql.Rows) {
// 	level.Debug(q.logger).Log("msg", fmt.Sprintf("close rows %p", rows))
// 	rows.Close()
// }

// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
func (q *Query) Collect(
	ctx context.Context,
	conn *sql.DB,
	symbols_table map[string]interface{},
	ch chan<- Metric) {
	if ctx.Err() != nil {
		ch <- NewInvalidMetric(q.logContext, ctx.Err())
		return
	}
	rows, err := q.run(ctx, conn, symbols_table)
	if err != nil {
		// TODO: increment an error counter
		ch <- NewInvalidMetric(q.logContext, err)
		return
	}

	// level.Debug(q.logger).Log("msg", fmt.Sprintf("opened rows %p", rows))
	defer rows.Close()
	// defer q.CloseTmp(rows)

	dest, err := q.scanDest(rows)
	if err != nil {
		// TODO: increment an error counter
		ch <- NewInvalidMetric(q.logContext, err)
		return
	}
	for rows.Next() {
		row, err := q.scanRow(rows, dest)
		if err != nil {
			ch <- NewInvalidMetric(q.logContext, err)
			continue
		}
		for _, mf := range q.metricFamilies {
			mf.Collect(row, ch)
		}
	}
	if err1 := rows.Err(); err1 != nil {
		ch <- NewInvalidMetric(q.logContext, err1)
	}
}

// run executes the query on the provided database, in the provided context.
func (q *Query) run(
	ctx context.Context,
	conn *sql.DB,
	symbols_table map[string]interface{}) (*sql.Rows, error) {
	if q.conn != nil && q.conn != conn {
		panic(fmt.Sprintf("[%s] Expecting to always run on the same database handle", q.logContext))
	}

	if q.stmt == nil {
		var query string
		// check if query contains a Template or is raw sql.
		// check if {{ is present in string
		if strings.Contains(q.config.Query, "{{") {
			tmpl, err := template.New("sql").Parse(q.config.Query)
			if err != nil {
				var logCtxt []interface{}
				logCtxt = append(logCtxt, q.logContext...)
				logCtxt = append(logCtxt, "msg", "prepare query failed with invalid template")
				return nil, ErrorWrap(logCtxt, err)
			}
			b := new(strings.Builder)
			err = tmpl.Execute(b, &symbols_table)
			if err != nil {
				var logCtxt []interface{}
				logCtxt = append(logCtxt, q.logContext...)
				logCtxt = append(logCtxt, "msg", "prepare query failed with invalid template render")
				return nil, ErrorWrap(logCtxt, err)
			}
			query = b.String()
		} else {
			query = q.config.Query
		}
		stmt, err := conn.PrepareContext(ctx, query)
		if err != nil {
			var logCtxt []interface{}
			logCtxt = append(logCtxt, q.logContext...)
			logCtxt = append(logCtxt, "msg", "prepare query failed")
			return nil, ErrorWrap(logCtxt, err)
		}
		q.conn = conn
		q.stmt = stmt
	}
	rows, err := q.stmt.QueryContext(ctx)
	return rows, ErrorWrap(q.logContext, err)
}

// scanDest creates a slice to scan the provided rows into, with strings for keys, float64s for values and interface{}
// for any extra columns.
func (q *Query) scanDest(rows *sql.Rows) ([]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, ErrorWrap(q.logContext, err)
	}
	var fieldType []*sql.ColumnType
	if len(q.fieldTypes) == 0 {
		fieldType, err = rows.ColumnTypes()
		if err != nil {
			return nil, ErrorWrap(q.logContext, err)
		}
	}

	// Create the slice to scan the row into, with strings for keys and float64s for values.
	dest := make([]interface{}, 0, len(columns))
	have := make(map[string]bool, len(q.columnTypes))
	for i, column := range columns {
		// check the fields type
		column = strings.ToLower(column)

		if len(fieldType) > 0 {
			if fieldType[i].DatabaseTypeName() == "SQL_VARIANT" {
				q.fieldTypes[column] = fieldTypeString
			} else {
				st := fieldType[i].ScanType()
				if st != nil {
					switch st.Kind() {
					case reflect.String:
						q.fieldTypes[column] = fieldTypeString
					case reflect.ValueOf(time.Time{}).Kind():
						q.fieldTypes[column] = fieldTypeTime
					default:
						q.fieldTypes[column] = fieldTypeNumber
					}
				}
			}
		}

		switch q.columnTypes[column] {
		// collect value for label
		case columnTypeKey:
			dest = append(dest, new(string))
			have[column] = true
			// collect value for value
		case columnTypeValue:
			if q.fieldTypes[column] == fieldTypeTime {
				dest = append(dest, new(NullTime))
				// } else if q.fieldTypes[column] == fieldTypeTime {
			} else if q.fieldTypes[column] == fieldTypeString {
				dest = append(dest, new(string))
			} else {
				dest = append(dest, new(float64))
			}
			have[column] = true
		default:
			var logCtx []interface{}

			logCtx = append(logCtx, q.logContext...)
			if column == "" {
				logCtx = append(logCtx, "msg", fmt.Sprintf("Unnamed column %d returned by query", i))
				level.Warn(q.logger).Log(logCtx...)
			} else {
				logCtx = append(logCtx, "msg", fmt.Sprintf("Extra column %q returned by query", column))
				level.Warn(q.logger).Log(logCtx...)
			}
			dest = append(dest, new(interface{}))
		}
	}
	// Not all requested columns could be mapped, fail.
	if len(have) != len(q.columnTypes) {
		missing := make([]string, 0, len(q.columnTypes)-len(have))
		for c := range q.columnTypes {
			if !have[c] {
				missing = append(missing, c)
			}
		}
		return nil, ErrorWrap(q.logContext, fmt.Errorf("column(s) %q missing from query result", missing))
	}

	return dest, nil
}

// scanRow scans the current row into a map of column name to value, with string values for key columns and float64
// values for value columns, using dest as a buffer.
func (q *Query) scanRow(rows *sql.Rows, dest []interface{}) (map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, ErrorWrap(q.logContext, err)
	}

	// Scan the row content into dest.
	if err := rows.Scan(dest...); err != nil {
		var logCtxt []interface{}
		logCtxt = append(logCtxt, q.logContext...)
		logCtxt = append(logCtxt, "msg", "scanning of query result failed")
		return nil, ErrorWrap(logCtxt, err)
	}

	// Pick all values we're interested in into a map.
	result := make(map[string]interface{}, len(q.columnTypes))
	for i, column := range columns {
		column = strings.ToLower(column)
		switch q.columnTypes[column] {
		case columnTypeKey:
			result[column] = *dest[i].(*string)
		case columnTypeValue:
			if q.fieldTypes[column] == fieldTypeTime {
				result[column] = (*dest[i].(*NullTime)).ToFloat()
			} else if q.fieldTypes[column] == fieldTypeString {
				v := *dest[i].(*string)
				v64, err := strconv.ParseFloat(v, 64)
				if err != nil {
					v64 = 0
				}
				result[column] = v64
			} else {
				result[column] = *dest[i].(*float64)
			}
		}
	}
	return result, nil
}
