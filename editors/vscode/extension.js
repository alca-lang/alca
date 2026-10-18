const vscode = require('vscode');
const cp = require('child_process');
const path = require('path');

let diagnosticCollection;

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
            terminal.sendText(`alca run "${filePath}"`);
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
            terminal.sendText(`alca vm "${filePath}"`);
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

    // Execute 'alca check <file>' in background
    cp.exec(`alca check "${filePath}"`, (err, stdout, stderr) => {
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
