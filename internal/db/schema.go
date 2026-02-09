package db

import (
	"context"
	"fmt"
	"time"
)

// ColumnInfo holds metadata about a table column.
type ColumnInfo struct {
	Name          string
	DataType      string
	IsNullable    string
	ColumnDefault *string
}

// ListDatabases returns all databases sorted by name.
func (d *DB) ListDatabases() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := d.Conn.Query(ctx, `
		SELECT datname
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}
	return databases, rows.Err()
}

// CopyDatabase creates a new database using an existing one as a template.
// PostgreSQL requires no active connections to the template, so if currently
// connected to the source database the method temporarily switches to "postgres".
func (d *DB) CopyDatabase(source, target string) error {
	previousDB := d.database
	if previousDB == source {
		if err := d.SwitchDatabase("postgres"); err != nil {
			return fmt.Errorf("switch to postgres: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sql := fmt.Sprintf(
		`CREATE DATABASE %q WITH TEMPLATE %q OWNER %q`,
		target, source, d.user,
	)
	_, err := d.Conn.Exec(ctx, sql)
	if err != nil {
		if previousDB == source {
			d.SwitchDatabase(previousDB)
		}
		return err
	}

	if previousDB == source {
		if err := d.SwitchDatabase(previousDB); err != nil {
			return fmt.Errorf("switch back to %s: %w", previousDB, err)
		}
	}
	return nil
}

// DropDatabase drops a database. If currently connected to it, switches to "postgres" first.
// After dropping, if we were on the dropped DB we stay on "postgres".
func (d *DB) DropDatabase(name string) error {
	wasOnTarget := d.database == name
	if wasOnTarget {
		if err := d.SwitchDatabase("postgres"); err != nil {
			return fmt.Errorf("switch to postgres: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sql := fmt.Sprintf(`DROP DATABASE %q`, name)
	_, err := d.Conn.Exec(ctx, sql)
	return err
}

// ListTables returns all public base tables sorted by name.
func (d *DB) ListTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := d.Conn.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// GetPrimaryKeys returns the primary key column names for a table.
func (d *DB) GetPrimaryKeys(tableName string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := d.Conn.Query(ctx, `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
		  AND tc.table_name = $1
		  AND tc.table_schema = 'public'
		ORDER BY kcu.ordinal_position
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pks []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		pks = append(pks, col)
	}
	return pks, rows.Err()
}

// GetColumns returns column metadata for a table.
func (d *DB) GetColumns(tableName string) ([]ColumnInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := d.Conn.Query(ctx, `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_name = $1
		  AND table_schema = 'public'
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, &c.DataType, &c.IsNullable, &c.ColumnDefault); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}
