import * as vscode from "vscode";
import { syncCommand, resyncCommand } from "./commands.js";

export function activate(context: vscode.ExtensionContext): void {
	context.subscriptions.push(
		vscode.commands.registerCommand("notionSync.sync", syncCommand),
		vscode.commands.registerCommand("notionSync.resync", resyncCommand)
	);
}

export function deactivate(): void {
	// Nothing to clean up
}
