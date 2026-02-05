import { Client } from "@notionhq/client";
import {
	DatabaseObjectResponse,
	DataSourceObjectResponse,
	PageObjectResponse,
	PartialPageObjectResponse,
	PartialDataSourceObjectResponse,
} from "@notionhq/client/build/src/api-endpoints";
import {
	DatabaseFreezeResult,
	FileSystem,
	FrontmatterReader,
	FrozenDatabase,
	ProgressCallback,
	DATABASE_METADATA_FILE,
} from "./types.js";
import { notionRequest } from "./notion-client.js";
import { convertRichText } from "./block-converter.js";
import { freezePage } from "./page-freezer.js";
import { sanitizeFileName, joinPath } from "./utils.js";

export interface DatabaseImportOptions {
	client: Client;
	fs: FileSystem;
	fm: FrontmatterReader;
	databaseId: string;
	outputFolder: string;
}

export interface RefreshOptions {
	client: Client;
	fs: FileSystem;
	fm: FrontmatterReader;
	folderPath: string;
	/** Skip timestamp comparison and resync all entries */
	force?: boolean;
}

/**
 * Read database metadata from _database.json in a folder.
 */
export async function readDatabaseMetadata(
	fs: FileSystem,
	folderPath: string
): Promise<FrozenDatabase | null> {
	const metaPath = joinPath(folderPath, DATABASE_METADATA_FILE);
	try {
		const content = await fs.readFile(metaPath);
		return JSON.parse(content) as FrozenDatabase;
	} catch {
		return null;
	}
}

/**
 * Write database metadata to _database.json in a folder.
 */
async function writeDatabaseMetadata(
	fs: FileSystem,
	folderPath: string,
	metadata: FrozenDatabase
): Promise<void> {
	const metaPath = joinPath(folderPath, DATABASE_METADATA_FILE);
	await fs.writeFile(metaPath, JSON.stringify(metadata, null, 2));
}

/**
 * List all synced databases in a folder by scanning for _database.json files.
 */
export async function listSyncedDatabases(
	fs: FileSystem,
	outputFolder: string
): Promise<FrozenDatabase[]> {
	const databases: FrozenDatabase[] = [];

	let dirs: string[];
	try {
		dirs = await fs.listDirectories(outputFolder);
	} catch {
		return databases;
	}

	for (const dir of dirs) {
		const folderPath = joinPath(outputFolder, dir);
		const metadata = await readDatabaseMetadata(fs, folderPath);
		if (metadata) {
			databases.push(metadata);
		}
	}

	return databases;
}

/**
 * Fresh import of a Notion database. Imports all entries without checking
 * for existing local files. Use this for first-time imports.
 */
export async function freshDatabaseImport(
	options: DatabaseImportOptions,
	onProgress?: ProgressCallback
): Promise<DatabaseFreezeResult> {
	const { client, fs, fm, databaseId, outputFolder } = options;

	onProgress?.({ phase: "querying" });

	// Fetch database metadata
	const database = (await notionRequest(() =>
		client.databases.retrieve({ database_id: databaseId })
	)) as DatabaseObjectResponse;

	const dbTitle = convertRichText(database.title) || "Untitled Database";
	const safeName = sanitizeFileName(dbTitle);
	const folderPath = joinPath(outputFolder, safeName);

	// Get the data source ID for querying entries
	if (!database.data_sources || database.data_sources.length === 0) {
		throw new Error(
			"This appears to be a linked database, which is not supported by the Notion API."
		);
	}
	const dataSourceId = database.data_sources[0].id;

	// Verify access
	await notionRequest(() =>
		client.dataSources.retrieve({ data_source_id: dataSourceId })
	);

	// Create folder
	await fs.mkdir(folderPath, true);

	// Query all entries
	const entries = await queryAllEntries(client, dataSourceId);
	const total = entries.length;

	onProgress?.({ phase: "stale-detected", stale: total, total });

	// Track results
	let created = 0;
	let updated = 0;
	let skipped = 0;
	let failed = 0;
	const errors: string[] = [];

	// Process all entries
	let current = 0;
	for (const entry of entries) {
		current++;
		onProgress?.({ phase: "importing", current, total, title: dbTitle });

		try {
			const result = await freezePage({
				client,
				fs,
				fm,
				notionId: entry.id,
				outputFolder: folderPath,
				databaseId,
				page: entry,
			});

			switch (result.status) {
				case "created":
					created++;
					break;
				case "updated":
					updated++;
					break;
				case "skipped":
					skipped++;
					break;
			}
		} catch (err) {
			failed++;
			const msg = `Entry ${entry.id}: ${err instanceof Error ? err.message : String(err)}`;
			errors.push(msg);
			console.error(`notion-sync: Failed to freeze entry ${entry.id}:`, err);
		}
	}

	// Write database metadata
	const metadata: FrozenDatabase = {
		databaseId,
		title: dbTitle,
		url: database.url,
		folderPath,
		lastSyncedAt: new Date().toISOString(),
		entryCount: total,
	};
	await writeDatabaseMetadata(fs, folderPath, metadata);

	onProgress?.({ phase: "complete" });

	return { title: dbTitle, folderPath, total, created, updated, skipped, deleted: 0, failed, errors };
}

/**
 * Refresh an existing frozen database. Only processes entries that have
 * changed since the last sync (based on last_edited_time). Also detects
 * and marks deleted entries.
 *
 * Reads database info from _database.json in the folder.
 */
