# Visual Selection and Copy/Paste Flow

## Visual Selection Mode

```
┌─────────────────────────────────────────────────────────────┐
│ Normal Navigation Mode                                      │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press 'V'
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Visual Selection Mode (Anchor at current cell)             │
│                                                             │
│  ┌───┬───┬───┬───┐                                         │
│  │ A │ B │ C │ D │   Anchor: (0, 0)                       │
│  ├───┼───┼───┼───┤   Cursor: (0, 0)                       │
│  │   │   │   │   │                                         │
│  ├───┼───┼───┼───┤                                         │
│  │   │   │   │   │                                         │
│  └───┴───┴───┴───┘                                         │
│                                                             │
│  Status: "hjkl Expand selection | y Copy | Esc Exit"       │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press 'l' (right) twice
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Horizontal Selection                                        │
│                                                             │
│  ┌───┬───┬───┬───┐                                         │
│  │▓▓▓│▓▓▓│▓▓▓│ D │   Anchor: (0, 0)                       │
│  ├───┼───┼───┼───┤   Cursor: (0, 2)                       │
│  │   │   │   │   │   Selection: A, B, C                   │
│  ├───┼───┼───┼───┤                                         │
│  │   │   │   │   │                                         │
│  └───┴───┴───┴───┘                                         │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press 'j' (down) once
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Block Selection                                             │
│                                                             │
│  ┌───┬───┬───┬───┐                                         │
│  │▓▓▓│▓▓▓│▓▓▓│   │   Anchor: (0, 0)                       │
│  ├───┼───┼───┼───┤   Cursor: (1, 2)                       │
│  │▓▓▓│▓▓▓│▓▓▓│   │   Selection: 3 cols × 2 rows           │
│  ├───┼───┼───┼───┤                                         │
│  │   │   │   │   │                                         │
│  └───┴───┴───┴───┘                                         │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press 'y' to copy
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Copied to Clipboard (TSV format)                           │
│                                                             │
│  A\tB\tC                                                    │
│  E\tF\tG                                                    │
│                                                             │
│  Visual mode exits automatically                            │
└─────────────────────────────────────────────────────────────┘
```

## Paste Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Clipboard Content                                           │
│                                                             │
│  "hello\tworld\nfoo\tbar"                                   │
│                                                             │
│  Parsed as:                                                 │
│    Row 1: ["hello", "world"]                                │
│    Row 2: ["foo", "bar"]                                    │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Navigate to (1, 1), Press 'p'
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Before Paste                                                │
│                                                             │
│  ┌───┬───┬───┬───┐                                         │
│  │ A │ B │ C │ D │                                         │
│  ├───┼───┼───┼───┤                                         │
│  │ E │[F]│ G │ H │  ← Cursor at (1, 1)                    │
│  ├───┼───┼───┼───┤                                         │
│  │ I │ J │ K │ L │                                         │
│  └───┴───┴───┴───┘                                         │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Paste distributes data
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ After Paste                                                 │
│                                                             │
│  ┌───┬───────┬─────┬───┐                                   │
│  │ A │   B   │  C  │ D │                                   │
│  ├───┼───────┼─────┼───┤                                   │
│  │ E │ hello │world│ H │  ← (1,1)="hello" (1,2)="world"   │
│  ├───┼───────┼─────┼───┤                                   │
│  │ I │  foo  │ bar │ L │  ← (2,1)="foo"   (2,2)="bar"     │
│  └───┴───────┴─────┴───┘                                   │
│                                                             │
│  Modified cells shown in yellow (staged edits)             │
└─────────────────────────────────────────────────────────────┘
```

## Copy Single Cell

```
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Navigate to Cell                                    │
│                                                             │
│  ┌───────────┬──────┬──────┐                               │
│  │   Name    │ Age  │ City │                               │
│  ├───────────┼──────┼──────┤                               │
│  │   John    │  25  │  NYC │                               │
│  ├───────────┼──────┼──────┤                               │
│  │   Jane    │ [30] │  LA  │  ← Cursor here                │
│  └───────────┴──────┴──────┘                               │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press 'y'
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Copied to Clipboard                                 │
│                                                             │
│  Clipboard: "30"                                            │
│                                                             │
│  Ready to paste in:                                         │
│    - Another cell (press p)                                 │
│    - External app (Ctrl+V)                                 │
│    - Text editor                                            │
└─────────────────────────────────────────────────────────────┘
```

## Edit Mode Paste

```
┌─────────────────────────────────────────────────────────────┐
│ Step 1: Enter Edit Mode                                     │
│                                                             │
│  ┌──────────┬────────┐                                     │
│  │   Name   │  Email │                                     │
│  ├──────────┼────────┤                                     │
│  │   John   │  john█ │  ← Press 'e' to edit                │
│  └──────────┴────────┘                                     │
│                                                             │
│  Status: "Type to edit | Ctrl+V Paste | Tab Next"          │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Press Ctrl+V (clipboard: "@example.com")
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Step 2: Content Pasted                                      │
│                                                             │
│  ┌──────────┬─────────────────────┐                        │
│  │   Name   │        Email        │                        │
│  ├──────────┼─────────────────────┤                        │
│  │   John   │ john@example.com█   │                        │
│  └──────────┴─────────────────────┘                        │
│                                                             │
│  Press Enter to commit, Esc to cancel                       │
└─────────────────────────────────────────────────────────────┘
```

## SQL Editor Copy/Paste

```
┌─────────────────────────────────────────────────────────────┐
│ SQL Editor - Copy All                                       │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐ │
│  │ SELECT u.name, o.total                                │ │
│  │ FROM users u                                          │ │
│  │ JOIN orders o ON u.id = o.user_id                     │ │
│  │ WHERE o.created_at > '2024-01-01'                     │ │
│  │ ORDER BY o.total DESC;█                               │ │
│  └───────────────────────────────────────────────────────┘ │
│                                                             │
│  Press Ctrl+Y → Entire query copied to clipboard          │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Share with teammate
                          │ or save to notes
                          ▼
