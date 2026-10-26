# Alca Language Support for Visual Studio Code & Cursor

Official Visual Studio Code and Cursor extension for the **Alca Programming Language** (`.al`).

## 🚀 Features

- **Complete syntax highlighting**: every Alca keyword, including classes and inheritance (`class`, `extends`, `public`, `private`, `const`), control flow, loops, concurrency (`ba`, `threads`, `wait`, `watch`), tests (`test`, `mock`, `assert`), transactions (`tx`), errors (`err`), and imports.
- **Rich literal support**: single/double/triple-quoted strings, raw strings (`r"..."`, backticks), formatted strings (`f"Hello {name}!"`, `rf"..."`) with interpolation highlighting, escapes, durations (`500ms`, `2s`), sizes (`64KB`, `10MB`), numbers with separators, and all stream operators (`>>`, `>!`, `>>>`, `?>>>`, `<<`, `|>`).
- **LSP-powered editor support**: completion, hover, and live diagnostics from `alca lsp` (syntax errors, type errors, class members, standard modules, and built-ins).
- **CLI fallback diagnostics**: if the language server is unavailable, the extension runs `alca check` on the in-editor buffer so unsaved edits are checked too.
- **Formatting**: `Format Document` and `Alca: Format File` run `alca fmt`; formatting is idempotent and never rewrites a file with syntax errors.
- **Code snippets**: functions, classes, inheritance, methods, public/private/const fields, nested structs, lambdas, variables (`:=`, `T | null`), if/elif/else, loops, `match`, baroutines, errors (`err` / `?>>>`), threads, tests, SQLite (`std.db`), Prometheus metrics, and structured logging.
- **Status bar**: shows whether the language server or CLI diagnostics are active. Click it to check the current file.
- **Explorer and editor context menus** for running, testing, building, checking, and formatting `.al` files.

## 📋 Commands

- `Alca: Run Script (Interpreter)`
- `Alca: Run on Bytecode VM`
- `Alca: Build Standalone Executable`
- `Alca: Check Syntax & Types`
- `Alca: Run Tests`
- `Alca: Format File`
- `Alca: Start REPL`
- `Alca: Show Output`
- `Alca: Restart Language Server`

## ⚙️ Settings

| Setting | Default | Description |
| --- | --- | --- |
| `alca.executablePath` | `alca` | Path to the Alca CLI. Set an absolute path if `alca` is not on `PATH`. |
| `alca.useLanguageServer` | `true` | Use `alca lsp` for completion, hover, and live diagnostics. |
| `alca.diagnostics.onOpen` | `true` | Run CLI diagnostics when a file is opened (fallback mode). |
| `alca.diagnostics.onSave` | `true` | Run CLI diagnostics when a file is saved (fallback mode). |
| `alca.diagnostics.onType` | `true` | Run CLI diagnostics while typing (fallback mode). |
| `alca.diagnostics.delay` | `600` | Delay in milliseconds before CLI diagnostics run after a change. |
| `alca.saveBeforeRun` | `true` | Save before running, testing, checking, or building. |
| `alca.showStatusBar` | `true` | Show the Alca status bar item for `.al` files. |

## 📦 Installation

### Quick Install (Local)

Copy the extension folder into your VS Code extensions directory:

**Windows PowerShell:**
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.vscode\extensions\alca-lang" -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force "editors\vscode" "$env:USERPROFILE\.vscode\extensions\alca-lang"
```

**Linux / macOS:**
```bash
cp -r editors/vscode ~/.vscode/extensions/alca-lang
```

Reload VS Code (`Ctrl+Shift+P` → `Developer: Reload Window`).

The extension expects the `alca` CLI on your `PATH` (`alca version` to verify). If it is installed elsewhere, set `alca.executablePath`.

## 📄 License

MIT License
