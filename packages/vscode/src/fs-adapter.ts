import * as vscode from "vscode";
import type { FileSystem } from "@notion-sync/core";

export function vscodeFs(workspaceRoot: vscode.Uri): FileSystem {
	return {
		readFile: async (p) => {
			const uri = vscode.Uri.joinPath(workspaceRoot, p);
			const bytes = await vscode.workspace.fs.readFile(uri);
			return Buffer.from(bytes).toString("utf-8");
		},

		writeFile: async (p, content) => {
			const uri = vscode.Uri.joinPath(workspaceRoot, p);
			// Ensure parent directory exists
			const parentUri = vscode.Uri.joinPath(uri, "..");
			try {
				await vscode.workspace.fs.createDirectory(parentUri);
			} catch {
				// Directory may already exist
			}
			await vscode.workspace.fs.writeFile(uri, Buffer.from(content, "utf-8"));
		},

		fileExists: async (p) => {
			const uri = vscode.Uri.joinPath(workspaceRoot, p);
			try {
				await vscode.workspace.fs.stat(uri);
				return true;
			} catch {
				return false;
			}
		},

		mkdir: async (p) => {
			const uri = vscode.Uri.joinPath(workspaceRoot, p);
			await vscode.workspace.fs.createDirectory(uri);
		},

		listMarkdownFiles: async (dir) => {
			const uri = vscode.Uri.joinPath(workspaceRoot, dir);
			const entries = await vscode.workspace.fs.readDirectory(uri);
			return entries
				.filter(
					([name, type]) =>
						type === vscode.FileType.File && name.endsWith(".md")
				)
				.map(([name]) => name);
		},
	};
}
