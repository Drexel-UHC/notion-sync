import { Client } from "@notionhq/client";
import {
	DatabaseObjectResponse,
	DataSourceObjectResponse,
	PageObjectResponse,
	PartialPageObjectResponse,
	PartialDataSourceObjectResponse,
} from "@notionhq/client/build/src/api-endpoints";
import { DatabaseFreezeResult, FileSystem, FrontmatterReader, FreezeOptions, ProgressCallback } from "./types.js";
import { notionRequest } from "./notion-client.js";
import { convertRichText } from "./block-converter.js";
import { freezePage } from "./page-freezer.js";

/**
 * Syncs a Notion database to local Markdown files.
 *
 * NOTE: This function uses `client.dataSources.retrieve()` and
 * `client.dataSources.query()` (NOT `databases.query()`). This is a
 * newer Notion API for querying database entries with full property data.
 */
export async function freezeDatabase(
	options: Omit<FreezeOptions, "databaseId">,
	onProgress?: ProgressCallback
): Promise<DatabaseFreezeResult> {
	const { client, fs, fm, notionId, outputFolder } = options;

	// Fetch database metadata
	const database = (await notionRequest(() =>
		client.databases.retrieve({ database_id: notionId })
	)) as DatabaseObjectResponse;

	const dbTitle = convertRichText(database.title) || "Untitled Database";
	const safeName = dbTitle.replace(/[\\/:*?"<>|]/g, "-").trim() || "Untitled Database";
	const folderPath = outputFolder + "/" + safeName;

	// Get the data source ID for querying entries and reading properties
	if (!database.data_sources || database.data_sources.length === 0) {
		throw new Error(
			"This appears to be a linked database, which is not supported by the Notion API."
		);
	}
	const dataSourceId = database.data_sources[0].id;

	// Retrieve the data source to get property schema (verifies access)
	await notionRequest(() =>
		client.dataSources.retrieve({ data_source_id: dataSourceId })
	);

	// Create folder if needed
	await fs.mkdir(folderPath, true);

	// Query all entries via dataSources.query (paginated)
	const entries = await queryAllEntries(client, dataSourceId);

	// Scan existing local files for this database
	const localFiles = await scanLocalFiles(fs, fm, folderPath);

	// Track results
	let created = 0;
	let updated = 0;
	let skipped = 0;
	let deleted = 0;
	let failed = 0;
	const errors: string[] = [];

	// Process each entry — continue on failure
	const total = entries.length;
	const processedIds = new Set<string>();
	let current = 0;
	for (const entry of entries) {
		processedIds.add(entry.id);
		current++;

		if (onProgress) onProgress(current, total, dbTitle);

		try {
			const result = await freezePage({
				client,
				fs,
				fm,
				notionId: entry.id,
				outputFolder: folderPath,
				databaseId: notionId,
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

	// Mark deleted entries (in Notion but not returned in query)
	for (const [id, filePath] of localFiles) {
		if (!processedIds.has(id)) {
			await markAsDeleted(fs, filePath);
			deleted++;
		}
	}

	return { title: dbTitle, folderPath, created, updated, skipped, deleted, failed, errors };
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

async function scanLocalFiles(
	fs: FileSystem,
	fm: FrontmatterReader,
	folderPath: string
): Promise<Map<string, string>> {
	const map = new Map<string, string>();

	let files: string[];
	try {
		files = await fs.listMarkdownFiles(folderPath);
	} catch {
		// Folder may not exist yet
		return map;
	}

	for (const fileName of files) {
		const filePath = folderPath + "/" + fileName;
		const frontmatter = await fm.readFrontmatter(filePath);
		const notionId = frontmatter?.["notion-id"];
		if (typeof notionId === "string") {
			map.set(notionId, filePath);
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
	const fm = "---\nnotion-deleted: true\n---\n";
	await fs.writeFile(filePath, fm + content);
}
