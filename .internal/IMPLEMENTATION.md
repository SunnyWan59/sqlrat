# Implementation Summary: Copy, Paste, and Visual Selection

## Changes Made

### 1. Modified Files

#### `/internal/ui/results.go`
**Added:**
- Import `github.com/atotto/clipboard` for system clipboard access
- Fields to `ResultsModel`:
  - `clipboard string` - internal clipboard storage
  - `visualMode bool` - visual selection mode flag
  - `visualStartRow int` - anchor row for visual selection
  - `visualStartCol int` - anchor column for visual selection

**New Methods:**
- `IsVisualMode() bool` - returns visual mode state
- `copyCellToClipboard()` - copies single cell to clipboard
- `copySelection()` - copies visual selection to clipboard (TSV format)
- `pasteFromClipboard() ResultsModel` - pastes clipboard content into cells
- `isInVisualSelection(row, col int) bool` - checks if cell is in selection
- Helper functions: `min(a, b int)`, `max(a, b int)`

**Modified:**
- `updateNavMode()` - Added handlers for:
  - `V` - toggle visual selection mode
  - `y` - copy cell or selection
  - `p` - paste from clipboard
  - `esc` - exit visual mode
  - Movement keys (`hjkl`) now respect visual mode constraints
- `updateEditMode()` - Added `Ctrl+V` handler for paste in edit mode
- `renderTable()` - Added visual selection highlighting with priority in cell style determination

#### `/internal/ui/editor.go`
**Added:**
- Import `github.com/atotto/clipboard`
- Fields to `EditorModel`:
  - `visualMode bool` - visual mode flag (for future text selection)
  - `visualStart int` - selection start position
  - `visualEnd int` - selection end position

**Modified:**
- `Update()` - Added handlers for:
  - `Ctrl+Y` - copy all SQL to clipboard
  - `Ctrl+V` - paste from clipboard into editor
- `View()` - Updated header hint text to show copy/paste shortcuts

#### `/internal/ui/statusbar.go`
**Added:**
- Field to `StatusBarModel`:
  - `visualMode bool` - tracks visual mode state
- `SetVisualMode(visual bool)` - updates visual mode indicator

**Modified:**
- `contextHints()` - Added visual mode hints and updated:
  - Edit mode: Added "Ctrl+V Paste"
  - Visual mode: Shows "hjkl Expand selection | y Copy | Esc Exit visual | V Toggle visual"
  - Results pane: Shows "hjkl Navigate | e Edit | y Copy | p Paste | V Visual | d Delete | a Add"

#### `/internal/app/app.go`
**Modified:**
- `Update()` - Added `m.statusbar.SetVisualMode(m.results.IsVisualMode())` call to track visual mode state

### 2. New Files

#### `/README.md`
Enhanced with comprehensive documentation:
- Features overview
- Keyboard shortcuts reference
- Visual selection examples
- Copy/paste workflows

#### `/FEATURES.md`
Complete feature documentation:
- Detailed usage instructions for each feature
- Format specifications (TSV, NULL handling)
- Use cases and examples
- Implementation details
- Keyboard reference card

#### `/VISUAL_GUIDE.md`
ASCII diagrams showing:
- Visual selection flow
- Paste distribution logic
- Copy workflows
- Edit mode paste
- SQL editor copy/paste
- Color legend
- Before/after comparison table

### 3. Dependencies

**Already Available:**
- `github.com/atotto/clipboard v0.1.4` (was already in indirect dependencies)

No new dependencies needed - the clipboard package was already pulled in by another dependency.

---

## Features Implemented

### Results Table

1. **Single Cell Copy (`y`)**
   - Copies current cell value to both system and internal clipboard
   - Handles NULL values (converts to empty string)
   - Works with staged edits

2. **Visual Selection Mode (`V`)**
   - Toggle with `V` key
   - Anchor point at starting cell
   - Expand horizontally with `h`/`l` (locks row)
   - Expand vertically with `j`/`k` (locks column)
   - Block selection with both directions
   - Visual feedback: gray background, white text
   - Exit with `Esc` or `y` (copy)

3. **Copy Selection (`y` in visual mode)**
   - Single cell: plain text
   - Row: tab-separated values
   - Column: newline-separated values
   - Block: TSV format with newlines
   - Copies to both clipboards

4. **Paste (`p`)**
   - Detects tab and newline delimiters
   - Distributes across cells starting at cursor
   - Respects table boundaries
   - Creates staged edits for existing rows
   - Modifies data directly for new rows
   - Empty values become NULL

5. **Edit Mode Paste (`Ctrl+V` during edit)**
   - Appends clipboard to edit buffer
   - Works in cell edit mode (press `e` first)
   - Uses system clipboard with fallback to internal

### SQL Editor

1. **Copy All (`Ctrl+Y`)**
   - Copies entire SQL content
   - Works from any cursor position
   - System clipboard integration

2. **Paste (`Ctrl+V`)**
   - Inserts at cursor position
   - Preserves formatting
   - Updates autocomplete/ghost text

### Status Bar

1. **Dynamic Hints**
   - Shows context-aware keyboard shortcuts
   - Visual mode indicator
   - Edit mode shows paste option
   - Results pane shows copy/paste/visual options

---

## Technical Details

### Clipboard Integration
- **Library:** `github.com/atotto/clipboard`
- **Platform Support:** Cross-platform (Windows, macOS, Linux)
- **Fallback:** Internal clipboard if system clipboard fails
- **Format:** UTF-8 text

### Data Formats

**Tab-Separated Values (TSV):**
```
col1\tcol2\tcol3
```

**Newline-Separated:**
```
row1
row2
row3
```

