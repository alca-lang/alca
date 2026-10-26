// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

const vscode = require('vscode');
const cp = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

let diagnosticCollection;
let outputChannel;
let statusBar;
let languageClient = null;
let cliTimer = null;
let extensionContext = null;
let alcaVersion = '';
let alcaAvailable = true;

const diagnosticGenerations = new Map();

// settings returns the live Alca configuration section.
function settings() {
    return vscode.workspace.getConfiguration('alca');
}

// executable resolves the configured Alca CLI path.
function executable() {
    const configured = settings().get('executablePath', 'alca');
    const value = configured ? String(configured).trim() : '';
    return value || 'alca';
}

// shellArg quotes one argument for the integrated terminal.
function shellArg(value) {
    return '"' + String(value).replace(/"/g, '\\"') + '"';
}

// terminalCommand builds a command line for the integrated terminal.
function terminalCommand(args) {
    return [executable(), ...args].map(shellArg).join(' ');
}

function workspaceRoot() {
    const folders = vscode.workspace.workspaceFolders;
    return folders && folders.length > 0 ? folders[0].uri.fsPath : undefined;
}

function getOrCreateTerminal(name, cwd) {
    const existing = vscode.window.terminals.find((terminal) => terminal.name === name);
    if (existing) {
        return existing;
    }
    return vscode.window.createTerminal({ name, cwd });
}

function runInTerminal(name, args, cwd) {
    const terminal = getOrCreateTerminal(name, cwd);
    terminal.show();
    terminal.sendText(terminalCommand(args));
}

function fullDocumentRange(document) {
    return new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length));
}

// execFileResult wraps execFile so callers can inspect stdout, stderr and the
// spawn/exit error without using a shell.
function execFileResult(command, args, options) {
    return new Promise((resolve) => {
        cp.execFile(command, args, options || {}, (error, stdout, stderr) => {
            resolve({
                error,
                stdout: stdout || '',
                stderr: stderr || '',
            });
        });
    });
}

