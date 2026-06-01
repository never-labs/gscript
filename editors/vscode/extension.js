"use strict";

const vscode = require("vscode");
const path = require("path");

function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand("leia.runFile", () => runCurrentFile("run")),
    vscode.commands.registerCommand("leia.formatFile", formatCurrentFile),
    vscode.commands.registerCommand("leia.testWorkspace", () => runWorkspaceCommand("test")),
    vscode.commands.registerCommand("leia.lintWorkspace", () => runWorkspaceCommand("lint")),
    vscode.commands.registerCommand("leia.checkWorkspace", () => runWorkspaceCommand("check")),
    vscode.commands.registerCommand("leia.previewSpec", previewSpec),
    vscode.tasks.registerTaskProvider("leia", new LeiaTaskProvider())
  );
}

function deactivate() {}

function executable() {
  return vscode.workspace.getConfiguration("leia").get("executable", "leia");
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

module.exports = {
  activate,
  deactivate
};
