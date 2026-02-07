# Persistent Database Connection Storage

## Architecture

```mermaid
flowchart TD
    Launch[App Launch] --> HasSaved{Saved connections exist?}
    HasSaved -->|Yes| Picker[Connection Picker Screen]
    HasSaved -->|No| Form[Connection Form]
    Picker -->|Select saved| Connect[Connect to DB]
    Picker -->|New connection| Form
    Picker -->|Delete connection| Picker
    Form -->|Success| Save[Save connection to config]
    Save --> MainTUI[Main TUI]
    Connect --> MainTUI
```

## Storage

- Config file at `~/.config/cli-sql/connections.json`
- Each connection stored as a JSON object with: `name`, `host`, `port`, `user`, `password`, `database`, and optionally a raw `uri` field
- Passwords stored in plaintext (per your preference)

### Example config shape:

```json
{
  "connections": [
    {
      "name": "local-dev",
      "host": "localhost",
      "port": "5432",
      "user": "postgres",
      "password": "secret",
      "database": "myapp_dev"
    },
    {
      "name": "staging",
      "uri": "postgres://user:pass@staging.example.com:5432/myapp"
    }
  ]
}
```

## New Files

**[`internal/config/config.go`](internal/config/config.go)** - New package handling:
- `SavedConnection` struct (name, host, port, user, password, database, uri)
- `Load()` / `Save()` functions that read/write `~/.config/cli-sql/connections.json`
- `Add()` / `Delete()` helpers

## Modified Files

**[`main.go`](main.go)** - Major changes to the startup flow:
1. New `pickerModel` Bubble Tea model that displays saved connections as a selectable list
2. On launch: load config, if saved connections exist show the picker; otherwise go straight to the connection form
3. Picker shows saved connections with arrow key navigation, Enter to connect, `n` for new connection, `d`/`x` to delete
4. After a successful connection via the form, auto-save it to the config (prompt for a name using a text input)
5. The existing `connectionModel` gets a small addition: after connecting, ask for a name to save under

The connection form (`connectionModel`) remains mostly the same -- it just gains a "save step" at the end that writes to config before handing off to the main TUI.