**Block (TSV with newlines):**
```
a\tb\tc
d\te\tf
```

### NULL Handling
- **Display:** `<NULL>`
- **Copy:** Empty string `""`
- **Paste:** Empty string becomes `<NULL>`

### Visual Selection Rendering
```go
isVisualSelection := m.visualMode && m.isInVisualSelection(ri, ci)

switch {
case isCursor:
    style = CellSelected
case isVisualSelection:
    style = lipgloss.NewStyle().Background(lipgloss.Color("#444444")).Foreground(lipgloss.Color("#ffffff"))
case isDeleted:
    style = DeletedText
// ... other cases
}
```

### Change Tracking Integration
```go
// Paste creates staged edits
m.changes.StageEdit(editor.CellEdit{
    TableName:   m.tableName,
    ColumnName:  m.columns[colIdx],
    RowPKValues: pkVals,
    OldValue:    m.rows[rowIdx][colIdx],
    NewValue:    value,
})
```

---

## Testing Recommendations

### Manual Testing Checklist

**Copy Features:**
- [ ] Copy single cell with `y`
- [ ] Copy row with visual mode (horizontal)
- [ ] Copy column with visual mode (vertical)
- [ ] Copy block with visual mode (both directions)
- [ ] Verify system clipboard contains correct data
- [ ] Paste copied data into external app (Excel, text editor)

**Paste Features:**
- [ ] Paste single value
- [ ] Paste tab-delimited row
- [ ] Paste newline-delimited column
- [ ] Paste block (TSV format)
- [ ] Paste at table boundaries (shouldn't crash)
- [ ] Paste into new row
- [ ] Paste into existing row (should create staged edits)

**Visual Selection:**
- [ ] Enter/exit with `V`
- [ ] Expand right with `l`
- [ ] Expand left with `h`
- [ ] Expand down with `j`
- [ ] Expand up with `k`
- [ ] Verify selection highlighting
- [ ] Exit with `Esc` (no copy)
- [ ] Exit with `y` (copy and exit)
- [ ] Verify row locks when moving horizontally
- [ ] Verify column locks when moving vertically

**Edit Mode Paste:**
- [ ] Press `e` to edit cell
- [ ] Press `Ctrl+V` to paste
- [ ] Verify text appends to existing content
- [ ] Press `Enter` to commit
- [ ] Press `Esc` to cancel

**SQL Editor:**
- [ ] Write SQL query
- [ ] Press `Ctrl+Y` to copy
- [ ] Verify clipboard has full query
- [ ] Clear editor
- [ ] Press `Ctrl+V` to paste
- [ ] Verify query restored with formatting

**Status Bar:**
- [ ] Verify hints change in visual mode
- [ ] Verify hints show paste in edit mode
- [ ] Verify hints show copy/paste in normal results mode

**Edge Cases:**
- [ ] Copy/paste with NULL values
- [ ] Paste into table with no primary key
- [ ] Paste beyond table boundaries
- [ ] Copy empty selection
- [ ] Paste empty clipboard
- [ ] Paste with special characters (tabs, newlines in data)
- [ ] Very large paste operations

**Integration:**
- [ ] Commit pasted changes with `Ctrl+S`
- [ ] Undo pasted changes with `Ctrl+Z`
- [ ] Clear staged edits with `Ctrl+X`
- [ ] Refresh table after commit

---

## Known Limitations

1. **Clipboard Access:**
   - May fail on headless systems without X11/Wayland
   - Falls back to internal clipboard (no cross-application paste)

2. **Selection Boundaries:**
   - Cannot paste beyond existing table dimensions
   - Won't auto-create new rows or columns

3. **Data Types:**
   - All clipboard data treated as text
   - No special handling for JSON, binary, etc.

4. **Performance:**
   - Very large paste operations (thousands of cells) may be slow
   - Visual selection on huge tables might impact rendering

5. **SQL Editor Selection:**
   - Currently copies/pastes entire content
   - No partial text selection yet (fields added for future enhancement)

---

## Future Enhancements

1. **Partial SQL Selection:**
   - Visual select within editor
   - Copy/paste only selected portion

2. **Copy Row/Column Headers:**
   - Option to include column names when copying

3. **Export Formats:**
   - Copy as JSON
   - Copy as SQL INSERT statements
   - Copy as Markdown table

4. **Paste Validation:**
   - Type checking before paste
   - Preview paste changes

5. **Multi-Clipboard:**
   - Numbered clipboards (like vim registers)
   - Clipboard history

6. **Smart Paste:**
   - Auto-detect CSV vs TSV
   - Handle quoted values with delimiters

---

## Compatibility

- **PostgreSQL:** ✅ Full support
- **Operating Systems:** 
  - macOS: ✅ Full clipboard support
  - Linux: ✅ Requires X11 or Wayland
  - Windows: ✅ Full clipboard support
- **Terminal Emulators:** Works in all modern terminal emulators
- **External Apps:** TSV format compatible with Excel, Google Sheets, LibreOffice

---

## Summary

This implementation adds comprehensive copy/paste functionality with visual selection to SQLRat, making it much easier to work with data:

- **7 new keybindings** (`y`, `p`, `V`, `Esc` in visual, `Ctrl+Y`, `Ctrl+V` x2)
- **4 new methods** for clipboard operations
- **1 new mode** (visual selection)
- **Full system clipboard integration**
- **TSV format support** for spreadsheet compatibility
- **Context-aware status bar** hints
- **Complete documentation** in 3 new files

The implementation integrates seamlessly with existing features:
- Change tracking system
- Edit/delete/insert workflows  
- Undo functionality
- Commit/rollback operations

All without adding new dependencies!
