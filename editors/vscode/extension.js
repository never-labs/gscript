"use strict";

const vscode = require("vscode");
const path = require("path");

let lspClient;
let lspContext;
let lspOutput;

function activate(context) {
  lspContext = context;
  context.subscriptions.push(
    vscode.commands.registerCommand("leia.runFile", () => runCurrentFile("run")),
    vscode.commands.registerCommand("leia.formatFile", formatCurrentFile),
    vscode.commands.registerCommand("leia.testWorkspace", () => runWorkspaceCommand("test")),
    vscode.commands.registerCommand("leia.lintWorkspace", () => runWorkspaceCommand("lint")),
    vscode.commands.registerCommand("leia.checkWorkspace", () => runWorkspaceCommand("check")),
    vscode.commands.registerCommand("leia.previewSpec", previewSpec),
    vscode.commands.registerCommand("leia.restartLanguageServer", restartLanguageServer),
    vscode.commands.registerCommand("leia.evaluate.case", (uri, name) => runEvaluateCase(uri, name)),
    vscode.commands.registerCommand("leia.agent.run", (uri) => runFileURI(uri)),
    vscode.tasks.registerTaskProvider("leia", new LeiaTaskProvider())
  );
  startLanguageServer(context);
}

function deactivate() {
  if (lspClient) {
    lspClient.dispose();
    lspClient = undefined;
  }
  if (lspOutput) {
    lspOutput.dispose();
    lspOutput = undefined;
  }
  lspContext = undefined;
}

function executable() {
  return vscode.workspace.getConfiguration("leia").get("executable", "leia");
}

function lspExecutable() {
  const config = vscode.workspace.getConfiguration("leia");
  return config.get("languageServer.executable", "leia-lsp");
}

function lspEnabled() {
  return vscode.workspace.getConfiguration("leia").get("languageServer.enabled", true);
}

function workspaceFolder() {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) {
    vscode.window.showErrorMessage("Leia: open a workspace folder first.");
    return undefined;
  }
  return folders[0].uri.fsPath;
}

function activeLeiaFile() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.languageId !== "leia") {
    vscode.window.showErrorMessage("Leia: open a .leia file first.");
    return undefined;
  }
  return editor.document.uri.fsPath;
}

function shellQuote(value) {
  if (process.platform === "win32") {
    return `"${String(value).replace(/"/g, '\\"')}"`;
  }
  return `'${String(value).replace(/'/g, "'\\''")}'`;
}

function terminal() {
  const name = "Leia";
  const existing = vscode.window.terminals.find((item) => item.name === name);
  if (existing) {
    return existing;
  }
  return vscode.window.createTerminal(name);
}

function runInTerminal(command, cwd) {
  const term = terminal();
  term.show(true);
  if (cwd) {
    term.sendText(`cd ${shellQuote(cwd)}`);
  }
  term.sendText(command);
}

async function runCurrentFile(subcommand) {
  const file = activeLeiaFile();
  if (!file) {
    return;
  }
  await vscode.window.activeTextEditor.document.save();
  runInTerminal(`${shellQuote(executable())} ${subcommand} ${shellQuote(file)}`, workspaceFolder());
}

async function runFileURI(uri) {
  const parsed = typeof uri === "string" ? vscode.Uri.parse(uri) : uri;
  if (!parsed) {
    return;
  }
  const doc = await vscode.workspace.openTextDocument(parsed);
  await doc.save();
  runInTerminal(`${shellQuote(executable())} run ${shellQuote(parsed.fsPath)}`, workspaceFolder());
}

async function runEvaluateCase(uri, name) {
  const parsed = typeof uri === "string" ? vscode.Uri.parse(uri) : uri;
  if (!parsed) {
    return;
  }
  const doc = await vscode.workspace.openTextDocument(parsed);
  await doc.save();
  const args = ["evaluate"];
  if (name) {
    args.push("--filter", name);
  }
  args.push(parsed.fsPath);
  runInTerminal(shellJoin([executable(), ...args]), workspaceFolder());
}

async function formatCurrentFile() {
  const file = activeLeiaFile();
  if (!file) {
    return;
  }
  await vscode.window.activeTextEditor.document.save();
  runInTerminal(`${shellQuote(executable())} fmt --write ${shellQuote(file)}`, workspaceFolder());
}

