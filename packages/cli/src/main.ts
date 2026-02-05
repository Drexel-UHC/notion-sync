#!/usr/bin/env node

import { parseArgs } from "node:util";
import {
	createNotionClient,
	normalizeNotionId,
	freshDatabaseImport,
	refreshDatabase,
	listSyncedDatabases,
	ProgressPhase,
} from "@notion-sync/core";
import { nodeFs } from "./fs-adapter.js";
import { nodeFm } from "./frontmatter-adapter.js";
import { loadConfig, saveConfig, migrateApiKeyToKeychain } from "./config.js";

async function main(): Promise<void> {
	await migrateApiKeyToKeychain();
	const args = process.argv.slice(2);

	if (args.length === 0 || args[0] === "--help" || args[0] === "-h") {
		printUsage();
		process.exit(0);
	}

	const command = args[0];

	switch (command) {
		case "sync":
			await runSync(args.slice(1));
			break;
		case "refresh":
			await runRefresh(args.slice(1));
			break;
		case "list":
			await runList(args.slice(1));
			break;
		case "config":
			await runConfig(args.slice(1));
			break;
		default:
			console.error(`Unknown command: ${command}`);
			printUsage();
			process.exit(1);
	}
}

function formatProgress(progress: ProgressPhase, dbTitle?: string): string {
	switch (progress.phase) {
		case "querying":
			return "Querying database entries...";
		case "diffing":
			return `Comparing ${progress.total} entries with local files...`;
		case "stale-detected":
			return `Found ${progress.stale} entries to sync (${progress.total} total)`;
		case "importing":
			return `Syncing "${dbTitle || progress.title}"... ${progress.current}/${progress.total}`;
		case "complete":
			return "Done";
	}
}

async function runSync(args: string[]): Promise<void> {
	const { values, positionals } = parseArgs({
		args,
		options: {
			output: { type: "string", short: "o" },
			"api-key": { type: "string" },
		},
		allowPositionals: true,
	});

	if (positionals.length === 0) {
		console.error("Error: Missing Notion database URL or ID");
		console.error("Usage: notion-sync sync <database-url-or-id> [--output <folder>] [--api-key <key>]");
		process.exit(1);
	}

	const config = await loadConfig();
	const apiKey = values["api-key"] || config.apiKey;

	if (!apiKey) {
		console.error("Error: No API key provided.");
		console.error("Set it via: notion-sync config set apiKey <key> (stored in OS keychain)");
		console.error("Or pass --api-key <key>, or set NOTION_SYNC_API_KEY env var.");
		process.exit(2);
	}

	const outputFolder = values.output || config.defaultOutputFolder;
	const rawId = positionals[0];

	let databaseId: string;
	try {
		databaseId = normalizeNotionId(rawId);
	} catch (err) {
		console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
		return process.exit(1);
	}

	const client = createNotionClient(apiKey);

	console.log("Syncing database...");
	let dbTitle = "";

	try {
		const result = await freshDatabaseImport(
			{
				client,
				fs: nodeFs,
				fm: nodeFm,
				databaseId,
				outputFolder,
			},
			(progress) => {
				if (progress.phase === "importing") {
					dbTitle = progress.title;
				}
				process.stdout.write(`\r${formatProgress(progress, dbTitle).padEnd(60)}`);
			}
		);
		process.stdout.write("\n");
		console.log(`Done: "${result.title}"`);
		console.log(`  Folder:  ${result.folderPath}`);
		console.log(`  Total:   ${result.total}`);
		console.log(`  Created: ${result.created}`);
		console.log(`  Updated: ${result.updated}`);
		console.log(`  Skipped: ${result.skipped}`);
		if (result.failed > 0) {
			console.log(`  Failed:  ${result.failed}`);
			for (const err of result.errors) {
				console.error(`    - ${err}`);
			}
		}
	} catch (err) {
		console.error(`\nError: ${err instanceof Error ? err.message : String(err)}`);
		process.exit(1);
	}
}

