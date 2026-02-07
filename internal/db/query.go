package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// QueryResult holds the result of a SELECT-like query.
type QueryResult struct {
	Columns     []string
	ColumnTypes []string
	Rows        [][]string
	RowCount    int
	ExecTime    time.Duration
}

// ExecResult holds the result of a DML query.
type ExecResult struct {
	RowsAffected int64
	ExecTime     time.Duration
}

// isSelectLike returns true if the query returns rows.
func isSelectLike(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "EXPLAIN")
}

// ExecuteQuery runs a SQL query and returns either a QueryResult or ExecResult.
// The second return value indicates if it was a SELECT-like query.
func (d *DB) ExecuteQuery(sql string) (*QueryResult, *ExecResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return nil, nil, fmt.Errorf("empty query")
	}

	start := time.Now()

	if isSelectLike(trimmed) {
		return d.executeSelect(ctx, trimmed, start)
	}
	return d.executeDML(ctx, trimmed, start)
}

func (d *DB) executeSelect(ctx context.Context, sql string, start time.Time) (*QueryResult, *ExecResult, error) {
	rows, err := d.Conn.Query(ctx, sql)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	columnTypes := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = f.Name
		columnTypes[i] = oidToTypeName(f.DataTypeOID)
	}

	var resultRows [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, nil, err
		}
		row := make([]string, len(values))
		for i, v := range values {
			if v == nil {
				row[i] = "<NULL>"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	elapsed := time.Since(start)
	return &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypes,
		Rows:        resultRows,
		RowCount:    len(resultRows),
		ExecTime:    elapsed,
	}, nil, nil
}

func (d *DB) executeDML(ctx context.Context, sql string, start time.Time) (*QueryResult, *ExecResult, error) {
	tag, err := d.Conn.Exec(ctx, sql)
	if err != nil {
		return nil, nil, err
	}
	elapsed := time.Since(start)
	return nil, &ExecResult{
		RowsAffected: tag.RowsAffected(),
		ExecTime:     elapsed,
	}, nil
}

// oidToTypeName maps common PostgreSQL OIDs to human-readable type names.
func oidToTypeName(oid uint32) string {
	switch oid {
	case 16:
		return "bool"
	case 20:
		return "int8"
	case 21:
		return "int2"
	case 23:
		return "int4"
	case 25:
		return "text"
	case 700:
		return "float4"
	case 701:
		return "float8"
	case 1042:
		return "bpchar"
	case 1043:
		return "varchar"
	case 1082:
		return "date"
	case 1114:
		return "timestamp"
	case 1184:
		return "timestamptz"
	case 1700:
		return "numeric"
	case 2950:
		return "uuid"
	case 3802:
		return "jsonb"
	case 114:
		return "json"
	default:
		return fmt.Sprintf("oid:%d", oid)
	}
}
