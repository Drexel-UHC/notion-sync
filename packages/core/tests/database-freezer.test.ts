import { describe, it, expect, vi, beforeEach } from "vitest";
import { freezeDatabase } from "../src/database-freezer.js";
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
		title: [{ type: "text", text: { content: title }, plain_text: title }],
		data_sources: [{ id: "ds-1" }],
	};
}

function makeEntry(id: string, title: string) {
	return {
		object: "page",
		id,
		properties: {
			Name: { type: "title", title: [{ plain_text: title }] },
		},
	};
}

describe("freezeDatabase", () => {
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

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

		expect(result.title).toBe("My Database");
		expect(result.folderPath).toBe("output/My Database");
		expect(result.created).toBe(2);
		expect(result.failed).toBe(0);
		expect(fs.mkdir).toHaveBeenCalledWith("output/My Database", true);
		expect(freezePage).toHaveBeenCalledTimes(2);
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

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

		expect(result.created).toBe(2);
		expect(client.dataSources.query).toHaveBeenCalledTimes(2);
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

		// Simulate existing local files with a deleted entry
		(fs.listMarkdownFiles as any).mockResolvedValue(["E1.md", "Deleted.md"]);
		(fm.readFrontmatter as any)
			.mockResolvedValueOnce({ "notion-id": "e-1" }) // E1.md
			.mockResolvedValueOnce({ "notion-id": "e-deleted" }); // Deleted.md

		// readFile for markAsDeleted
		(fs.readFile as any).mockResolvedValue("---\nnotion-id: e-deleted\n---\nContent");

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

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

		(fs.listMarkdownFiles as any).mockResolvedValue(["Gone.md"]);
		(fm.readFrontmatter as any).mockResolvedValue({ "notion-id": "gone-id" });
		(fs.readFile as any).mockResolvedValue("---\nnotion-id: gone-id\nnotion-deleted: true\n---\nOld content");

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

		expect(result.deleted).toBe(1);
		// writeFile should NOT be called because already marked
		expect(fs.writeFile).not.toHaveBeenCalled();
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

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

		expect(result.created).toBe(1);
		expect(result.failed).toBe(1);
		expect(result.errors).toHaveLength(1);
		expect(result.errors[0]).toContain("API error");
	});

	it("calls progress callback", async () => {
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

		await freezeDatabase(
			{ client, fs, fm, notionId: "db-123", outputFolder: "output" },
			onProgress
		);

		expect(onProgress).toHaveBeenCalledTimes(2);
		expect(onProgress).toHaveBeenCalledWith(1, 2, "DB");
		expect(onProgress).toHaveBeenCalledWith(2, 2, "DB");
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

		await expect(freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		})).rejects.toThrow("linked database");
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

		// Two local files, one without notion-id
		(fs.listMarkdownFiles as any).mockResolvedValue(["A.md", "B.md", "C.md"]);
		(fm.readFrontmatter as any)
			.mockResolvedValueOnce({ "notion-id": "e-1" })
			.mockResolvedValueOnce({ "notion-id": "e-orphan" })
			.mockResolvedValueOnce(null); // C.md has no frontmatter

		(fs.readFile as any).mockResolvedValue("---\nnotion-id: e-orphan\n---\nContent");

		const result = await freezeDatabase({
			client, fs, fm,
			notionId: "db-123",
			outputFolder: "output",
		});

		// e-orphan should be marked as deleted, C.md (no notion-id) should be ignored
		expect(result.deleted).toBe(1);
	});
});
