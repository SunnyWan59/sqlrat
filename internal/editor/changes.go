package editor

import (
	"fmt"
	"strings"
)

// OpType represents the type of a staged change.
type OpType int

const (
	OpEdit OpType = iota
	OpDelete
	OpInsert
)

// CellEdit represents a staged cell modification.
type CellEdit struct {
	TableName   string
	RowPKValues map[string]string
	ColumnName  string
	OldValue    string
	NewValue    string
}

// RowDelete represents a staged row deletion.
type RowDelete struct {
	TableName   string
	RowPKValues map[string]string
}

// RowInsert represents a staged row insertion.
type RowInsert struct {
	TableName string
	Values    map[string]string
}

// UndoEntry records an operation for undo.
type UndoEntry struct {
	Type   OpType
	Index  int // index within the respective slice
}

// ChangeTracker tracks all staged modifications before commit.
type ChangeTracker struct {
	Edits     []CellEdit
	Deletes   []RowDelete
	Inserts   []RowInsert
	undoStack []UndoEntry
}

// NewChangeTracker creates a new empty change tracker.
func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{}
}

// StageEdit adds a cell edit to staged changes.
func (ct *ChangeTracker) StageEdit(edit CellEdit) {
	// Check if there is already an edit for the same cell, and update it
	for i, e := range ct.Edits {
		if e.TableName == edit.TableName &&
			e.ColumnName == edit.ColumnName &&
			pkMatch(e.RowPKValues, edit.RowPKValues) {
			ct.Edits[i].NewValue = edit.NewValue
			return
		}
	}
	ct.Edits = append(ct.Edits, edit)
	ct.undoStack = append(ct.undoStack, UndoEntry{Type: OpEdit, Index: len(ct.Edits) - 1})
}

// StageDelete adds a row deletion to staged changes.
func (ct *ChangeTracker) StageDelete(del RowDelete) {
	ct.Deletes = append(ct.Deletes, del)
	ct.undoStack = append(ct.undoStack, UndoEntry{Type: OpDelete, Index: len(ct.Deletes) - 1})
}

// UnstageDelete removes a row deletion from staged changes.
func (ct *ChangeTracker) UnstageDelete(tableName string, pkValues map[string]string) {
	for i, d := range ct.Deletes {
		if d.TableName == tableName && pkMatch(d.RowPKValues, pkValues) {
			ct.Deletes = append(ct.Deletes[:i], ct.Deletes[i+1:]...)
			// Remove from undo stack too
			for j := len(ct.undoStack) - 1; j >= 0; j-- {
				if ct.undoStack[j].Type == OpDelete && ct.undoStack[j].Index == i {
					ct.undoStack = append(ct.undoStack[:j], ct.undoStack[j+1:]...)
					break
				}
			}
			return
		}
	}
}

// IsRowDeleted checks if a row is marked for deletion.
func (ct *ChangeTracker) IsRowDeleted(tableName string, pkValues map[string]string) bool {
	for _, d := range ct.Deletes {
		if d.TableName == tableName && pkMatch(d.RowPKValues, pkValues) {
			return true
		}
	}
	return false
}

// StageInsert adds a row insertion to staged changes.
func (ct *ChangeTracker) StageInsert(ins RowInsert) {
	ct.Inserts = append(ct.Inserts, ins)
	ct.undoStack = append(ct.undoStack, UndoEntry{Type: OpInsert, Index: len(ct.Inserts) - 1})
}

// Undo pops the last operation from the undo stack.
func (ct *ChangeTracker) Undo() {
	if len(ct.undoStack) == 0 {
		return
	}
	last := ct.undoStack[len(ct.undoStack)-1]
	ct.undoStack = ct.undoStack[:len(ct.undoStack)-1]

	switch last.Type {
	case OpEdit:
		if last.Index < len(ct.Edits) {
			ct.Edits = append(ct.Edits[:last.Index], ct.Edits[last.Index+1:]...)
		}
	case OpDelete:
		if last.Index < len(ct.Deletes) {
			ct.Deletes = append(ct.Deletes[:last.Index], ct.Deletes[last.Index+1:]...)
		}
	case OpInsert:
		if last.Index < len(ct.Inserts) {
			ct.Inserts = append(ct.Inserts[:last.Index], ct.Inserts[last.Index+1:]...)
		}
	}
}