function stripAnsi(text) {
    return String(text).replace(/\x1b\[[0-9;]*m/g, '');
}

function delay(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

// documentForCommand resolves the document for a command invoked from the
// editor or the explorer. The optional save flag honours alca.saveBeforeRun.
async function documentForCommand(arg, save) {
    let document = null;
    if (arg && typeof arg.fsPath === 'string') {
        if (!arg.fsPath.endsWith('.al')) {
            vscode.window.showWarningMessage('Alca: expected an .al file.');
            return null;
        }
        try {
            document = await vscode.workspace.openTextDocument(arg);
        } catch (err) {
            vscode.window.showErrorMessage('Alca: cannot open file: ' + (err.message || err));
            return null;
        }
    } else if (vscode.window.activeTextEditor) {
        document = vscode.window.activeTextEditor.document;
    }

    if (!document || document.languageId !== 'alca') {
        vscode.window.showWarningMessage('Alca: open an Alca (.al) file first.');
        return null;
    }

    if (save !== false && settings().get('saveBeforeRun', true) && document.isDirty) {
        const saved = await document.save();
        if (!saved) {
            return null;
        }
    }
    return document;
}

async function runDocument(arg, mode) {
    const document = await documentForCommand(arg, true);
    if (!document) {
        return;
    }
    if (document.isUntitled) {
        vscode.window.showWarningMessage('Alca: save the file before running it.');
        return;
    }
    const args = mode === 'vm'
        ? ['run', '--vm', document.fileName]
        : ['run', '--interp', document.fileName];
    runInTerminal(mode === 'vm' ? 'Alca VM' : 'Alca', args, path.dirname(document.fileName));
}

async function buildDocument(arg) {
    const document = await documentForCommand(arg, true);
    if (!document) {
        return;
    }
    if (document.isUntitled) {
        vscode.window.showWarningMessage('Alca: save the file before building it.');
        return;
    }
    runInTerminal('Alca Build', ['build', document.fileName], path.dirname(document.fileName));
}

async function testDocument(arg) {
    const document = await documentForCommand(arg, true);
    if (!document) {
        return;
    }
    if (document.isUntitled) {
        vscode.window.showWarningMessage('Alca: save the file before running tests.');
        return;
    }
    runInTerminal('Alca Test', ['test', document.fileName], path.dirname(document.fileName));
}

function startREPL() {
    runInTerminal('Alca REPL', ['repl'], workspaceRoot());
}

// formatSource formats source text with the CLI formatter and resolves with
// the formatted text, or null when the source does not parse.
async function formatSource(text) {
    const tmp = path.join(os.tmpdir(), `alca-fmt-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}.al`);
    try {
        await fs.promises.writeFile(tmp, text, 'utf8');
        const result = await execFileResult(executable(), ['fmt', tmp], {
            maxBuffer: 16 * 1024 * 1024,
            windowsHide: true,
        });
        return result.error ? null : result.stdout;
    } catch (err) {
        outputChannel.appendLine('Alca formatter error: ' + (err.message || err));
        return null;
    } finally {
        try {
            await fs.promises.unlink(tmp);
        } catch (err) {
            // The temporary file may already be gone.
        }
    }
}

async function formatDocument(arg) {
    const document = await documentForCommand(arg, false);
    if (!document) {
        return;
    }
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

// parseDiagnostics turns the CLI's Rust-style reports into VS Code
// diagnostics. It keeps the exact reported position and uses a one-character
// range, which is more accurate than a fixed-width squiggle.
function parseDiagnostics(output, document) {
    const diagnostics = [];
    const seen = new Set();
    const lines = String(output).split(/\r?\n/);
    const header = /^(error|warning|note|help)(?:\[([^\]]+)\])?:\s*(.+)$/;

    for (let i = 0; i < lines.length; i++) {
        const match = header.exec(lines[i]);
        if (!match) {
            continue;
        }

        let location = null;
        for (let j = i + 1; j < Math.min(lines.length, i + 7); j++) {
            const found = /^\s*-->\s*(.+?):(\d+):(\d+)\s*$/.exec(lines[j]);
            if (found) {
                location = found;
                break;
            }
        }
        if (!location) {
            continue;
        }

        const line = Math.max(0, parseInt(location[2], 10) - 1);
        const column = Math.max(0, parseInt(location[3], 10) - 1);
        const lineText = line < document.lineCount ? document.lineAt(line).text : '';
        const start = Math.min(column, lineText.length);
        const end = Math.min(lineText.length, start + 1);
        const message = match[3].trim();
        const key = `${line}:${start}:${message}`;
        if (seen.has(key)) {
            continue;
        }
        seen.add(key);

        let severity = vscode.DiagnosticSeverity.Error;
        if (match[1] === 'warning') {
            severity = vscode.DiagnosticSeverity.Warning;
        } else if (match[1] === 'note') {
            severity = vscode.DiagnosticSeverity.Information;
        } else if (match[1] === 'help') {
            severity = vscode.DiagnosticSeverity.Hint;
        }

        const diagnostic = new vscode.Diagnostic(
            new vscode.Range(line, start, line, end),
            message,
            severity
        );
        diagnostic.source = 'alca';
        if (match[2]) {
            diagnostic.code = match[2];
        }
        diagnostics.push(diagnostic);
    }
    return diagnostics;
}

// runCLICheck checks the in-editor buffer (including unsaved edits) through a
// temporary file, so diagnostics do not depend on the file being saved.
async function runCLICheck(document) {
    const tmp = path.join(os.tmpdir(), `alca-check-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}.al`);
    try {
        await fs.promises.writeFile(tmp, document.getText(), 'utf8');
        const result = await execFileResult(executable(), ['check', tmp], {
            maxBuffer: 16 * 1024 * 1024,
            windowsHide: true,
        });
        const output = stripAnsi((result.stderr || '') + (result.stdout ? '\n' + result.stdout : ''));
        const failedToRun = Boolean(result.error) && (
            result.error.code === 'ENOENT' ||
            result.error.code === 'EACCES' ||
            result.error.code === 'EPERM'
        );
        const exitCode = result.error
            ? (typeof result.error.code === 'number' ? result.error.code : 1)
            : 0;
        return {
            diagnostics: document.isClosed ? [] : parseDiagnostics(output, document),
            output,
            exitCode,
            failedToRun,
        };
    } catch (err) {
        return {
            diagnostics: [],
            output: String(err && err.message ? err.message : err),
            exitCode: 1,
            failedToRun: true,
        };
    } finally {
        try {
            await fs.promises.unlink(tmp);
        } catch (err) {
            // The temporary file may already be gone.
        }
    }
}

async function updateDiagnosticsFromCLI(document, force) {
    if (!document || document.languageId !== 'alca' || document.isClosed) {
        return null;
    }
    if (!force && languageClient && languageClient.isReady()) {
        return null;
    }

    const key = document.uri.toString();
    const generation = (diagnosticGenerations.get(key) || 0) + 1;
    const version = document.version;
    diagnosticGenerations.set(key, generation);
    const result = await runCLICheck(document);
    if (diagnosticGenerations.get(key) !== generation) {
        return null;
    }
    if (document.isClosed) {
        return null;
    }
    if (!force && document.version !== version) {
        return null;
    }
    if (!force && languageClient && languageClient.isReady()) {
        return null;
    }
    diagnosticCollection.set(document.uri, result.diagnostics);
    return result;
}

function cancelCLIDiagnostics() {
    if (cliTimer) {
        clearTimeout(cliTimer);
        cliTimer = null;
    }
}

function scheduleCLIDiagnostics(document) {
    if (!document || document.languageId !== 'alca') {
        return;
    }
    if (languageClient && languageClient.isReady()) {
        return;
    }
    if (!settings().get('diagnostics.onType', true)) {
        return;
    }
    cancelCLIDiagnostics();
    const configuredDelay = Number(settings().get('diagnostics.delay', 600));
    const wait = Number.isFinite(configuredDelay) ? Math.max(200, configuredDelay) : 600;
    cliTimer = setTimeout(() => {
        cliTimer = null;
        updateDiagnosticsFromCLI(document);
    }, wait);
}

async function refreshAllDiagnostics() {
    for (const document of vscode.workspace.textDocuments) {
        if (document.languageId === 'alca' && !document.isClosed) {
            await updateDiagnosticsFromCLI(document, true);
        }
    }
}

async function checkDocument(arg) {
    const document = await documentForCommand(arg, true);
    if (!document) {
        return;
    }
    const result = await updateDiagnosticsFromCLI(document, true);
    if (!result) {
        return;
    }
    if (result.failedToRun) {
        outputChannel.clear();
        outputChannel.appendLine(result.output);
        outputChannel.show(true);
        vscode.window.showErrorMessage('Alca: cannot run the Alca CLI. Check alca.executablePath.');
        return;
    }
    if (result.diagnostics.length === 0 && result.exitCode === 0) {
        vscode.window.showInformationMessage('Alca: no syntax or type errors.');
        return;
    }
    outputChannel.clear();
    outputChannel.appendLine(result.output);
    outputChannel.show(true);
    vscode.window.showWarningMessage(`Alca: ${result.diagnostics.length} issue(s) found.`);
}

function handleLSPDiagnostics(params) {
    if (!params || !params.uri) {
        return;
    }
    const uri = vscode.Uri.parse(params.uri);
    const key = uri.toString();
    diagnosticGenerations.set(key, (diagnosticGenerations.get(key) || 0) + 1);
    cancelCLIDiagnostics();

    const items = (params.diagnostics || []).map((item) => {
        const range = item.range || {};
        const start = range.start || { line: 0, character: 0 };
        const end = range.end || start;
        let severity = vscode.DiagnosticSeverity.Error;
        if (item.severity === 2) {
            severity = vscode.DiagnosticSeverity.Warning;
        } else if (item.severity === 3) {
            severity = vscode.DiagnosticSeverity.Information;
        } else if (item.severity === 4) {
            severity = vscode.DiagnosticSeverity.Hint;
        }
        const diagnostic = new vscode.Diagnostic(
            new vscode.Range(start.line, start.character, end.line, end.character),
            item.message || 'Alca error',
            severity
        );
        diagnostic.source = item.source || 'alca';
        if (item.code !== undefined && item.code !== null) {
            diagnostic.code = item.code;
        }
        return diagnostic;
    });
    diagnosticCollection.set(uri, items);
}

// AlcaLanguageClient is a small dependency-free LSP client. The extension
// deliberately has no npm dependencies, so it speaks Content-Length framed
// JSON-RPC directly to `alca lsp`.
class AlcaLanguageClient {
    constructor(executablePath) {
        this.executable = executablePath;
        this.child = null;
        this.buffer = Buffer.alloc(0);
        this.nextId = 1;
        this.pending = new Map();
        this.ready = false;
        this.stopping = false;
        this.onDiagnostics = null;
        this.onStateChange = null;
    }

    isReady() {
        return this.ready;
    }

    async start() {
        if (this.child) {
            return;
        }

        const child = cp.spawn(this.executable, ['lsp'], {
            stdio: ['pipe', 'pipe', 'pipe'],
            windowsHide: true,
        });
        this.child = child;

        child.stdout.on('data', (chunk) => this.receive(chunk));
        child.stderr.on('data', (chunk) => outputChannel.append(chunk.toString()));
        child.stdin.on('error', (err) => this.fail(err));
        child.on('error', (err) => this.fail(err));
        child.on('close', (code, signal) => {
            if (this.child === child) {
                this.handleExit(code, signal);
            }
        });

        const root = workspaceRoot();
        await this.request('initialize', {
            processId: process.pid,
            clientInfo: { name: 'alca-vscode', version: '1.1.0' },
            rootUri: root ? vscode.Uri.file(root).toString() : null,
            capabilities: {
                textDocument: {
                    synchronization: { dynamicRegistration: false },
                    completion: { completionItem: { snippetSupport: false } },
                    hover: { contentFormat: ['markdown', 'plaintext'] },
                    publishDiagnostics: { relatedInformation: false },
                },
                workspace: {},
            },
        });
        this.sendNotification('initialized', {});
        this.ready = true;
        if (this.onStateChange) {
            this.onStateChange(true);
        }
    }

    request(method, params) {
        if (!this.child) {
            return Promise.reject(new Error('Alca language server is not running.'));
        }
        const id = this.nextId++;
        return new Promise((resolve, reject) => {
            this.pending.set(id, { resolve, reject });
            try {
                this.send({ jsonrpc: '2.0', id, method, params });
            } catch (err) {
                this.pending.delete(id);
                reject(err);
            }
        });
    }

    sendNotification(method, params) {
        if (!this.child) {
            return;
        }
        try {
            this.send({ jsonrpc: '2.0', method, params });
        } catch (err) {
            this.fail(err);
        }
    }

    send(message) {
        if (!this.child || !this.child.stdin || this.child.stdin.destroyed) {
            throw new Error('Alca language server is not running.');
        }
        const data = Buffer.from(JSON.stringify(message), 'utf8');
        this.child.stdin.write(`Content-Length: ${data.length}\r\n\r\n`);
        this.child.stdin.write(data);
    }

    receive(chunk) {
        this.buffer = Buffer.concat([this.buffer, chunk]);
        while (true) {
            const headerEnd = this.buffer.indexOf('\r\n\r\n');
            if (headerEnd < 0) {
                return;
            }
            const header = this.buffer.slice(0, headerEnd).toString('ascii');
            const lengthMatch = /Content-Length:\s*(\d+)/i.exec(header);
            if (!lengthMatch) {
                this.buffer = this.buffer.slice(headerEnd + 4);
                continue;
            }
            const length = parseInt(lengthMatch[1], 10);
            const bodyStart = headerEnd + 4;
            if (this.buffer.length < bodyStart + length) {
                return;
            }
            const body = this.buffer.slice(bodyStart, bodyStart + length).toString('utf8');
            this.buffer = this.buffer.slice(bodyStart + length);
            let message;
            try {
                message = JSON.parse(body);
            } catch (err) {
                continue;
            }
            this.handleMessage(message);
        }
    }

    handleMessage(message) {
        if (message.method && message.id !== undefined) {
            try {
                this.send({ jsonrpc: '2.0', id: message.id, result: null });
            } catch (err) {
                // The server is going away.
            }
            return;
        }

        if (message.method) {
            if (message.method === 'textDocument/publishDiagnostics' && this.onDiagnostics) {
                this.onDiagnostics(message.params);
            } else if (message.method === 'window/logMessage' && message.params) {
                outputChannel.appendLine(message.params.message || '');
            } else if (message.method === 'window/showMessage' && message.params) {
                const text = message.params.message || '';
                if (message.params.type === 1) {
                    vscode.window.showErrorMessage(text);
                } else if (message.params.type === 2) {
                    vscode.window.showWarningMessage(text);
                } else {
                    outputChannel.appendLine(text);
                }
            }
            return;
        }

        if (message.id !== undefined) {
            const pending = this.pending.get(message.id);
            if (!pending) {
                return;
            }
            this.pending.delete(message.id);
            if (message.error) {
                pending.reject(new Error(message.error.message || 'Alca language server error'));
            } else {
                pending.resolve(message.result);
            }
        }
    }

    fail(err) {
        this.ready = false;
        this.rejectPending(err || new Error('Alca language server failed.'));
    }

    handleExit(code, signal) {
        this.ready = false;
        this.child = null;
        this.buffer = Buffer.alloc(0);
        const suffix = code !== null && code !== undefined ? `code ${code}` : `signal ${signal}`;
        this.rejectPending(new Error(`Alca language server exited (${suffix}).`));
        if (this.onStateChange) {
            this.onStateChange(false, { code, signal, unexpected: !this.stopping });
        }
    }

    rejectPending(err) {
        for (const pending of this.pending.values()) {
            pending.reject(err);
        }
        this.pending.clear();
    }

    open(document) {
        if (!this.ready || document.languageId !== 'alca' || document.isClosed) {
            return;
        }
        this.sendNotification('textDocument/didOpen', {
            textDocument: {
                uri: document.uri.toString(),
                languageId: 'alca',
                version: document.version,
                text: document.getText(),
            },
        });
    }

    change(document) {
        if (!this.ready || document.languageId !== 'alca' || document.isClosed) {
            return;
        }
        this.sendNotification('textDocument/didChange', {
            textDocument: {
                uri: document.uri.toString(),
                version: document.version,
            },
            contentChanges: [{ text: document.getText() }],
        });
    }

    close(document) {
        if (!this.ready || document.languageId !== 'alca') {
            return;
        }
        this.sendNotification('textDocument/didClose', {
            textDocument: { uri: document.uri.toString() },
        });
    }

    async stop() {
        if (!this.child) {
            return;
        }
        this.stopping = true;
        const child = this.child;
        try {
            await Promise.race([this.request('shutdown', null), delay(500)]);
        } catch (err) {
            // The server may already be gone.
        }
        this.sendNotification('exit', null);
        this.child = null;
        this.ready = false;
        this.rejectPending(new Error('Alca language server stopped.'));
        setTimeout(() => {
            try {
                child.kill();
            } catch (err) {
                // Already exited.
            }
        }, 250);
    }
}

function openAlcaDocuments(client) {
    for (const document of vscode.workspace.textDocuments) {
        if (document.languageId === 'alca' && !document.isClosed) {
            client.open(document);
        }
    }
}

async function startLanguageServer() {
    if (!settings().get('useLanguageServer', true)) {
        return false;
    }
    if (languageClient && languageClient.isReady()) {
        return true;
    }

    const client = new AlcaLanguageClient(executable());
    client.onDiagnostics = handleLSPDiagnostics;
    client.onStateChange = (ready, info) => {
        updateStatusBar();
        if (!ready && info && info.unexpected) {
            outputChannel.appendLine('Alca language server stopped unexpectedly; falling back to CLI diagnostics.');
            refreshAllDiagnostics();
        }
    };
    languageClient = client;

    try {
        await client.start();
        outputChannel.appendLine(`Alca language server started (${executable()} lsp).`);
        openAlcaDocuments(client);
        cancelCLIDiagnostics();
        updateStatusBar();
        return true;
    } catch (err) {
        outputChannel.appendLine('Alca language server unavailable: ' + (err.message || err));
        await client.stop();
        if (languageClient === client) {
            languageClient = null;
        }
        updateStatusBar();
        refreshAllDiagnostics();
        return false;
    }
}

async function stopLanguageServer() {
    cancelCLIDiagnostics();
    if (languageClient) {
        const client = languageClient;
        languageClient = null;
        await client.stop();
    }
    diagnosticCollection.clear();
    updateStatusBar();
}

async function restartLanguageServer() {
    await stopLanguageServer();
    if (!settings().get('useLanguageServer', true)) {
        await refreshAllDiagnostics();
        return false;
    }
    return startLanguageServer();
}

async function probeVersion() {
    const result = await execFileResult(executable(), ['version'], { windowsHide: true });
    alcaAvailable = !result.error;
    if (!result.error) {
        const text = String(result.stdout).trim();
        alcaVersion = text.replace(/^alca version\s+/i, '');
        outputChannel.appendLine(`Using ${text} (${executable()})`);
    }
    updateStatusBar();
}

function updateStatusBar() {
    if (!statusBar) {
        return;
    }
    if (!settings().get('showStatusBar', true)) {
        statusBar.hide();
        return;
    }
    const editor = vscode.window.activeTextEditor;
    if (!editor || editor.document.languageId !== 'alca') {
        statusBar.hide();
        return;
    }

    const base = alcaVersion ? `Alca ${alcaVersion}` : 'Alca';
    if (!alcaAvailable) {
        statusBar.text = '$(error) Alca';
        statusBar.tooltip = 'Alca CLI not found. Set alca.executablePath.';
        statusBar.show();
        return;
    }
    if (languageClient && languageClient.isReady()) {
        statusBar.text = '$(check) Alca';
        statusBar.tooltip = `${base} — language server active. Click to check.`;
    } else {
        statusBar.text = '$(terminal) Alca';
        statusBar.tooltip = `${base} — CLI diagnostics. Click to check.`;
    }
    statusBar.show();
}

function completionItemFromLSP(item) {
    const label = typeof item.label === 'string'
        ? item.label
        : (item.label && item.label.label ? item.label.label : '');
    const kind = typeof item.kind === 'number' && item.kind >= 1
        ? item.kind - 1
        : vscode.CompletionItemKind.Text;
    const completion = new vscode.CompletionItem(label, kind);
    if (item.detail) {
        completion.detail = String(item.detail);
    }
    if (item.documentation) {
        const documentation = item.documentation;
        if (typeof documentation === 'string') {
            completion.documentation = new vscode.MarkdownString(documentation);
        } else if (documentation.value) {
            completion.documentation = new vscode.MarkdownString(documentation.value);
        }
    }
    if (item.insertText) {
        completion.insertText = item.insertText;
    }
    if (item.sortText) {
        completion.sortText = item.sortText;
    }
    if (item.filterText) {
        completion.filterText = item.filterText;
    }
    return completion;
}

function hoverFromLSP(result) {
    if (!result || !result.contents) {
        return undefined;
    }
    const contents = result.contents;
    let markdown = '';
    if (typeof contents === 'string') {
        markdown = contents;
    } else if (Array.isArray(contents)) {
        markdown = contents.map((part) => {
            if (typeof part === 'string') {
                return part;
            }
            if (part && part.value) {
                return part.language ? `\`\`\`${part.language}\n${part.value}\n\`\`\`` : part.value;
            }
            return '';
        }).filter(Boolean).join('\n\n');
    } else if (contents.value) {
        markdown = contents.kind === 'markdown'
            ? contents.value
            : `\`\`\`\n${contents.value}\n\`\`\``;
    }
    return markdown ? new vscode.Hover(new vscode.MarkdownString(markdown)) : undefined;
}

function command(handler) {
    return async (...args) => {
        try {
            await handler(...args);
        } catch (err) {
            outputChannel.appendLine('Alca command error: ' + (err && err.stack ? err.stack : err));
            vscode.window.showErrorMessage('Alca: ' + (err && err.message ? err.message : err));
        }
    };
}

function activate(context) {
    extensionContext = context;
    diagnosticCollection = vscode.languages.createDiagnosticCollection('alca');
    outputChannel = vscode.window.createOutputChannel('Alca');
    statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    statusBar.command = 'alca.checkFile';
    context.subscriptions.push(diagnosticCollection, outputChannel, statusBar);

    context.subscriptions.push(
        vscode.commands.registerCommand('alca.runFile', command((arg) => runDocument(arg, 'interp'))),
        vscode.commands.registerCommand('alca.runFileVM', command((arg) => runDocument(arg, 'vm'))),
        vscode.commands.registerCommand('alca.buildFile', command(buildDocument)),
        vscode.commands.registerCommand('alca.testFile', command(testDocument)),
        vscode.commands.registerCommand('alca.checkFile', command(checkDocument)),
        vscode.commands.registerCommand('alca.fmtFile', command(formatDocument)),
        vscode.commands.registerCommand('alca.repl', command(startREPL)),
        vscode.commands.registerCommand('alca.showOutput', command(() => outputChannel.show(true))),
        vscode.commands.registerCommand('alca.restartLanguageServer', command(async () => {
            if (!settings().get('useLanguageServer', true)) {
                vscode.window.showWarningMessage('Alca: the language server is disabled in settings.');
                return;
            }
            await restartLanguageServer();
            vscode.window.showInformationMessage('Alca: language server restarted.');
        }))
    );

    context.subscriptions.push(
        vscode.languages.registerDocumentFormattingEditProvider('alca', {
            async provideDocumentFormattingEdits(document) {
                const formatted = await formatSource(document.getText());
                if (formatted === null || formatted === document.getText()) {
                    return [];
                }
                return [vscode.TextEdit.replace(fullDocumentRange(document), formatted)];
            },
        })
    );

    context.subscriptions.push(
        vscode.languages.registerCompletionItemProvider('alca', {
            async provideCompletionItems(document, position) {
                const client = languageClient;
                if (!client || !client.isReady()) {
                    return undefined;
                }
                try {
                    const result = await client.request('textDocument/completion', {
                        textDocument: { uri: document.uri.toString() },
                        position: { line: position.line, character: position.character },
                    });
                    const items = Array.isArray(result)
                        ? result
                        : (result && Array.isArray(result.items) ? result.items : []);
                    return items.map(completionItemFromLSP);
                } catch (err) {
                    outputChannel.appendLine('Alca completion error: ' + (err.message || err));
                    return undefined;
                }
            },
        }, '.', ':')
    );

    context.subscriptions.push(
        vscode.languages.registerHoverProvider('alca', {
            async provideHover(document, position) {
                const client = languageClient;
                if (!client || !client.isReady()) {
                    return undefined;
                }
                try {
                    const result = await client.request('textDocument/hover', {
                        textDocument: { uri: document.uri.toString() },
                        position: { line: position.line, character: position.character },
                    });
                    return hoverFromLSP(result);
                } catch (err) {
                    outputChannel.appendLine('Alca hover error: ' + (err.message || err));
                    return undefined;
                }
            },
        })
    );

    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument((document) => {
            if (document.languageId !== 'alca') {
                return;
            }
            updateStatusBar();
            if (languageClient && languageClient.isReady()) {
                languageClient.open(document);
            } else if (settings().get('diagnostics.onOpen', true)) {
                updateDiagnosticsFromCLI(document);
            }
        }),
        vscode.workspace.onDidChangeTextDocument((event) => {
            if (event.document.languageId !== 'alca') {
                return;
            }
            if (languageClient && languageClient.isReady()) {
                languageClient.change(event.document);
            } else {
                scheduleCLIDiagnostics(event.document);
            }
        }),
        vscode.workspace.onDidSaveTextDocument((document) => {
            if (document.languageId !== 'alca') {
                return;
            }
            if (!languageClient || !languageClient.isReady()) {
                if (settings().get('diagnostics.onSave', true)) {
                    updateDiagnosticsFromCLI(document);
                }
            }
        }),
        vscode.workspace.onDidCloseTextDocument((document) => {
            if (document.languageId !== 'alca') {
                return;
            }
            if (languageClient && languageClient.isReady()) {
                languageClient.close(document);
            }
            const key = document.uri.toString();
            diagnosticGenerations.set(key, (diagnosticGenerations.get(key) || 0) + 1);
            diagnosticCollection.delete(document.uri);
            updateStatusBar();
        }),
        vscode.window.onDidChangeActiveTextEditor(() => updateStatusBar()),
        vscode.workspace.onDidChangeConfiguration((event) => {
            if (!event.affectsConfiguration('alca')) {
                return;
            }
            if (event.affectsConfiguration('alca.executablePath') || event.affectsConfiguration('alca.useLanguageServer')) {
                restartLanguageServer().then(probeVersion);
            }
            if (event.affectsConfiguration('alca.showStatusBar')) {
                updateStatusBar();
            }
            if (event.affectsConfiguration('alca.diagnostics')) {
                if (!languageClient || !languageClient.isReady()) {
                    refreshAllDiagnostics();
                }
            }
        })
    );

    if (vscode.window.activeTextEditor && vscode.window.activeTextEditor.document.languageId === 'alca') {
        updateStatusBar();
    }

    probeVersion();
    startLanguageServer();
}

async function deactivate() {
    cancelCLIDiagnostics();
    if (languageClient) {
        const client = languageClient;
        languageClient = null;
        await client.stop();
    }
    if (diagnosticCollection) {
        diagnosticCollection.clear();
    }
}

module.exports = {
    activate,
    deactivate,
};
