import { describe, it, expect, vi, beforeEach } from "vitest";
import { freshDatabaseImport, refreshDatabase, listSyncedDatabases, readDatabaseMetadata } from "../src/database-freezer.js";
import { Client } from "@notionhq/client";
import type { FileSystem, FrontmatterReader } from "../src/types.js";

// Mock notion-client to skip throttle/retry
vi.mock("../src/notion-client.js", () => ({
	notionRequest: <T>(fn: () => Promise<T>) => fn(),
}));

// Mock page-freezer
vi.mock("../src/page-freezer.js", () => ({
	freezePage: vi.fn().mockResolvedValue({ status: "created", filePath: "test.md", title: "Test" }),
}));

// Mock block-converter
vi.mock("../src/block-converter.js", () => ({
	convertRichText: vi.fn((items: any[]) => items.map((i: any) => i.plain_text || "").join("")),
}));

import { freezePage } from "../src/page-freezer.js";

function mockFs(): FileSystem {
	return {
		readFile: vi.fn().mockResolvedValue(""),
		writeFile: vi.fn().mockResolvedValue(undefined),
		fileExists: vi.fn().mockResolvedValue(false),
		mkdir: vi.fn().mockResolvedValue(undefined),
		listMarkdownFiles: vi.fn().mockResolvedValue([]),
		listDirectories: vi.fn().mockResolvedValue([]),
	};
}

function mockFm(): FrontmatterReader {
	return {
		readFrontmatter: vi.fn().mockResolvedValue(null),
	};
}

function makeDatabase(title: string) {
	return {
		object: "database",
		id: "db-123",
		url: "https://www.notion.so/db-123",
		title: [{ type: "text", text: { content: title }, plain_text: title }],
		data_sources: [{ id: "ds-1" }],
	};
}

function makeEntry(id: string, title: string, lastEdited = "2024-01-15T10:00:00.000Z") {
	return {
		object: "page",
		id,
		last_edited_time: lastEdited,
		properties: {
			Name: { type: "title", title: [{ plain_text: title }] },
		},
	};
}

describe("freshDatabaseImport", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("syncs all entries from a database", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("My Database")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [
						makeEntry("entry-1", "Entry 1"),
						makeEntry("entry-2", "Entry 2"),
					],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		const result = await freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		});

		expect(result.title).toBe("My Database");
		expect(result.folderPath).toBe("output/My Database");
		expect(result.created).toBe(2);
		expect(result.total).toBe(2);
		expect(result.failed).toBe(0);
		expect(fs.mkdir).toHaveBeenCalledWith("output/My Database", true);
		expect(freezePage).toHaveBeenCalledTimes(2);
	});

	it("passes pre-fetched page to freezePage", async () => {
		const entry = makeEntry("e-1", "E1");
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [entry],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		await freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		});

		expect(freezePage).toHaveBeenCalledWith(
			expect.objectContaining({ page: entry })
		);
	});

	it("paginates through all entries", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn()
					.mockResolvedValueOnce({
						results: [makeEntry("e-1", "E1")],
						has_more: true,
						next_cursor: "cursor-1",
					})
					.mockResolvedValueOnce({
						results: [makeEntry("e-2", "E2")],
						has_more: false,
					}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		const result = await freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		});

		expect(result.created).toBe(2);
		expect(client.dataSources.query).toHaveBeenCalledTimes(2);
	});

	it("handles failed entries without aborting", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [
						makeEntry("e-1", "E1"),
						makeEntry("e-2", "E2"),
					],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		(freezePage as any)
			.mockResolvedValueOnce({ status: "created", filePath: "E1.md", title: "E1" })
			.mockRejectedValueOnce(new Error("API error"));

		const result = await freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		});

		expect(result.created).toBe(1);
		expect(result.failed).toBe(1);
		expect(result.errors).toHaveLength(1);
		expect(result.errors[0]).toContain("API error");
	});

	it("calls progress callback with phases", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [makeEntry("e-1", "E1"), makeEntry("e-2", "E2")],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();
		const onProgress = vi.fn();

		await freshDatabaseImport(
			{ client, fs, fm, databaseId: "db-123", outputFolder: "output" },
			onProgress
		);

		expect(onProgress).toHaveBeenCalledWith({ phase: "querying" });
		expect(onProgress).toHaveBeenCalledWith({ phase: "stale-detected", stale: 2, total: 2 });
		expect(onProgress).toHaveBeenCalledWith({ phase: "importing", current: 1, total: 2, title: "DB" });
		expect(onProgress).toHaveBeenCalledWith({ phase: "importing", current: 2, total: 2, title: "DB" });
		expect(onProgress).toHaveBeenCalledWith({ phase: "complete" });
	});

	it("throws if database has no data sources", async () => {
		const db = makeDatabase("DB");
		db.data_sources = [];

		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(db),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		await expect(freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		})).rejects.toThrow("linked database");
	});
});

