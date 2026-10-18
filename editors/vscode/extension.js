const vscode = require('vscode');
const cp = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

let diagnosticCollection;

// formatSource formats Alca source text with the CLI formatter and resolves
// with the formatted text, or null when the source does not parse.
function formatSource(text) {
    return new Promise((resolve) => {
        const tmp = path.join(os.tmpdir(), `alca-fmt-${process.pid}-${Date.now()}.al`);
        fs.writeFile(tmp, text, (err) => {
            if (err) {
                resolve(null);
                return;
            }
            cp.exec(`alca fmt "${tmp}"`, { maxBuffer: 16 * 1024 * 1024 }, (ferr, stdout) => {
                fs.unlink(tmp, () => {});
                resolve(ferr ? null : stdout);
            });
        });
    });
}

// fullDocumentRange returns the range covering the whole document, used to
// replace it with formatted text.
function fullDocumentRange(document) {
    return new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length));
}

async function applyFormat(document) {
    const formatted = await formatSource(document.getText());
    if (formatted === null) {
        vscode.window.showWarningMessage('Alca: cannot format a file with syntax errors.');
        return;
    }
    if (formatted === document.getText()) {
        return;
    }
    const edit = new vscode.WorkspaceEdit();
    edit.replace(document.uri, fullDocumentRange(document), formatted);
    await vscode.workspace.applyEdit(edit);
}

function activate(context) {
    diagnosticCollection = vscode.languages.createDiagnosticCollection('alca');
    context.subscriptions.push(diagnosticCollection);

    // Register command: Run File (Interpreter)
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.runFile', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const filePath = editor.document.fileName;
            const terminal = getOrCreateTerminal('Alca');
            terminal.show();
            terminal.sendText(`alca run --interp "${filePath}"`);
        })
    );

    // Register command: Run File on Bytecode VM
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.runFileVM', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const filePath = editor.document.fileName;
            const terminal = getOrCreateTerminal('Alca VM');
            terminal.show();
            terminal.sendText(`alca run --vm "${filePath}"`);
        })
    );

    // Register command: Build Native Standalone Executable
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.buildFile', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const filePath = editor.document.fileName;
            const terminal = getOrCreateTerminal('Alca Build');
            terminal.show();
            terminal.sendText(`alca build "${filePath}"`);
        })
    );

    // Register command: Run Tests
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.testFile', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            const filePath = editor.document.fileName;
            const terminal = getOrCreateTerminal('Alca Test');
            terminal.show();
            terminal.sendText(`alca test "${filePath}"`);
        })
    );

    // Register command: Check Syntax & Types
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.checkFile', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor) return;
            runDiagnostics(editor.document);
        })
    );

    // Register command: Format File
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.fmtFile', () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor || editor.document.languageId !== 'alca') return;
            applyFormat(editor.document);
        })
    );

    // Register command: Start REPL
    context.subscriptions.push(
        vscode.commands.registerCommand('alca.repl', () => {
            const terminal = getOrCreateTerminal('Alca REPL');
            terminal.show();
            terminal.sendText('alca repl');
        })
    );

    // Register formatter: "Format Document" runs the Alca formatter.
    context.subscriptions.push(
        vscode.languages.registerDocumentFormattingEditProvider('alca', {
            provideDocumentFormattingEdits(document) {
                return formatSource(document.getText()).then((formatted) => {
                    if (formatted === null || formatted === document.getText()) {
                        return [];
                    }
                    return [vscode.TextEdit.replace(fullDocumentRange(document), formatted)];
                });
            },
        })
    );

    // Run diagnostics on file save & open
    context.subscriptions.push(
        vscode.workspace.onDidSaveTextDocument((doc) => {
            if (doc.languageId === 'alca') {
                runDiagnostics(doc);
            }
        })
    );

    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument((doc) => {
            if (doc.languageId === 'alca') {
                runDiagnostics(doc);
            }
        })
    );

    // Run diagnostics on active editor change
    if (vscode.window.activeTextEditor && vscode.window.activeTextEditor.document.languageId === 'alca') {
        runDiagnostics(vscode.window.activeTextEditor.document);
    }
}

function runDiagnostics(document) {
    const filePath = document.fileName;
    if (!filePath.endsWith('.al')) return;

    // Ask the CLI to type-check the in-editor text, so unsaved edits are
    // diagnosed too; fall back to the file on disk when the buffer is empty.
    const source = document.getText();
    const tmp = path.join(os.tmpdir(), `alca-check-${process.pid}-${Date.now()}.al`);
    fs.writeFile(tmp, source, (werr) => {
        const target = werr ? filePath : tmp;

        cp.exec(`alca check "${target}"`, (err, stdout, stderr) => {
            if (!werr) {
                fs.unlink(tmp, () => {});
            }
            const diagnostics = [];
            const output = (stderr || '') + '\n' + (stdout || '');

            // Parse compiler error reports:
            // error: <message>
            //   --> <file>:<line>:<col>
            const errorRegex = /error:\s*(.+?)\r?\n\s*-->\s*.*?:(\d+):(\d+)/g;
            let match;

            while ((match = errorRegex.exec(output)) !== null) {
                const message = match[1].trim();
                const line = Math.max(0, parseInt(match[2], 10) - 1);
                const col = Math.max(0, parseInt(match[3], 10) - 1);

                const range = new vscode.Range(line, col, line, col + 10);
                const diag = new vscode.Diagnostic(range, message, vscode.DiagnosticSeverity.Error);
                diag.source = 'alca';
                diagnostics.push(diag);
            }

            diagnosticCollection.set(document.uri, diagnostics);
        });
    });
}

function getOrCreateTerminal(name) {
    const existing = vscode.window.terminals.find(t => t.name === name);
    if (existing) return existing;
    return vscode.window.createTerminal(name);
}

function deactivate() {
    if (diagnosticCollection) {
        diagnosticCollection.clear();
    }
}

module.exports = {
    activate,
    deactivate
};