async function runRefresh(args: string[]): Promise<void> {
	const { values, positionals } = parseArgs({
		args,
		options: {
			"api-key": { type: "string" },
			"force": { type: "boolean", short: "f" },
		},
		allowPositionals: true,
	});

	if (positionals.length === 0) {
		console.error("Error: Missing database folder path");
		console.error("Usage: notion-sync refresh <database-folder> [--force] [--api-key <key>]");
		console.error("Example: notion-sync refresh ./notion/My\\ Database");
		console.error("         notion-sync refresh ./notion/My\\ Database --force");
		process.exit(1);
	}

	const config = await loadConfig();
	const apiKey = values["api-key"] || config.apiKey;
	const force = values.force ?? false;

	if (!apiKey) {
		console.error("Error: No API key provided.");
		process.exit(2);
	}

	const folderPath = positionals[0];
	const client = createNotionClient(apiKey);

	console.log(force ? "Force refreshing database (ignoring timestamps)..." : "Refreshing database...");
	let dbTitle = "";

	try {
		const result = await refreshDatabase(
			{
				client,
				fs: nodeFs,
				fm: nodeFm,
				folderPath,
				force,
			},
			(progress) => {
				if (progress.phase === "importing") {
					dbTitle = progress.title;
				}
				process.stdout.write(`\r${formatProgress(progress, dbTitle).padEnd(60)}`);
			}
		);
		process.stdout.write("\n");
		console.log(`Done: "${result.title}"`);
		console.log(`  Total:   ${result.total}`);
		console.log(`  Created: ${result.created}`);
		console.log(`  Updated: ${result.updated}`);
		console.log(`  Skipped: ${result.skipped}`);
		console.log(`  Deleted: ${result.deleted}`);
		if (result.failed > 0) {
			console.log(`  Failed:  ${result.failed}`);
			for (const err of result.errors) {
				console.error(`    - ${err}`);
			}
		}
	} catch (err) {
		console.error(`\nError: ${err instanceof Error ? err.message : String(err)}`);
		process.exit(1);
	}
}

async function runList(args: string[]): Promise<void> {
	const { positionals } = parseArgs({
		args,
		options: {},
		allowPositionals: true,
	});

	const outputFolder = positionals[0] || "./notion";

	try {
		const databases = await listSyncedDatabases(nodeFs, outputFolder);

		if (databases.length === 0) {
			console.log(`No synced databases found in ${outputFolder}`);
			return;
		}

		console.log(`Synced databases in ${outputFolder}:\n`);
		for (const db of databases) {
			console.log(`  ${db.title}`);
			console.log(`    Folder:      ${db.folderPath}`);
			console.log(`    Database ID: ${db.databaseId}`);
			console.log(`    Entries:     ${db.entryCount}`);
			console.log(`    Last synced: ${db.lastSyncedAt}`);
			console.log();
		}
	} catch (err) {
		console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
		process.exit(1);
	}
}

async function runConfig(args: string[]): Promise<void> {
	if (args.length < 3 || args[0] !== "set") {
		console.error("Usage: notion-sync config set <key> <value>");
		console.error("Keys: apiKey, defaultOutputFolder");
		process.exit(1);
	}

	const key = args[1];
	const value = args[2];

	const validKeys = ["apiKey", "defaultOutputFolder"];
	if (!validKeys.includes(key)) {
		console.error(`Unknown config key: ${key}`);
		console.error(`Valid keys: ${validKeys.join(", ")}`);
		process.exit(1);
	}

	await saveConfig(key, value);
	console.log(`Saved ${key}`);
}

function printUsage(): void {
	console.log(`notion-sync — Sync Notion databases to Markdown

Usage:
  notion-sync sync <database-url-or-id> [--output <folder>] [--api-key <key>]
  notion-sync refresh <database-folder> [--force] [--api-key <key>]
  notion-sync list [<output-folder>]
  notion-sync config set <key> <value>

Commands:
  sync      Import a Notion database to local Markdown files
  refresh   Refresh an existing synced database (incremental update)
            --force, -f  Resync all entries, ignoring timestamps
  list      List all synced databases in a folder
  config    Manage configuration (apiKey, defaultOutputFolder)

Examples:
  notion-sync sync https://notion.so/mydb... --output ./notion
  notion-sync refresh ./notion/My\\ Database
  notion-sync refresh ./notion/My\\ Database --force
  notion-sync list ./notion

API Key Priority:
  1. NOTION_SYNC_API_KEY env var
  2. OS keychain (set via: notion-sync config set apiKey <key>)
  3. Config file fallback (~/.notion-sync.json)`);
}

main().catch((err) => {
	console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
	process.exit(1);
});