describe("refreshDatabase", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("skips entries where last_edited_time matches stored frontmatter", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [
						makeEntry("e-1", "E1", "2024-01-15T10:00:00.000Z"),
						makeEntry("e-2", "E2", "2024-01-15T10:00:00.000Z"),
						makeEntry("e-3", "E3", "2024-02-01T12:00:00.000Z"),
					],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		// Mock _database.json read
		const dbMetadata = JSON.stringify({
			databaseId: "db-123",
			title: "DB",
			url: "https://www.notion.so/db-123",
			folderPath: "output/DB",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 3,
		});
		(fs.readFile as any).mockImplementation((path: string) => {
			if (path.includes("_database.json")) {
				return Promise.resolve(dbMetadata);
			}
			return Promise.resolve("");
		});

		// e-1 and e-2 have matching local frontmatter, e-3 is changed
		(fs.listMarkdownFiles as any).mockResolvedValue(["E1.md", "E2.md", "E3.md"]);
		(fm.readFrontmatter as any)
			.mockResolvedValueOnce({ "notion-id": "e-1", "notion-last-edited": "2024-01-15T10:00:00.000Z" })
			.mockResolvedValueOnce({ "notion-id": "e-2", "notion-last-edited": "2024-01-15T10:00:00.000Z" })
			.mockResolvedValueOnce({ "notion-id": "e-3", "notion-last-edited": "2024-01-01T00:00:00.000Z" });

		const result = await refreshDatabase({ client, fs, fm, folderPath: "output/DB" });

		expect(freezePage).toHaveBeenCalledTimes(1);
		expect(result.skipped).toBe(2);
		expect(result.created).toBe(1);
		expect(result.deleted).toBe(0);
	});

	it("tracks deleted entries", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [makeEntry("e-1", "E1")],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		// Mock _database.json read and deleted file read
		const dbMetadata = JSON.stringify({
			databaseId: "db-123",
			title: "DB",
			url: "https://www.notion.so/db-123",
			folderPath: "output/DB",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 2,
		});
		(fs.readFile as any).mockImplementation((path: string) => {
			if (path.includes("_database.json")) {
				return Promise.resolve(dbMetadata);
			}
			// readFile for markAsDeleted
			return Promise.resolve("---\nnotion-id: e-deleted\n---\nContent");
		});

		// Simulate existing local files with a deleted entry
		(fs.listMarkdownFiles as any).mockResolvedValue(["E1.md", "Deleted.md"]);
		(fm.readFrontmatter as any)
			.mockResolvedValueOnce({ "notion-id": "e-1", "notion-last-edited": "2024-01-15T10:00:00.000Z" }) // E1.md — matches entry
			.mockResolvedValueOnce({ "notion-id": "e-deleted" }); // Deleted.md

		const result = await refreshDatabase({ client, fs, fm, folderPath: "output/DB" });

		expect(result.deleted).toBe(1);
		expect(fs.writeFile).toHaveBeenCalledWith(
			"output/DB/Deleted.md",
			expect.stringContaining("notion-deleted: true")
		);
	});

	it("does not double-mark already deleted entries", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		// Mock _database.json read and already-deleted file read
		const dbMetadata = JSON.stringify({
			databaseId: "db-123",
			title: "DB",
			url: "https://www.notion.so/db-123",
			folderPath: "output/DB",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 1,
		});
		(fs.readFile as any).mockImplementation((path: string) => {
			if (path.includes("_database.json")) {
				return Promise.resolve(dbMetadata);
			}
			return Promise.resolve("---\nnotion-id: gone-id\nnotion-deleted: true\n---\nOld content");
		});

		(fs.listMarkdownFiles as any).mockResolvedValue(["Gone.md"]);
		(fm.readFrontmatter as any).mockResolvedValue({ "notion-id": "gone-id" });

		const result = await refreshDatabase({ client, fs, fm, folderPath: "output/DB" });

		expect(result.deleted).toBe(1);
		// writeFile should only be called for _database.json update, not for marking deleted
		expect(fs.writeFile).toHaveBeenCalledTimes(1);
		expect(fs.writeFile).toHaveBeenCalledWith(
			"output/DB/_database.json",
			expect.any(String)
		);
	});

	it("calls progress callback with phases including diffing", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [makeEntry("e-1", "E1"), makeEntry("e-2", "E2")],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();
		const onProgress = vi.fn();

		// Mock _database.json read
		const dbMetadata = JSON.stringify({
			databaseId: "db-123",
			title: "DB",
			folderPath: "output/DB",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 2,
		});
		(fs.readFile as any).mockResolvedValue(dbMetadata);

		await refreshDatabase({ client, fs, fm, folderPath: "output/DB" }, onProgress);

		expect(onProgress).toHaveBeenCalledWith({ phase: "querying" });
		expect(onProgress).toHaveBeenCalledWith({ phase: "diffing", total: 2 });
		expect(onProgress).toHaveBeenCalledWith({ phase: "stale-detected", stale: 2, total: 2 });
		expect(onProgress).toHaveBeenCalledWith({ phase: "complete" });
	});

	it("builds correct notion-id map from scanLocalFiles", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("DB")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [makeEntry("e-1", "E1")],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		// Mock _database.json read and orphan file read
		const dbMetadata = JSON.stringify({
			databaseId: "db-123",
			title: "DB",
			url: "https://www.notion.so/db-123",
			folderPath: "output/DB",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 3,
		});
		(fs.readFile as any).mockImplementation((path: string) => {
			if (path.includes("_database.json")) {
				return Promise.resolve(dbMetadata);
			}
			return Promise.resolve("---\nnotion-id: e-orphan\n---\nContent");
		});

		// Two local files, one without notion-id
		(fs.listMarkdownFiles as any).mockResolvedValue(["A.md", "B.md", "C.md"]);
		(fm.readFrontmatter as any)
			.mockResolvedValueOnce({ "notion-id": "e-1", "notion-last-edited": "2024-01-15T10:00:00.000Z" })
			.mockResolvedValueOnce({ "notion-id": "e-orphan" })
			.mockResolvedValueOnce(null); // C.md has no frontmatter

		const result = await refreshDatabase({ client, fs, fm, folderPath: "output/DB" });

		// e-orphan should be marked as deleted, C.md (no notion-id) should be ignored
		expect(result.deleted).toBe(1);
	});

	it("throws error when _database.json is missing", async () => {
		const client = {} as unknown as Client;
		const fs = mockFs();
		const fm = mockFm();

		// Mock _database.json not found
		(fs.readFile as any).mockRejectedValue(new Error("File not found"));

		await expect(
			refreshDatabase({ client, fs, fm, folderPath: "output/DB" })
		).rejects.toThrow("No _database.json found");
	});
});

