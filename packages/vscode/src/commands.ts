import * as vscode from "vscode";
import {
	createNotionClient,
	normalizeNotionId,
	detectNotionObject,
	freezePage,
	freezeDatabase,
} from "@notion-sync/core";
import { vscodeFs } from "./fs-adapter.js";
import { vscodeFm } from "./frontmatter-adapter.js";

function getWorkspaceRoot(): vscode.Uri {
	const folders = vscode.workspace.workspaceFolders;
	if (!folders || folders.length === 0) {
		throw new Error("No workspace folder open");
	}
	return folders[0].uri;
}

function getApiKey(): string {
	const config = vscode.workspace.getConfiguration("notionSync");
	const apiKey = config.get<string>("apiKey", "");
	if (!apiKey) {
		throw new Error(
			"No API key configured. Set it in Settings > Notion Sync > Api Key"
		);
	}
	return apiKey;
}

function getDefaultOutputFolder(): string {
	const config = vscode.workspace.getConfiguration("notionSync");
	return config.get<string>("defaultOutputFolder", "notion");
}

export async function syncCommand(): Promise<void> {
	try {
		const apiKey = getApiKey();

		const rawInput = await vscode.window.showInputBox({
			prompt: "Enter a Notion page or database URL (or ID)",
			placeHolder: "https://www.notion.so/... or 32-char ID",
		});
		if (!rawInput) return;

		const defaultFolder = getDefaultOutputFolder();
		const outputFolder = await vscode.window.showInputBox({
			prompt: "Output folder (relative to workspace root)",
			value: defaultFolder,
		});
		if (outputFolder === undefined) return;

		let notionId: string;
		try {
			notionId = normalizeNotionId(rawInput);
		} catch (err) {
			vscode.window.showErrorMessage(
				`Invalid Notion ID: ${err instanceof Error ? err.message : String(err)}`
			);
			return;
		}

		const client = createNotionClient(apiKey);
		const workspaceRoot = getWorkspaceRoot();
		const fs = vscodeFs(workspaceRoot);
		const fm = vscodeFm(fs);

		await vscode.window.withProgress(
			{
				location: vscode.ProgressLocation.Notification,
				title: "Notion Sync",
				cancellable: false,
			},
			async (progress) => {
				progress.report({ message: "Detecting Notion object type..." });
				const detection = await detectNotionObject(client, notionId);

				if (detection.type === "page") {
					progress.report({ message: "Syncing page..." });
					const result = await freezePage({
						client,
						fs,
						fm,
						notionId: detection.id,
						outputFolder,
					});
					vscode.window.showInformationMessage(
						`Page ${result.status}: ${result.filePath}`
					);
				} else {
					const result = await freezeDatabase(
						{
							client,
							fs,
							fm,
							notionId: detection.id,
							outputFolder,
						},
						(current, total, title) => {
							progress.report({
								message: `Syncing "${title}"... ${current}/${total}`,
								increment: (1 / total) * 100,
							});
						}
					);
					const summary = [
						`"${result.title}" synced.`,
						`Created: ${result.created}`,
						`Updated: ${result.updated}`,
						`Skipped: ${result.skipped}`,
						`Deleted: ${result.deleted}`,
					];
					if (result.failed > 0) {
						summary.push(`Failed: ${result.failed}`);
					}
					vscode.window.showInformationMessage(summary.join(" | "));
				}
			}
		);
	} catch (err) {
		vscode.window.showErrorMessage(
			`Notion Sync error: ${err instanceof Error ? err.message : String(err)}`
		);
	}
}

export async function resyncCommand(): Promise<void> {
	try {
		const apiKey = getApiKey();
		const editor = vscode.window.activeTextEditor;

		if (!editor) {
			vscode.window.showErrorMessage("No active file to re-sync");
			return;
		}

		const workspaceRoot = getWorkspaceRoot();
		const fs = vscodeFs(workspaceRoot);
		const fm = vscodeFm(fs);

		// Get relative path from workspace root
		const relativePath = vscode.workspace.asRelativePath(editor.document.uri);
		const frontmatter = await fm.readFrontmatter(relativePath);

		if (!frontmatter || !frontmatter["notion-id"]) {
			vscode.window.showErrorMessage(
				"This file does not contain a notion-id in its frontmatter"
			);
			return;
		}

		const client = createNotionClient(apiKey);
		const notionId = frontmatter["notion-id"] as string;
		const databaseId = frontmatter["notion-database-id"] as string | undefined;

		await vscode.window.withProgress(
			{
				location: vscode.ProgressLocation.Notification,
				title: "Notion Sync",
				cancellable: false,
			},
			async (progress) => {
				if (databaseId) {
					// Re-sync entire database
					const parts = relativePath.split("/");
					const outputFolder = parts.length >= 2
						? parts.slice(0, -2).join("/") || "."
						: ".";

					const result = await freezeDatabase(
						{
							client,
							fs,
							fm,
							notionId: databaseId,
							outputFolder,
						},
						(current, total, title) => {
							progress.report({
								message: `Re-syncing "${title}"... ${current}/${total}`,
								increment: (1 / total) * 100,
							});
						}
					);
					vscode.window.showInformationMessage(
						`"${result.title}" re-synced: ${result.created} created, ${result.updated} updated, ${result.skipped} skipped`
					);
				} else {
					// Re-sync single page
					const parts = relativePath.split("/");
					const outputFolder = parts.length >= 2
						? parts.slice(0, -1).join("/")
						: ".";

					progress.report({ message: "Re-syncing page..." });
					const result = await freezePage({
						client,
						fs,
						fm,
						notionId,
						outputFolder,
					});
					vscode.window.showInformationMessage(
						`Page ${result.status}: ${result.filePath}`
					);
				}
			}
		);
	} catch (err) {
		vscode.window.showErrorMessage(
			`Notion Sync error: ${err instanceof Error ? err.message : String(err)}`
		);
	}
}
