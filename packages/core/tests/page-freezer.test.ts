import { describe, it, expect, vi, beforeEach } from "vitest";
import { freezePage } from "../src/page-freezer.js";
import { Client } from "@notionhq/client";
import type { FileSystem, FrontmatterReader } from "../src/types.js";

// Mock notion-client to skip throttle/retry
vi.mock("../src/notion-client.js", () => ({
	notionRequest: <T>(fn: () => Promise<T>) => fn(),
}));

// Mock block-converter
vi.mock("../src/block-converter.js", () => ({
	convertBlocksToMarkdown: vi.fn().mockResolvedValue("# Page content"),
	convertRichText: vi.fn((items: any[]) => items.map((i: any) => i.plain_text || i.text?.content || "").join("")),
	fetchAllChildren: vi.fn().mockResolvedValue([]),
}));

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

function makePage(title: string, properties: Record<string, any> = {}) {
	return {
		object: "page",
		id: "page-123",
		url: "https://notion.so/page-123",
		last_edited_time: "2024-01-15T10:00:00.000Z",
		properties: {
			Name: {
				type: "title",
				title: [{ type: "text", text: { content: title }, plain_text: title }],
			},
			...properties,
		},
	};
}

function mockClient(page: any): Client {
	return {
		pages: {
			retrieve: vi.fn().mockResolvedValue(page),
		},
		blocks: {
			children: {
				list: vi.fn().mockResolvedValue({ results: [], has_more: false }),
			},
		},
	} as unknown as Client;
}

describe("freezePage", () => {
	it("creates a new file for a new page", async () => {
		const page = makePage("My Page");
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		const result = await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
		});

		expect(result.status).toBe("created");
		expect(result.filePath).toBe("output/My Page.md");
		expect(result.title).toBe("My Page");
		expect(fs.mkdir).toHaveBeenCalledWith("output", true);
		expect(fs.writeFile).toHaveBeenCalledOnce();

		const written = (fs.writeFile as any).mock.calls[0][1] as string;
		expect(written).toContain("---");
		expect(written).toContain("notion-id: page-123");
		expect(written).toContain('notion-url: "https://notion.so/page-123"');
		expect(written).toContain('notion-last-edited: "2024-01-15T10:00:00.000Z"');
		expect(written).toContain("# Page content");
	});

	it("updates an existing file", async () => {
		const page = makePage("My Page");
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		(fs.fileExists as any).mockResolvedValue(true);
		(fm.readFrontmatter as any).mockResolvedValue({
			"notion-id": "page-123",
			"notion-last-edited": "2024-01-10T00:00:00.000Z", // older
		});

		const result = await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
		});

		expect(result.status).toBe("updated");
		expect(fs.writeFile).toHaveBeenCalledOnce();
		expect(fs.mkdir).not.toHaveBeenCalled();
	});

	it("skips when last_edited_time matches", async () => {
		const page = makePage("My Page");
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		(fs.fileExists as any).mockResolvedValue(true);
		(fm.readFrontmatter as any).mockResolvedValue({
			"notion-id": "page-123",
			"notion-last-edited": "2024-01-15T10:00:00.000Z", // same
		});

		const result = await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
		});

		expect(result.status).toBe("skipped");
		expect(fs.writeFile).not.toHaveBeenCalled();
	});

	it("includes database-id in frontmatter when provided", async () => {
		const page = makePage("Entry");
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
			databaseId: "db-456",
		});

		const written = (fs.writeFile as any).mock.calls[0][1] as string;
		expect(written).toContain("notion-database-id: db-456");
	});

	it("sanitizes filenames with special characters", async () => {
		const page = makePage("My: Page / Test");
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		const result = await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
		});

		expect(result.filePath).toBe("output/My- Page - Test.md");
	});

	it("maps database properties to frontmatter", async () => {
		const page = makePage("Entry", {
			Status: { type: "select", select: { name: "Done" } },
			Tags: { type: "multi_select", multi_select: [{ name: "tag1" }, { name: "tag2" }] },
			Count: { type: "number", number: 42 },
			Done: { type: "checkbox", checkbox: true },
			Website: { type: "url", url: "https://example.com" },
			Email: { type: "email", email: "test@example.com" },
			Phone: { type: "phone_number", phone_number: "+1234567890" },
			"Due Date": { type: "date", date: { start: "2024-03-01", end: null } },
			"Date Range": { type: "date", date: { start: "2024-01-01", end: "2024-12-31" } },
			"No Date": { type: "date", date: null },
			Created: { type: "created_time", created_time: "2024-01-01T00:00:00.000Z" },
			Edited: { type: "last_edited_time", last_edited_time: "2024-06-15T00:00:00.000Z" },
			Notes: { type: "rich_text", rich_text: [{ type: "text", text: { content: "Some notes" }, plain_text: "Some notes" }] },
			Related: { type: "relation", relation: [{ id: "rel-1" }, { id: "rel-2" }] },
			People: { type: "people", people: [{ id: "u1", name: "Alice" }, { id: "u2", name: null }] },
			Attachments: {
				type: "files", files: [
					{ name: "doc.pdf", type: "external", external: { url: "https://example.com/doc.pdf" } },
				],
			},
			StatusProp: { type: "status", status: { name: "In Progress" } },
		});
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
			databaseId: "db-456",
		});

		const written = (fs.writeFile as any).mock.calls[0][1] as string;
		expect(written).toContain("Status: Done");
		expect(written).toContain("Count: 42");
		expect(written).toContain("Done: true");
		expect(written).toContain('Website: "https://example.com"');
		expect(written).toContain("Email: test@example.com");
		expect(written).toContain("Phone: +1234567890");
		expect(written).toContain('"Due Date": 2024-03-01');
		expect(written).toContain('"No Date": null');
		expect(written).toContain("Tags:");
		expect(written).toContain("  - tag1");
		expect(written).toContain("  - tag2");
		expect(written).toContain("Related:");
		expect(written).toContain("  - rel-1");
		expect(written).toContain("People:");
		expect(written).toContain("  - Alice");
		expect(written).toContain("  - u2");
	});

	it("handles untitled page", async () => {
		const page = {
			object: "page",
			id: "page-123",
			url: "https://notion.so/page-123",
			last_edited_time: "2024-01-15T10:00:00.000Z",
			properties: {},
		};
		const client = mockClient(page);
		const fs = mockFs();
		const fm = mockFm();

		const result = await freezePage({
			client, fs, fm,
			notionId: "page-123",
			outputFolder: "output",
		});

		expect(result.filePath).toBe("output/Untitled.md");
	});
});