describe("readDatabaseMetadata", () => {
	it("reads and parses _database.json", async () => {
		const fs = mockFs();
		const metadata = {
			databaseId: "db-123",
			title: "My Database",
			url: "https://www.notion.so/db-123",
			folderPath: "output/My Database",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 5,
		};
		(fs.readFile as any).mockResolvedValue(JSON.stringify(metadata));

		const result = await readDatabaseMetadata(fs, "output/My Database");

		expect(result).toEqual(metadata);
		expect(fs.readFile).toHaveBeenCalledWith("output/My Database/_database.json");
	});

	it("returns null when file does not exist", async () => {
		const fs = mockFs();
		(fs.readFile as any).mockRejectedValue(new Error("File not found"));

		const result = await readDatabaseMetadata(fs, "output/Missing");

		expect(result).toBeNull();
	});
});

describe("listSyncedDatabases", () => {
	it("lists all databases with _database.json files", async () => {
		const fs = mockFs();
		(fs.listDirectories as any).mockResolvedValue(["DB1", "DB2", "NotADb"]);

		const db1Meta = {
			databaseId: "db-1",
			title: "DB1",
			url: "https://www.notion.so/db-1",
			folderPath: "output/DB1",
			lastSyncedAt: "2024-01-15T10:00:00.000Z",
			entryCount: 3,
		};
		const db2Meta = {
			databaseId: "db-2",
			title: "DB2",
			url: "https://www.notion.so/db-2",
			folderPath: "output/DB2",
			lastSyncedAt: "2024-01-16T10:00:00.000Z",
			entryCount: 5,
		};

		(fs.readFile as any).mockImplementation((path: string) => {
			if (path.includes("DB1/_database.json")) {
				return Promise.resolve(JSON.stringify(db1Meta));
			}
			if (path.includes("DB2/_database.json")) {
				return Promise.resolve(JSON.stringify(db2Meta));
			}
			return Promise.reject(new Error("File not found"));
		});

		const result = await listSyncedDatabases(fs, "output");

		expect(result).toHaveLength(2);
		expect(result).toContainEqual(db1Meta);
		expect(result).toContainEqual(db2Meta);
	});

	it("returns empty array when output folder does not exist", async () => {
		const fs = mockFs();
		(fs.listDirectories as any).mockRejectedValue(new Error("Directory not found"));

		const result = await listSyncedDatabases(fs, "output");

		expect(result).toEqual([]);
	});
});

