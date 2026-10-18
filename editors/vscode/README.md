# Alca Language Support for Visual Studio Code & Cursor

Official Visual Studio Code and Cursor extension for the **Alca Programming Language** (`.al`).

## 🚀 Features

- **Syntax Highlighting**: TextMate grammar for the strict Alca syntax — keywords (`fn`, `foreach`, `match`, `case`, `ba`, …), stream operators (`>>`, `>!`, `>>>`, `?>>>`, `<<`, `|>`), declarations (`:=`, `->`, `T | null`), decorators (`@model`), built-in types (`int`, `list[T]`, `secret`, …), every standard module (`std.db`, `std.metrics`, …), built-in functions, durations (`500ms`, `2s`), sizes (`64KB`, `10MB`), and single/double/triple-quoted strings with escapes.
- **Auto-Indentation & Folding**: Indents 4 spaces after block headers ending with `:`, outdents `elif`/`else`/`case`, and folds `// region` / `// endregion` markers.
- **Live Diagnostics**: Runs `alca check` on the in-editor buffer (so unsaved edits are checked too) and highlights syntax and static type errors with red squiggles.
- **Formatting**: `Format Document` and `Alca: Format File` run `alca fmt`; formatting is idempotent and never rewrites a file with syntax errors.
- **Code Snippets**: Strict-syntax templates for functions, structs, variables (`:=`, `T | null`), if/elif/else, loops, `match`, baroutines (`ba`), errors (`err` / `?>>>`), threads, tests, SQLite (`std.db`), Prometheus metrics, and structured logging.
- **One-Click Commands**:
  - `Alca: Run Script` (interpreter)
  - `Alca: Run on Bytecode VM` (default engine, strict)
  - `Alca: Build Standalone Native Binary`
  - `Alca: Check Syntax & Types`
  - `Alca: Run Tests`
  - `Alca: Format File`
  - `Alca: Start REPL`

## 📦 Installation

### Quick Install (Local):
Copy the extension folder into your VS Code extensions directory:

**Windows PowerShell:**
```powershell
Copy-Item -Recurse -Force "editors\vscode" "$env:USERPROFILE\.vscode\extensions\alca-lang"
```

**Linux / macOS:**
```bash
cp -r editors/vscode ~/.vscode/extensions/alca-lang
```

Restart or reload VS Code (`Ctrl+Shift+P` -> `Developer: Reload Window`).

The extension expects the `alca` CLI on your `PATH` (`alca version` to verify).

## 📄 License

MIT License