function runWorkspaceCommand(subcommand) {
  const cwd = workspaceFolder();
  if (!cwd) {
    return;
  }
  runInTerminal(`${shellQuote(executable())} ${subcommand}`, cwd);
}

function previewSpec() {
  const cwd = workspaceFolder();
  if (!cwd) {
    return;
  }
  const config = vscode.workspace.getConfiguration("leia");
  const script = config.get("specPreviewScript", "scripts/spec_preview.py");
  const output = config.get("specPreviewOutput", "docs/spec/index.html");
  const command = `python3 ${shellQuote(path.join(cwd, script))} --output ${shellQuote(path.join(cwd, output))}`;
  runInTerminal(command, cwd);
  vscode.env.openExternal(vscode.Uri.file(path.join(cwd, output)));
}

class LeiaTaskProvider {
  provideTasks() {
    const cwd = workspaceFolder();
    if (!cwd) {
      return [];
    }
    return [
      leiaTask("Run Current File", { command: "run", file: "${file}" }, ["run", "${file}"], cwd),
      leiaTask("Test Workspace", { command: "test" }, ["test"], cwd),
      leiaTask("Format Current File", { command: "fmt", file: "${file}" }, ["fmt", "--write", "${file}"], cwd),
      leiaTask("Lint Workspace", { command: "lint" }, ["lint"], cwd),
      leiaTask("Check Workspace", { command: "check" }, ["check"], cwd),
      shellTask("Preview Spec", { command: "specPreview" }, specPreviewCommand(cwd), cwd)
    ];
  }

  resolveTask(taskToResolve) {
    const cwd = workspaceFolder();
    if (!cwd) {
      return undefined;
    }
    const definition = taskToResolve.definition;
    const args = taskArgs(definition, cwd);
    if (!args) {
      return undefined;
    }
    if (definition.command === "specPreview") {
      return shellTask(taskToResolve.name, definition, specPreviewCommand(cwd, args), cwd);
    }
    return leiaTask(taskToResolve.name, definition, args, cwd);
  }
}

function leiaTask(name, definition, args, cwd) {
  return shellTask(name, definition, [executable(), ...args], cwd);
}

function shellTask(name, definition, commandParts, cwd) {
  const execution = new vscode.ShellExecution(shellJoin(commandParts), { cwd });
  const t = new vscode.Task(definition, vscode.TaskScope.Workspace, name, "leia", execution);
  t.presentationOptions = {
    reveal: vscode.TaskRevealKind.Always,
    panel: vscode.TaskPanelKind.Shared
  };
  return t;
}

function taskArgs(definition, cwd) {
  const extra = Array.isArray(definition.args) ? definition.args : [];
  const target = definition.file || definition.path;
  switch (definition.command) {
    case "run":
      return ["run", target || "${file}", ...extra];
    case "test":
      return target ? ["test", target, ...extra] : ["test", ...extra];
    case "fmt":
      return ["fmt", "--write", target || "${file}", ...extra];
    case "lint":
      return target ? ["lint", target, ...extra] : ["lint", ...extra];
    case "check":
      return target ? ["check", target, ...extra] : ["check", ...extra];
    case "specPreview":
      return extra;
    default:
      return undefined;
  }
}

function specPreviewArgs(cwd) {
  const config = vscode.workspace.getConfiguration("leia");
  const script = config.get("specPreviewScript", "scripts/spec_preview.py");
  const output = config.get("specPreviewOutput", "docs/spec/index.html");
  return ["python3", path.join(cwd, script), "--output", path.join(cwd, output)];
}

function specPreviewCommand(cwd, extraArgs) {
  return [...specPreviewArgs(cwd), ...(extraArgs || [])];
}

function shellJoin(parts) {
  return parts.map(shellQuote).join(" ");
}

function startLanguageServer(context) {
	if (!lspEnabled()) {
		return;
	}
	const command = lspExecutable();

	const output = vscode.window.createOutputChannel("Leia Language Server");
  lspOutput = output;
  const child = require("child_process").spawn(command, [], {
    stdio: ["pipe", "pipe", "pipe"]
  });
  lspClient = new MinimalLanguageClient(child, output);
  context.subscriptions.push(lspClient);
}