describe("freshDatabaseImport metadata", () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it("writes _database.json after import", async () => {
		const client = {
			databases: {
				retrieve: vi.fn().mockResolvedValue(makeDatabase("My Database")),
			},
			dataSources: {
				retrieve: vi.fn().mockResolvedValue({ id: "ds-1", title: [] }),
				query: vi.fn().mockResolvedValue({
					results: [makeEntry("e-1", "E1")],
					has_more: false,
				}),
			},
		} as unknown as Client;

		const fs = mockFs();
		const fm = mockFm();

		await freshDatabaseImport({
			client, fs, fm,
			databaseId: "db-123",
			outputFolder: "output",
		});

		// Should write _database.json
		expect(fs.writeFile).toHaveBeenCalledWith(
			"output/My Database/_database.json",
			expect.stringContaining("db-123")
		);

		// Verify the content structure
		const writeCall = (fs.writeFile as any).mock.calls.find(
			(call: any[]) => call[0].includes("_database.json")
		);
		expect(writeCall).toBeDefined();
		const metadata = JSON.parse(writeCall[1]);
		expect(metadata.databaseId).toBe("db-123");
		expect(metadata.title).toBe("My Database");
		expect(metadata.folderPath).toBe("output/My Database");
		expect(metadata.entryCount).toBe(1);
		expect(metadata.lastSyncedAt).toBeDefined();
	});
});
