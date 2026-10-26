# Editor Support & Tooling (IDE)

[English](editors_EN.md) | [Русский](editors_RU.md)

Alca provides first-class editor integration through the official **Language Server Protocol (LSP)** daemon and dedicated editor extensions for **Visual Studio Code**, **Cursor**, and **Zed**.

---

## 1. Visual Studio Code & Cursor Extension

Located in [`editors/vscode/`](../editors/vscode/).

### Features
- **Complete Syntax Highlighting**: Keywords, classes, inheritance, control flow, concurrency (`ba`, `threads`), pipelines (`|>`), stream redirects (`>>`, `>!`, `>>>`, `?>>>`), and test blocks.
- **Language Server Protocol (LSP)**: Real-time static diagnostics, auto-completion, hover type inspection, and signature help powered by `alca lsp`.
- **In-Editor Fallback Diagnostics**: If LSP is disabled or starting up, runs `alca check` on the active editor buffer to show compiler errors and warnings on the fly.
- **Auto-Formatting**: Integrated with `alca fmt` on save (`editor.formatOnSave`) or command (`Shift+Alt+F`).
- **Comprehensive Snippets**: Ready-to-use snippets for functions, classes, interfaces, match statements, error handling, Prometheus metrics, and SQLite queries.

### Installation

#### Windows (PowerShell):
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.vscode\extensions\alca-lang" -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force "editors\vscode" "$env:USERPROFILE\.vscode\extensions\alca-lang"
```

#### Linux / macOS:
```bash
cp -r editors/vscode ~/.vscode/extensions/alca-lang
```

After copying, reload VS Code (`Ctrl+Shift+P` → `Developer: Reload Window`).

### Configuration Settings

| Setting | Default | Description |
| --- | --- | --- |
| `alca.executablePath` | `"alca"` | Path to the Alca CLI executable. |
| `alca.useLanguageServer` | `true` | Enables real-time LSP daemon integration. |
| `alca.diagnostics.onOpen` | `true` | Run diagnostics when opening a file. |
| `alca.diagnostics.onSave` | `true` | Re-check types when saving a file. |
| `alca.diagnostics.onType` | `true` | Typecheck as you type (with debounce). |
| `alca.diagnostics.delay` | `600` | Delay (in ms) before checking after typing stops. |

---

## 2. Zed Editor Extension

Located in [`editors/zed/`](../editors/zed/).

Provides syntax highlighting, custom file icons (`.al`), folder icons, and theme integration for the high-performance Zed code editor.

### Installation
Copy or symlink `editors/zed` into your Zed extensions directory:
- Linux: `~/.local/share/zed/extensions/installed/alca`
- macOS: `~/Library/Application Support/Zed/extensions/installed/alca`
- Windows: `%LOCALAPPDATA%\Zed\extensions\installed\alca`

---

## 3. Language Server Protocol (`alca lsp`)

The Alca CLI includes an integrated LSP daemon:

```bash
alca lsp
```

Communicates over standard input/output (stdio) with any LSP-compliant editor (including Neovim, Helix, Emacs, Sublime Text, Kate).

### Capabilities Provided:
- **`textDocument/didOpen` & `textDocument/didChange`**: Live incremental parsing and AST verification.
- **`textDocument/publishDiagnostics`**: Immediate syntax and strict static type error reports with file, line, and column precision.
- **`textDocument/completion`**: Auto-completion for built-ins, standard library namespaces (`std.*`), imported symbols, struct fields, and class methods.
- **`textDocument/hover`**: Type signatures and documentation docstrings on symbol hover.