┌─────────────────────────────────────────────────────────────┐
│ Later: Paste Query Back                                     │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐ │
│  │ █                                                      │ │
│  │                                                        │ │
│  │                                                        │ │
│  └───────────────────────────────────────────────────────┘ │
│                                                             │
│  Press Ctrl+V → Full query pasted with formatting          │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐ │
│  │ SELECT u.name, o.total                                │ │
│  │ FROM users u                                          │ │
│  │ JOIN orders o ON u.id = o.user_id                     │ │
│  │ WHERE o.created_at > '2024-01-01'                     │ │
│  │ ORDER BY o.total DESC;█                               │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Color Legend

```
┌─────────────────────────────────────────────────────────────┐
│ Cell State Indicators                                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ▓▓▓  Visual Selection (gray bg, white fg)                 │
│                                                             │
│  [X]  Cursor (reverse video)                                │
│                                                             │
│  🟨   Modified Cell (yellow)                                │
│                                                             │
│  🟩   New Row (green)                                       │
│                                                             │
│  🟥   Deleted Row (red/strikethrough)                       │
│                                                             │
│  NULL <NULL> (dim gray)                                     │
│                                                             │
│  🔍   Search Match (highlighted)                            │
└─────────────────────────────────────────────────────────────┘
```

## Feature Comparison

```
╔═══════════════════════════════════════════════════════════════╗
║                    Before    →    After (New)                 ║
╠═══════════════════════════════════════════════════════════════╣
║ Copy single cell          ❌  →  ✅ Press 'y'                 ║
║ Copy multiple cells       ❌  →  ✅ Visual mode + 'y'         ║
║ Paste into cells          ❌  →  ✅ Press 'p'                 ║
║ Visual selection          ❌  →  ✅ Press 'V'                 ║
║ Paste in edit mode        ❌  →  ✅ Ctrl+V                    ║
║ Copy SQL to clipboard     ❌  →  ✅ Ctrl+Y in editor          ║
║ Paste SQL from clipboard  ❌  →  ✅ Ctrl+V in editor          ║
║ System clipboard support  ❌  →  ✅ Full integration          ║
║ TSV format support        ❌  →  ✅ Excel/Sheets compatible   ║
║ Multi-row/col paste       ❌  →  ✅ Automatic distribution    ║
╚═══════════════════════════════════════════════════════════════╝
```