export async function refreshDatabase(
	options: RefreshOptions,
	onProgress?: ProgressCallback
): Promise<DatabaseFreezeResult> {
	const { client, fs, fm, folderPath, force = false } = options;

	// Read existing metadata
	const metadata = await readDatabaseMetadata(fs, folderPath);
	if (!metadata) {
		throw new Error(
			`No _database.json found in ${folderPath}. Use 'sync' to import the database first.`
		);
	}

	const databaseId = metadata.databaseId;

	onProgress?.({ phase: "querying" });

	// Fetch database metadata
	const database = (await notionRequest(() =>
		client.databases.retrieve({ database_id: databaseId })
	)) as DatabaseObjectResponse;

	const dbTitle = convertRichText(database.title) || "Untitled Database";

	// Get the data source ID
	if (!database.data_sources || database.data_sources.length === 0) {
		throw new Error(
			"This appears to be a linked database, which is not supported by the Notion API."
		);
	}
	const dataSourceId = database.data_sources[0].id;

	// Verify access
	await notionRequest(() =>
		client.dataSources.retrieve({ data_source_id: dataSourceId })
	);

	// Query all entries
	const entries = await queryAllEntries(client, dataSourceId);
	const total = entries.length;

	onProgress?.({ phase: "diffing", total });

	// Scan existing local files
	const localFiles = await scanLocalFiles(fs, fm, folderPath);

	// Track results
	let created = 0;
	let updated = 0;
	let skipped = 0;
	let deleted = 0;
	let failed = 0;
	const errors: string[] = [];

	// Pre-filter: skip entries whose last_edited_time matches stored frontmatter
	// (unless force=true, which resyncs everything)
	const allEntryIds = new Set(entries.map(e => e.id));

	const entriesToProcess = entries.filter(entry => {
		if (force) return true;
		const local = localFiles.get(entry.id);
		if (local?.lastEdited && local.lastEdited === entry.last_edited_time) {
			skipped++;
			return false;
		}
		return true;
	});

	const staleCount = entriesToProcess.length;
	onProgress?.({ phase: "stale-detected", stale: staleCount, total });

	// Process only changed/new entries
	let current = 0;
	for (const entry of entriesToProcess) {
		current++;
		onProgress?.({ phase: "importing", current, total: staleCount, title: dbTitle });

		try {
			const result = await freezePage({
				client,
				fs,
				fm,
				notionId: entry.id,
				outputFolder: folderPath,
				databaseId,
				page: entry,
			});

			switch (result.status) {
				case "created":
					created++;
					break;
				case "updated":
					updated++;
					break;
				case "skipped":
					skipped++;
					break;
			}
		} catch (err) {
			failed++;
			const msg = `Entry ${entry.id}: ${err instanceof Error ? err.message : String(err)}`;
			errors.push(msg);
			console.error(`notion-sync: Failed to freeze entry ${entry.id}:`, err);
		}
	}

	// Mark deleted entries
	for (const [id, info] of localFiles) {
		if (!allEntryIds.has(id)) {
			await markAsDeleted(fs, info.filePath);
			deleted++;
		}
	}

	// Update database metadata
	const updatedMetadata: FrozenDatabase = {
		databaseId,
		title: dbTitle,
		url: database.url,
		folderPath,
		lastSyncedAt: new Date().toISOString(),
		entryCount: total,
	};
	await writeDatabaseMetadata(fs, folderPath, updatedMetadata);

	onProgress?.({ phase: "complete" });

	return { title: dbTitle, folderPath, total, created, updated, skipped, deleted, failed, errors };
}

async function queryAllEntries(
	client: Client,
	dataSourceId: string
): Promise<PageObjectResponse[]> {
	const entries: PageObjectResponse[] = [];
	let cursor: string | undefined = undefined;

	do {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		const response: any = await notionRequest(() =>
			client.dataSources.query({
				data_source_id: dataSourceId,
				start_cursor: cursor,
				page_size: 100,
			})
		);
		const results: Array<
			| PageObjectResponse
			| PartialPageObjectResponse
			| PartialDataSourceObjectResponse
			| DataSourceObjectResponse
		> = response.results;
		for (const result of results) {
			if (result.object === "page" && "properties" in result) {
				entries.push(result as PageObjectResponse);
			}
		}
		cursor = response.has_more
			? (response.next_cursor ?? undefined)
			: undefined;
	} while (cursor);

	return entries;
}

interface LocalFileInfo {
	filePath: string;
	lastEdited?: string;
}

async function scanLocalFiles(
	fs: FileSystem,
	fm: FrontmatterReader,
	folderPath: string
): Promise<Map<string, LocalFileInfo>> {
	const map = new Map<string, LocalFileInfo>();

	let files: string[];
	try {
		files = await fs.listMarkdownFiles(folderPath);
	} catch {
		// Folder may not exist yet
		return map;
	}

	for (const fileName of files) {
		const filePath = joinPath(folderPath, fileName);
		const frontmatter = await fm.readFrontmatter(filePath);
		const notionId = frontmatter?.["notion-id"];
		if (typeof notionId === "string") {
			const lastEdited = frontmatter?.["notion-last-edited"];
			map.set(notionId, {
				filePath,
				lastEdited: typeof lastEdited === "string" ? lastEdited : undefined,
			});
		}
	}

	return map;
}

async function markAsDeleted(fs: FileSystem, filePath: string): Promise<void> {
	let content: string;
	try {
		content = await fs.readFile(filePath);
	} catch {
		return;
	}

	// Check if already marked
	if (content.includes("notion-deleted: true")) return;

	// Insert notion-deleted into frontmatter
	if (content.startsWith("---\n")) {
		const endIdx = content.indexOf("\n---", 3);
		if (endIdx !== -1) {
			const before = content.slice(0, endIdx);
			const after = content.slice(endIdx);
			await fs.writeFile(filePath, before + "\nnotion-deleted: true" + after);
			return;
		}
	}

	// No frontmatter found, add it
	const fmStr = "---\nnotion-deleted: true\n---\n";
	await fs.writeFile(filePath, fmStr + content);
}
