# SQLRat Features - Copy, Paste, and Visual Selection

## Overview

Enhanced clipboard and selection features for efficient data manipulation in the SQL results table and editor.

---

## Results Table Features

### 1. Single Cell Copy (`y`)

**Usage:**
- Navigate to any cell using `hjkl` or arrow keys
- Press `y` to copy the cell value to system clipboard

**Behavior:**
- Copies the displayed value (respects staged edits)
- NULL values are copied as empty strings
- Also stores in internal clipboard for paste operations

---

### 2. Visual Selection Mode (`V`)

**Entering Visual Mode:**
- Press `V` to toggle visual selection mode
- The starting cell is marked as the anchor point
- Status bar shows: "hjkl Expand selection | y Copy | Esc Exit visual"

**Expanding Selection:**

**Horizontal (columns):**
- `h` or `left arrow` - Expand left
- `l` or `right arrow` - Expand right
- Row stays locked to anchor row when moving horizontally

**Vertical (rows):**
- `j` or `down arrow` - Expand down
- `k` or `up arrow` - Expand up
- Column stays locked to anchor column when moving vertically

**Block Selection:**
- Move both vertically and horizontally to select a rectangular block
- All cells within the rectangle are highlighted with gray background

**Visual Feedback:**
- Selected cells are highlighted with background color `#444444`
- Selected cells use white foreground `#ffffff` for visibility
- Visual selection overrides other cell styles (except cursor)

**Exiting Visual Mode:**
- Press `Esc` to exit without copying
- Press `V` again to toggle off
- Press `y` to copy and exit

---

### 3. Copy Selection (`y` in Visual Mode)

**Format:**
- **Single cell**: Plain text value
- **Single row (multiple columns)**: Tab-separated values (TSV)
- **Single column (multiple rows)**: Newline-separated values
- **Block (rows × columns)**: TSV format with newlines between rows

**Examples:**

```
Single cell:
user@example.com

Row (3 columns):
John	Doe	john@example.com

Column (3 rows):
John
Jane
Bob

Block (3 rows × 2 columns):
John	Doe
Jane	Smith
Bob	Johnson
```

**Clipboard Behavior:**
- Copies to both system clipboard and internal clipboard
- NULL values are copied as empty strings
- Compatible with Excel, Google Sheets, CSV editors

---

### 4. Paste from Clipboard (`p`)

