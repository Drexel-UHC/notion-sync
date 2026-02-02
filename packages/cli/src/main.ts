#!/usr/bin/env node

import { parseArgs } from "node:util";
import {
	createNotionClient,
	normalizeNotionId,
	detectNotionObject,
	freezePage,
	freezeDatabase,
} from "@notion-sync/core";
import { nodeFs } from "./fs-adapter.js";
import { nodeFm } from "./frontmatter-adapter.js";
import { loadConfig, saveConfig } from "./config.js";

async function main(): Promise<void> {
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
		case "resync":
			await runResync(args.slice(1));
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
		console.error("Error: Missing Notion URL or ID");
		console.error("Usage: notion-sync sync <url-or-id> [--output <folder>] [--api-key <key>]");
		process.exit(1);
	}

	const config = await loadConfig();
	const apiKey = values["api-key"] || config.apiKey;

	if (!apiKey) {
		console.error("Error: No API key provided.");
		console.error("Set it via: notion-sync config set apiKey <key>");
		console.error("Or pass --api-key <key>, or set NOTION_SYNC_API_KEY env var.");
		process.exit(2);
	}

	const outputFolder = values.output || config.defaultOutputFolder;
	const rawId = positionals[0];

	let notionId: string;
	try {
		notionId = normalizeNotionId(rawId);
	} catch (err) {
		console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
		return process.exit(1);
	}

	const client = createNotionClient(apiKey);

	console.log("Detecting Notion object type...");
	const detection = await detectNotionObject(client, notionId);

	if (detection.type === "page") {
		console.log("Syncing page...");
		const result = await freezePage({
			client,
			fs: nodeFs,
			fm: nodeFm,
			notionId: detection.id,
			outputFolder,
		});
		console.log(`${result.status}: ${result.filePath}`);
	} else {
		console.log("Syncing database...");
		const result = await freezeDatabase(
			{
				client,
				fs: nodeFs,
				fm: nodeFm,
				notionId: detection.id,
				outputFolder,
			},
			(current, total, title) => {
				process.stdout.write(`\rSyncing "${title}"... ${current}/${total} entries`);
			}
		);
		process.stdout.write("\n");
		console.log(`Done: "${result.title}"`);
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
	}
}

async function runResync(args: string[]): Promise<void> {
	const { values, positionals } = parseArgs({
		args,
		options: {
			"api-key": { type: "string" },
		},
		allowPositionals: true,
	});

	if (positionals.length === 0) {
		console.error("Error: Missing file or folder path");
		console.error("Usage: notion-sync resync <path> [--api-key <key>]");
		process.exit(1);
	}

	const config = await loadConfig();
	const apiKey = values["api-key"] || config.apiKey;

	if (!apiKey) {
		console.error("Error: No API key provided.");
		process.exit(2);
	}

	const filePath = positionals[0];
	const frontmatter = await nodeFm.readFrontmatter(filePath);

	if (!frontmatter || !frontmatter["notion-id"]) {
		console.error("Error: File does not contain notion-id in frontmatter");
		return process.exit(1);
	}

	const client = createNotionClient(apiKey);
	const notionId = frontmatter["notion-id"] as string;
	const databaseId = frontmatter["notion-database-id"] as string | undefined;

	if (databaseId) {
		// Re-sync the entire database
		const { dirname } = await import("node:path");
		const outputFolder = dirname(dirname(filePath));
		console.log("Re-syncing database...");
		const result = await freezeDatabase(
			{
				client,
				fs: nodeFs,
				fm: nodeFm,
				notionId: databaseId,
				outputFolder,
			},
			(current, total, title) => {
				process.stdout.write(`\rSyncing "${title}"... ${current}/${total} entries`);
			}
		);
		process.stdout.write("\n");
		console.log(`Done: "${result.title}" — ${result.created} created, ${result.updated} updated, ${result.skipped} skipped`);
	} else {
		// Re-sync single page
		const { dirname } = await import("node:path");
		const outputFolder = dirname(filePath);
		console.log("Re-syncing page...");
		const result = await freezePage({
			client,
			fs: nodeFs,
			fm: nodeFm,
			notionId,
			outputFolder,
		});
		console.log(`${result.status}: ${result.filePath}`);
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
	console.log(`notion-sync — Sync Notion pages and databases to Markdown

Usage:
  notion-sync sync <url-or-id> [--output <folder>] [--api-key <key>]
  notion-sync resync <path> [--api-key <key>]
  notion-sync config set <key> <value>

Commands:
  sync     Sync a Notion page or database to local Markdown files
  resync   Re-sync a previously synced file or database
  config   Manage configuration (apiKey, defaultOutputFolder)

Environment:
  NOTION_SYNC_API_KEY   API key (overrides config file)`);
}

main().catch((err) => {
	console.error(`Error: ${err instanceof Error ? err.message : String(err)}`);
	process.exit(1);
});