// HasChanges returns whether there are any pending changes.
func (ct *ChangeTracker) HasChanges() bool {
	return len(ct.Edits) > 0 || len(ct.Deletes) > 0 || len(ct.Inserts) > 0
}

// PendingCount returns total count of pending operations.
func (ct *ChangeTracker) PendingCount() int {
	return len(ct.Edits) + len(ct.Deletes) + len(ct.Inserts)
}

// GenerateSQL generates parameterized SQL statements and their args.
// Order: INSERTs first, then UPDATEs, then DELETEs.
func (ct *ChangeTracker) GenerateSQL() ([]string, [][]interface{}) {
	var queries []string
	var allArgs [][]interface{}

	// INSERTs
	for _, ins := range ct.Inserts {
		if len(ins.Values) == 0 {
			continue
		}
		cols := make([]string, 0, len(ins.Values))
		placeholders := make([]string, 0, len(ins.Values))
		args := make([]interface{}, 0, len(ins.Values))
		i := 1
		for col, val := range ins.Values {
			cols = append(cols, fmt.Sprintf("%q", col))
			if val == "<NULL>" {
				placeholders = append(placeholders, "NULL")
			} else {
				placeholders = append(placeholders, fmt.Sprintf("$%d", i))
				args = append(args, val)
				i++
			}
		}
		q := fmt.Sprintf(`INSERT INTO %q (%s) VALUES (%s)`,
			ins.TableName,
			strings.Join(cols, ", "),
			strings.Join(placeholders, ", "))
		queries = append(queries, q)
		allArgs = append(allArgs, args)
	}

	// UPDATEs (edits)
	for _, edit := range ct.Edits {
		args := []interface{}{}
		var setClause string
		if edit.NewValue == "<NULL>" {
			setClause = fmt.Sprintf("%q = NULL", edit.ColumnName)
		} else {
			setClause = fmt.Sprintf("%q = $1", edit.ColumnName)
			args = append(args, edit.NewValue)
		}

		whereParts := make([]string, 0, len(edit.RowPKValues))
		paramIdx := len(args) + 1
		for col, val := range edit.RowPKValues {
			if val == "<NULL>" {
				whereParts = append(whereParts, fmt.Sprintf("%q IS NULL", col))
			} else {
				whereParts = append(whereParts, fmt.Sprintf("%q = $%d", col, paramIdx))
				args = append(args, val)
				paramIdx++
			}
		}

		q := fmt.Sprintf(`UPDATE %q SET %s WHERE %s`,
			edit.TableName,
			setClause,
			strings.Join(whereParts, " AND "))
		queries = append(queries, q)
		allArgs = append(allArgs, args)
	}

	// DELETEs
	for _, del := range ct.Deletes {
		args := make([]interface{}, 0, len(del.RowPKValues))
		whereParts := make([]string, 0, len(del.RowPKValues))
		i := 1
		for col, val := range del.RowPKValues {
			if val == "<NULL>" {
				whereParts = append(whereParts, fmt.Sprintf("%q IS NULL", col))
			} else {
				whereParts = append(whereParts, fmt.Sprintf("%q = $%d", col, i))
				args = append(args, val)
				i++
			}
		}
		q := fmt.Sprintf(`DELETE FROM %q WHERE %s`,
			del.TableName,
			strings.Join(whereParts, " AND "))
		queries = append(queries, q)
		allArgs = append(allArgs, args)
	}

	return queries, allArgs
}

// Clear removes all staged changes.
func (ct *ChangeTracker) Clear() {
	ct.Edits = nil
	ct.Deletes = nil
	ct.Inserts = nil
	ct.undoStack = nil
}

// GetCellEdit returns the new value for a cell if it has a staged edit.
func (ct *ChangeTracker) GetCellEdit(tableName string, pkValues map[string]string, columnName string) (string, bool) {
	for _, e := range ct.Edits {
		if e.TableName == tableName &&
			e.ColumnName == columnName &&
			pkMatch(e.RowPKValues, pkValues) {
			return e.NewValue, true
		}
	}
	return "", false
}

func pkMatch(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