**Requirements:**
- Cell must be editable (table has primary key OR it's a new row)
- Works with both system clipboard and internal clipboard

**Format Detection:**
- Tab characters (`\t`) separate columns
- Newline characters (`\n`) separate rows
- Automatically distributes data across cells starting from cursor

**Behavior:**
- Pastes starting at current cursor position
- Expands right for tab-separated values
- Expands down for newline-separated values
- Stops at table boundaries (won't create new columns/rows)
- Empty values are treated as NULL

**Examples:**

```
Paste single value:
Cursor at (0,0), paste "hello"
Result: Sets cell (0,0) to "hello"

Paste row:
Cursor at (0,1), paste "A\tB\tC"
Result: (0,1)="A", (0,2)="B", (0,3)="C"

Paste column:
Cursor at (1,0), paste "X\nY\nZ"
Result: (1,0)="X", (2,0)="Y", (3,0)="Z"

Paste block:
Cursor at (0,0), paste "A\tB\nC\tD\nE\tF"
Result:
(0,0)="A"  (0,1)="B"
(1,0)="C"  (1,1)="D"
(2,0)="E"  (2,1)="F"
```

**Change Tracking:**
- For existing rows: Creates staged edits (shows in yellow)
- For new rows: Directly modifies the row data
- Use `Ctrl+S` to commit changes
- Use `Ctrl+X` to discard changes

---

### 5. Paste in Edit Mode (`Ctrl+V`)

**Usage:**
- Press `e` to enter edit mode on a cell
- Press `Ctrl+V` to paste clipboard content into the edit buffer
- Status bar shows: "Type to edit | Ctrl+V Paste | Tab/Enter Next col"

**Behavior:**
- Appends clipboard content to current edit buffer
- Works with both system clipboard and internal clipboard
- Press `Enter` or `Tab` to commit and move to next cell
- Press `Esc` to cancel without saving

---

## SQL Editor Features

### 1. Copy All SQL (`Ctrl+Y`)

**Usage:**
- Press `Ctrl+Y` anywhere in the editor
- Copies entire SQL content to system clipboard

**Behavior:**
- Copies all text, preserving formatting
- Useful for sharing queries or backing up work
- Works regardless of cursor position

---

### 2. Paste SQL (`Ctrl+V`)

**Usage:**
- Press `Ctrl+V` in the editor
- Pastes clipboard content at cursor position

**Behavior:**
- Inserts text at current cursor location
- Preserves formatting (indentation, newlines)
- Triggers autocomplete/ghost text updates
- Can paste multi-line SQL queries

**Examples:**

```sql
-- Paste a full query:
SELECT * FROM users
WHERE created_at > '2024-01-01'
ORDER BY created_at DESC;

-- Paste partial snippet:
JOIN orders ON users.id = orders.user_id
```

---

## Status Bar Indicators

The status bar dynamically updates to show available commands:

**Normal Mode (Results Table):**
```
hjkl Navigate | e Edit | y Copy | p Paste | V Visual | d Delete | a Add
```

**Visual Mode:**
```
hjkl Expand selection | y Copy | Esc Exit visual | V Toggle visual
```

**Edit Mode:**
```
Type to edit | Ctrl+V Paste | Tab/Enter Next col | Shift+Tab Prev col | Esc Cancel
```

**SQL Editor:**
```
Ctrl+Y copy | Ctrl+V paste | Ctrl+J run | Ctrl+E all | Ctrl+O scripts
```

---

## Use Cases

### 1. Copy Data to Excel/Sheets

```
1. Navigate to desired cell or press V for range
2. Use hjkl to select multiple cells
3. Press y to copy
4. Paste into spreadsheet (auto-formats as table)
```

### 2. Import Data from CSV/Excel

```
1. Copy data from spreadsheet (Ctrl+C)
2. Navigate to target cell in SQLRat
3. Press p to paste
4. Press Ctrl+S to commit changes
```

### 3. Duplicate Rows

```
1. Press V on first cell of row
2. Press l repeatedly to select entire row
3. Press y to copy
4. Press a to add new row
5. Press p to paste data
```

### 4. Fill Column with Same Value

```
1. Type or paste value in one cell
2. Press y to copy
3. Navigate to each target cell and press p
```

### 5. Bulk Edit from External Tool

```
1. Select and copy data (V, hjkl, y)
2. Paste into text editor or spreadsheet
3. Make bulk edits
4. Copy edited data
5. Select original range (V, hjkl)
6. Press p to paste updated data
7. Press Ctrl+S to commit
```

### 6. SQL Query Reuse

```
1. Write query in editor
2. Press Ctrl+Y to copy
3. Share with teammates or save to notes
4. Later: Ctrl+V to paste back
```

---

## Implementation Details

### Clipboard Integration

Uses `github.com/atotto/clipboard` for system clipboard access:
- Works across different operating systems
- Supports UTF-8 text
- Graceful fallback to internal clipboard on errors

### Data Format

**Internal Format:**
- Tab-separated values (TSV) for multi-column
- Newline-separated for multi-row
- Plain text for single values

**NULL Handling:**
- Display: `<NULL>` in UI
- Copy: Empty string
- Paste: Empty string becomes `<NULL>`

### Change Tracking

Paste operations integrate with the existing change tracking system:
- Edits to existing rows create `CellEdit` entries
- New row edits modify row data directly
- All changes visible with color coding
- Undo with `Ctrl+Z`
- Commit with `Ctrl+S`

---

## Keyboard Reference Card

```
┌─────────────────────────────────────────────────────┐
│ COPY / PASTE / SELECT                               │
├─────────────────────────────────────────────────────┤
│ Results Table:                                      │
│   y        Copy cell or selection                   │
│   V        Toggle visual selection mode             │
│   p        Paste from clipboard                     │
│   e, then  Enter edit mode, then paste             │
│   Ctrl+V                                            │
│                                                     │
│ SQL Editor:                                         │
│   Ctrl+Y   Copy all SQL to clipboard               │
│   Ctrl+V   Paste from clipboard                    │
└─────────────────────────────────────────────────────┘
```