function restartLanguageServer() {
  if (!lspEnabled()) {
    vscode.window.showInformationMessage("Leia language server is disabled.");
    return;
  }
  if (!lspContext) {
    vscode.window.showErrorMessage("Leia: extension context is not available.");
    return;
  }
  if (lspClient) {
    lspClient.dispose();
    lspClient = undefined;
  }
  if (lspOutput) {
    lspOutput.dispose();
    lspOutput = undefined;
  }
  startLanguageServer(lspContext);
  vscode.window.showInformationMessage("Leia language server restarted.");
}

class MinimalLanguageClient {
  constructor(child, output) {
    this.child = child;
    this.output = output;
    this.buffer = Buffer.alloc(0);
    this.nextID = 1;
    this.disposed = false;
    this.documents = new Map();
    this.disposables = [];
    this.child.stdout.on("data", (chunk) => this.handleData(chunk));
    this.child.stderr.on("data", (chunk) => output.append(chunk.toString()));
    this.child.on("error", (err) => {
      if (this.disposed) {
        return;
      }
      output.appendLine(`failed to start leia-lsp: ${err.message}`);
      this.failPending(err.message);
    });
    this.child.on("close", (code, signal) => {
      if (this.disposed) {
        return;
      }
      const suffix = signal ? `signal ${signal}` : `exit ${code}`;
      output.appendLine(`leia-lsp stopped (${suffix})`);
      this.failPending(`leia-lsp stopped (${suffix})`);
    });
    this.sendRequest("initialize", {
      processId: process.pid,
      rootUri: vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0]
        ? vscode.workspace.workspaceFolders[0].uri.toString()
        : null,
      capabilities: {}
    });
    this.sendNotification("initialized", {});
    this.disposables.push(
      vscode.workspace.onDidOpenTextDocument((doc) => this.didOpen(doc)),
      vscode.workspace.onDidChangeTextDocument((event) => this.didChange(event.document)),
      vscode.workspace.onDidCloseTextDocument((doc) => this.didClose(doc)),
      vscode.languages.registerHoverProvider("leia", {
        provideHover: (doc, pos) => this.provideHover(doc, pos)
      }),
      vscode.languages.registerDocumentSymbolProvider("leia", {
        provideDocumentSymbols: (doc) => this.provideDocumentSymbols(doc)
      }),
      vscode.languages.registerWorkspaceSymbolProvider({
        provideWorkspaceSymbols: (query) => this.provideWorkspaceSymbols(query)
      }),
      vscode.languages.registerCodeLensProvider("leia", {
        provideCodeLenses: (doc) => this.provideCodeLenses(doc)
      }),
      vscode.languages.registerInlayHintsProvider("leia", {
        provideInlayHints: (doc, range) => this.provideInlayHints(doc, range)
      }),
      vscode.languages.registerCompletionItemProvider("leia", {
        provideCompletionItems: (doc, pos) => this.provideCompletionItems(doc, pos)
      }),
      vscode.languages.registerDefinitionProvider("leia", {
        provideDefinition: (doc, pos) => this.provideDefinition(doc, pos)
      }),
      vscode.languages.registerReferenceProvider("leia", {
        provideReferences: (doc, pos, context) => this.provideReferences(doc, pos, context)
      }),
      vscode.languages.registerRenameProvider("leia", {
        provideRenameEdits: (doc, pos, newName) => this.provideRenameEdits(doc, pos, newName)
      }),
      vscode.languages.registerDocumentFormattingEditProvider("leia", {
        provideDocumentFormattingEdits: (doc) => this.provideFormatting(doc)
      })
    );
    for (const doc of vscode.workspace.textDocuments) {
      this.didOpen(doc);
    }
  }

	dispose() {
    this.disposed = true;
    this.failPending("leia-lsp client disposed");
		for (const disposable of this.disposables) {
			disposable.dispose();
		}
		this.disposables = [];
		if (this.collection) {
			this.collection.dispose();
			this.collection = undefined;
		}
		if (this.child && !this.child.killed) {
			this.child.kill();
		}
  }

  didOpen(doc) {
    if (doc.languageId !== "leia") {
      return;
    }
    this.documents.set(doc.uri.toString(), doc);
    this.sendNotification("textDocument/didOpen", {
      textDocument: {
        uri: doc.uri.toString(),
        languageId: "leia",
        version: doc.version,
        text: doc.getText()
      }
    });
  }

  didChange(doc) {
    if (doc.languageId !== "leia") {
      return;
    }
    this.documents.set(doc.uri.toString(), doc);
    this.sendNotification("textDocument/didChange", {
      textDocument: { uri: doc.uri.toString(), version: doc.version },
      contentChanges: [{ text: doc.getText() }]
    });
  }

  didClose(doc) {
    if (doc.languageId !== "leia") {
      return;
    }
    this.documents.delete(doc.uri.toString());
    this.sendNotification("textDocument/didClose", {
      textDocument: { uri: doc.uri.toString() }
    });
  }

  provideHover(doc, pos) {
    return this.request("textDocument/hover", lspPositionParams(doc, pos)).then((result) => {
      if (!result) {
        return undefined;
      }
      return new vscode.Hover(new vscode.MarkdownString(result.contents.value), lspRange(result.range));
    });
  }

  provideDocumentSymbols(doc) {
    return this.request("textDocument/documentSymbol", {
      textDocument: { uri: doc.uri.toString() }
    }).then((result) => (result || []).map((sym) =>
      new vscode.DocumentSymbol(
        sym.name,
        sym.detail || "",
        sym.kind || vscode.SymbolKind.Function,
        lspRange(sym.range),
        lspRange(sym.selectionRange)
      )
    ));
  }

  provideWorkspaceSymbols(query) {
    return this.request("workspace/symbol", { query }).then((result) =>
      (result || []).map((sym) =>
        new vscode.SymbolInformation(
          sym.name,
          sym.kind || vscode.SymbolKind.Function,
          sym.containerName || "",
          new vscode.Location(vscode.Uri.parse(sym.location.uri), lspRange(sym.location.range))
        )
      )
    );
  }

  provideCompletionItems(doc, pos) {
    return this.request("textDocument/completion", lspPositionParams(doc, pos)).then((result) =>
      (result || []).map((item) => {
        const completion = new vscode.CompletionItem(item.label, item.kind || vscode.CompletionItemKind.Keyword);
        completion.detail = item.detail;
        completion.insertText = item.insertText || item.label;
        return completion;
      })
    );
  }

  provideCodeLenses(doc) {
    return this.request("textDocument/codeLens", {
      textDocument: { uri: doc.uri.toString() }
    }).then((result) =>
      (result || []).map((lens) =>
        new vscode.CodeLens(lspRange(lens.range), lens.command ? {
          title: lens.command.title,
          command: lens.command.command,
          arguments: lens.command.arguments || []
        } : undefined)
      )
    );
  }

  provideInlayHints(doc, range) {
    return this.request("textDocument/inlayHint", {
      textDocument: { uri: doc.uri.toString() },
      range: {
        start: { line: range.start.line, character: range.start.character },
        end: { line: range.end.line, character: range.end.character }
      }
    }).then((result) =>
      (result || []).map((hint) => {
        const out = new vscode.InlayHint(
          new vscode.Position(hint.position.line, hint.position.character),
          hint.label,
          vscode.InlayHintKind.Type
        );
        out.tooltip = hint.tooltip;
        return out;
      })
    );
  }

  provideDefinition(doc, pos) {
    return this.request("textDocument/definition", lspPositionParams(doc, pos)).then((result) => {
      if (!result) {
        return undefined;
      }
      return new vscode.Location(vscode.Uri.parse(result.uri), lspRange(result.range));
    });
  }

  provideReferences(doc, pos, context) {
    return this.request("textDocument/references", {
      ...lspPositionParams(doc, pos),
      context: { includeDeclaration: context.includeDeclaration }
    }).then((result) =>
      (result || []).map((loc) => new vscode.Location(vscode.Uri.parse(loc.uri), lspRange(loc.range)))
    );
  }

  provideRenameEdits(doc, pos, newName) {
    return this.request("textDocument/rename", {
      ...lspPositionParams(doc, pos),
      newName
    }).then((result) => {
      const edit = new vscode.WorkspaceEdit();
      const changes = result && result.changes ? result.changes : {};
      for (const [uri, edits] of Object.entries(changes)) {
        for (const item of edits) {
          edit.replace(vscode.Uri.parse(uri), lspRange(item.range), item.newText);
        }
      }
      return edit;
    });
  }

  provideFormatting(doc) {
    return this.request("textDocument/formatting", {
      textDocument: { uri: doc.uri.toString() }
    }).then((result) =>
      (result || []).map((edit) => new vscode.TextEdit(lspRange(edit.range), edit.newText))
    );
  }

  request(method, params) {
    const id = this.nextID++;
    return new Promise((resolve, reject) => {
      this.pending = this.pending || new Map();
      this.pending.set(id, { resolve, reject });
      this.send({ jsonrpc: "2.0", id, method, params });
    });
  }

  sendRequest(method, params) {
    return this.request(method, params).catch((err) => {
      this.output.appendLine(err.message);
    });
  }

  sendNotification(method, params) {
    this.send({ jsonrpc: "2.0", method, params });
  }

  send(payload) {
    const body = Buffer.from(JSON.stringify(payload), "utf8");
    if (!this.child || this.child.killed || !this.child.stdin.writable) {
      throw new Error("leia-lsp is not running");
    }
    this.child.stdin.write(`Content-Length: ${body.length}\r\n\r\n`);
    this.child.stdin.write(body);
  }

  handleData(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    while (true) {
      const marker = this.buffer.indexOf("\r\n\r\n");
      if (marker < 0) {
        return;
      }
      const header = this.buffer.slice(0, marker).toString();
      const match = /Content-Length:\s*(\d+)/i.exec(header);
      if (!match) {
        this.buffer = this.buffer.slice(marker + 4);
        continue;
      }
      const length = Number(match[1]);
      const start = marker + 4;
      const end = start + length;
      if (this.buffer.length < end) {
        return;
      }
      const payload = JSON.parse(this.buffer.slice(start, end).toString());
      this.buffer = this.buffer.slice(end);
      this.handleMessage(payload);
    }
  }

  handleMessage(message) {
    if (message.method === "textDocument/publishDiagnostics") {
      this.publishDiagnostics(message.params);
      return;
    }
    if (message.id !== undefined && this.pending && this.pending.has(message.id)) {
      const pending = this.pending.get(message.id);
      this.pending.delete(message.id);
      if (message.error) {
        pending.reject(new Error(message.error.message));
      } else {
        pending.resolve(message.result);
      }
    }
  }

	publishDiagnostics(params) {
		if (!this.collection) {
			this.collection = vscode.languages.createDiagnosticCollection("leia");
		}
		const uri = vscode.Uri.parse(params.uri);
		const diagnostics = (params.diagnostics || []).map((diag) => {
			const item = new vscode.Diagnostic(lspRange(diag.range), diag.message, diagnosticSeverity(diag.severity));
			item.code = diag.code;
			item.source = diag.source || "leia";
			return item;
    });
    this.collection.set(uri, diagnostics);
  }

  failPending(message) {
    if (!this.pending) {
      return;
    }
    for (const pending of this.pending.values()) {
      pending.reject(new Error(message));
    }
    this.pending.clear();
  }
}

function lspPositionParams(doc, pos) {
  return {
    textDocument: { uri: doc.uri.toString() },
    position: { line: pos.line, character: pos.character }
  };
}

function lspRange(range) {
	return new vscode.Range(
		new vscode.Position(range.start.line, range.start.character),
		new vscode.Position(range.end.line, range.end.character)
	);
}

function diagnosticSeverity(value) {
	switch (value) {
		case 1:
			return vscode.DiagnosticSeverity.Error;
		case 2:
			return vscode.DiagnosticSeverity.Warning;
		case 3:
			return vscode.DiagnosticSeverity.Information;
		case 4:
			return vscode.DiagnosticSeverity.Hint;
		default:
			return vscode.DiagnosticSeverity.Error;
	}
}

module.exports = {
  activate,
  deactivate
};
