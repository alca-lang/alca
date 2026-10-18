# Alca Language Support for Visual Studio Code & Cursor

Official Visual Studio Code and Cursor extension for the **Alca Programming Language** (`.al`).

## 🚀 Features

- **Syntax Highlighting**: Comprehensive TextMate grammar highlighting keywords, stream operators (`>>`, `>!`, `>>>`, `<<`, `|>` etc.), decorators (`@model`), built-in types, durations (`500ms`, `2s`), sizes (`64KB`, `10MB`), and strings with `${interpolation}`.
- **Auto-Indentation & Folding**: Python-like indentation behavior automatically indenting 4 spaces after lines ending with `:`.
- **Live Diagnostics**: Automatically runs `alca check` on file save, highlighting syntax and static type errors with red squiggles directly in the editor.
- **Code Snippets**: Quick templates for functions, structs, if-statements, while-loops, baroutines (`ba`), SQLite databases, Prometheus metrics, and test blocks.
- **One-Click Commands**:
  - `Alca: Run Script`
  - `Alca: Run on Bytecode VM` (High performance)
  - `Alca: Build Standalone Native Binary (.exe)`
  - `Alca: Check Syntax & Types`
  - `Alca: Run Tests`

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

## 📄 License
MIT License
