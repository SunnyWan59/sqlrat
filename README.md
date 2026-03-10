# SQL Rat
CLI PSQL client. Why? Cause fuck DBeaver

## Installation

```bash
go install github.com/SunnyWan59/sqlrat@latest
```

Or build from source:

```bash
git clone https://github.com/SunnyWan59/sqlrat.git
cd sqlrat
go build -o sqlrat
```

## Features

### Claude AI Integration

Ask Claude for SQL help directly in the editor:
- Press `Alt+I` (Option+I on Mac) in the editor to open the Claude modal
- Type what you want to do (e.g., "create a users table", "fix this query")
- Claude sees your current SQL and available tables
- Press Enter to get pure SQL code back
- The response replaces your editor content
- Set your API key in `.env`:
  ```
  ANTHROPIC_API_KEY=sk-ant-api03-...
  ```

### Copy, Paste, and Visual Selection

**Results Table:**
- **Copy Cell**: Press `y` on any cell to copy its value to clipboard
- **Visual Mode**: Press `V` to enter visual selection mode
  - Use `hjkl` or arrow keys to expand selection
  - `h`/`l` expands horizontally (columns)
  - `j`/`k` expands vertically (rows)
  - Press `y` to copy selection to clipboard (tab-delimited for columns, newline-delimited for rows)
  - Press `Esc` to exit visual mode
- **Paste**: Press `p` to paste clipboard content into cells
  - Pastes tab-delimited data across columns
  - Pastes newline-delimited data across rows
  - Works on both new rows and existing editable rows
- **Edit Mode**: Press `e` to edit a cell, then `Ctrl+V` to paste clipboard content into the edit field

**SQL Editor:**
- **Copy All SQL**: Press `Ctrl+Y` to copy all SQL in the editor to clipboard
- **Paste SQL**: Press `Ctrl+V` to paste clipboard content into editor

### Keyboard Shortcuts

**Navigation:**
- `Tab` / `Shift+Tab` - Cycle between panes (sidebar, editor, results)
- `hjkl` or arrow keys - Navigate within focused pane

**SQL Editor:**
- `Ctrl+J` - Execute SQL statement at cursor
- `Ctrl+E` - Execute all SQL in editor
- `Ctrl+Y` - Copy all SQL to clipboard
- `Ctrl+V` - Paste from clipboard
- `Ctrl+O` - Open scripts modal
- `Alt+I` - Ask Claude AI for SQL help (opens modal)
- `Tab` - Accept autocomplete suggestion

**Results Table:**
- `e` - Edit cell
- `y` - Copy cell (or selection in visual mode)
- `p` - Paste from clipboard
- `V` - Toggle visual selection mode
- `d` - Delete row (toggle)
- `a` - Add new row
- `v` - Preview cell in large viewer
- `/` - Search/filter rows
- `n` / `N` - Next/previous search match
- `g` / `G` - Jump to top/bottom

**Sidebar:**
- `Enter` - Select table/database
- `/` - Search tables
- `D` - Toggle between tables and databases view
- `c` - Copy database (when in databases view)
- `x` - Drop database (when in databases view)

**Global:**
- `Ctrl+S` - Commit pending changes
- `Ctrl+X` - Clear all pending changes
- `Ctrl+R` - Reconnect to database
- `Ctrl+C` - Quit

### Visual Selection Examples

**Copy a single cell:**
```
1. Navigate to cell with hjkl
2. Press y
```

**Copy multiple columns:**
```
1. Press V to enter visual mode
2. Press l (or right arrow) to expand right
3. Press y to copy (copies tab-delimited)
```

**Copy multiple rows:**
```
1. Press V to enter visual mode
2. Press j (or down arrow) to expand down
3. Press y to copy (copies newline-delimited)
```

**Copy a block (multiple rows and columns):**
```
1. Press V to enter visual mode
2. Use hjkl to select desired range
3. Press y to copy (copies as TSV format)
```

**Paste data:**
```
1. Copy data from anywhere (Excel, CSV, etc.)
2. Navigate to target cell
3. Press p to paste
4. Data will be distributed across cells based on tabs and newlines
```
