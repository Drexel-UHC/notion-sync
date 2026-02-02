import { describe, it, expect, vi } from "vitest";
import { convertBlocksToMarkdown, convertRichText, fetchAllChildren } from "../src/block-converter.js";
import { Client } from "@notionhq/client";
import type { BlockObjectResponse, RichTextItemResponse } from "@notionhq/client/build/src/api-endpoints";

// Mock notion-client to skip throttle/retry in tests
vi.mock("../src/notion-client.js", () => ({
	notionRequest: <T>(fn: () => Promise<T>) => fn(),
}));

function makeBlock(overrides: Partial<BlockObjectResponse> & { type: string }): BlockObjectResponse {
	return {
		object: "block",
		id: "block-id",
		parent: { type: "page_id", page_id: "page-id" },
		created_time: "2024-01-01T00:00:00.000Z",
		last_edited_time: "2024-01-01T00:00:00.000Z",
		created_by: { object: "user", id: "user-id" },
		last_edited_by: { object: "user", id: "user-id" },
		has_children: false,
		archived: false,
		in_trash: false,
		...overrides,
	} as BlockObjectResponse;
}

function makeRichText(content: string, annotations?: Partial<RichTextItemResponse["annotations"]>): RichTextItemResponse {
	return {
		type: "text",
		text: { content, link: null },
		annotations: {
			bold: false,
			italic: false,
			strikethrough: false,
			underline: false,
			code: false,
			color: "default",
			...annotations,
		},
		plain_text: content,
		href: null,
	} as RichTextItemResponse;
}

const mockClient = {} as Client;
const ctx = { client: mockClient, indentLevel: 0 };

describe("convertRichText", () => {
	it("converts plain text", () => {
		const result = convertRichText([makeRichText("Hello world")]);
		expect(result).toBe("Hello world");
	});

	it("converts bold text", () => {
		const result = convertRichText([makeRichText("bold", { bold: true })]);
		expect(result).toBe("**bold**");
	});

	it("converts italic text", () => {
		const result = convertRichText([makeRichText("italic", { italic: true })]);
		expect(result).toBe("*italic*");
	});

	it("converts strikethrough text", () => {
		const result = convertRichText([makeRichText("deleted", { strikethrough: true })]);
		expect(result).toBe("~~deleted~~");
	});

	it("converts code text", () => {
		const result = convertRichText([makeRichText("code", { code: true })]);
		expect(result).toBe("`code`");
	});

	it("converts underline text", () => {
		const result = convertRichText([makeRichText("underlined", { underline: true })]);
		expect(result).toBe("<u>underlined</u>");
	});

	it("converts highlighted text", () => {
		const result = convertRichText([makeRichText("highlight", { color: "yellow_background" as any })]);
		expect(result).toBe("==highlight==");
	});

	it("converts linked text", () => {
		const item: RichTextItemResponse = {
			type: "text",
			text: { content: "click here", link: { url: "https://example.com" } },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "click here",
			href: "https://example.com",
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("[click here](https://example.com)");
	});

	it("converts multiple rich text items", () => {
		const result = convertRichText([
			makeRichText("Hello "),
			makeRichText("world", { bold: true }),
		]);
		expect(result).toBe("Hello **world**");
	});

	it("converts equation inline", () => {
		const item: RichTextItemResponse = {
			type: "equation",
			equation: { expression: "E=mc^2" },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "E=mc^2",
			href: null,
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("$E=mc^2$");
	});

	it("converts mention of user", () => {
		const item: RichTextItemResponse = {
			type: "mention",
			mention: { type: "user", user: { object: "user", id: "u1" } },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "John Doe",
			href: null,
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("@John Doe");
	});

	it("converts mention of page", () => {
		const item: RichTextItemResponse = {
			type: "mention",
			mention: { type: "page", page: { id: "page-123" } },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "Some Page",
			href: null,
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("[[notion-id: page-123]]");
	});

	it("converts mention of date", () => {
		const item: RichTextItemResponse = {
			type: "mention",
			mention: { type: "date", date: { start: "2024-01-01", end: null, time_zone: null } },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "2024-01-01",
			href: null,
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("2024-01-01");
	});

	it("converts date range mention", () => {
		const item: RichTextItemResponse = {
			type: "mention",
			mention: { type: "date", date: { start: "2024-01-01", end: "2024-01-31", time_zone: null } },
			annotations: {
				bold: false, italic: false, strikethrough: false,
				underline: false, code: false, color: "default",
			},
			plain_text: "2024-01-01 → 2024-01-31",
			href: null,
		} as RichTextItemResponse;
		const result = convertRichText([item]);
		expect(result).toBe("2024-01-01 → 2024-01-31");
	});
});

describe("convertBlocksToMarkdown", () => {
	it("converts paragraph", async () => {
		const blocks = [makeBlock({
			type: "paragraph",
			paragraph: { rich_text: [makeRichText("Hello")], color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("Hello");
	});

	it("converts heading_1", async () => {
		const blocks = [makeBlock({
			type: "heading_1",
			heading_1: { rich_text: [makeRichText("Title")], is_toggleable: false, color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("# Title");
	});

	it("converts heading_2", async () => {
		const blocks = [makeBlock({
			type: "heading_2",
			heading_2: { rich_text: [makeRichText("Subtitle")], is_toggleable: false, color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("## Subtitle");
	});

	it("converts heading_3", async () => {
		const blocks = [makeBlock({
			type: "heading_3",
			heading_3: { rich_text: [makeRichText("Section")], is_toggleable: false, color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("### Section");
	});

	it("converts bulleted_list_item", async () => {
		const blocks = [makeBlock({
			type: "bulleted_list_item",
			bulleted_list_item: { rich_text: [makeRichText("Item 1")], color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("- Item 1");
	});

	it("converts numbered_list_item with correct numbering", async () => {
		const blocks = [
			makeBlock({
				type: "numbered_list_item",
				numbered_list_item: { rich_text: [makeRichText("First")], color: "default" },
			}),
			makeBlock({
				type: "numbered_list_item",
				numbered_list_item: { rich_text: [makeRichText("Second")], color: "default" },
			}),
		];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("1. First\n2. Second");
	});

	it("resets numbered list counter after different block type", async () => {
		const blocks = [
			makeBlock({
				type: "numbered_list_item",
				numbered_list_item: { rich_text: [makeRichText("First")], color: "default" },
			}),
			makeBlock({
				type: "paragraph",
				paragraph: { rich_text: [makeRichText("Break")], color: "default" },
			}),
			makeBlock({
				type: "numbered_list_item",
				numbered_list_item: { rich_text: [makeRichText("Again first")], color: "default" },
			}),
		];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("1. First\nBreak\n1. Again first");
	});

	it("converts to_do checked", async () => {
		const blocks = [makeBlock({
			type: "to_do",
			to_do: { rich_text: [makeRichText("Done")], checked: true, color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("- [x] Done");
	});

	it("converts to_do unchecked", async () => {
		const blocks = [makeBlock({
			type: "to_do",
			to_do: { rich_text: [makeRichText("Todo")], checked: false, color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("- [ ] Todo");
	});

	it("converts code block", async () => {
		const blocks = [makeBlock({
			type: "code",
			code: { rich_text: [makeRichText("const x = 1;")], language: "javascript", caption: [] },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("```javascript\nconst x = 1;\n```");
	});

	it("converts code block with plain text language", async () => {
		const blocks = [makeBlock({
			type: "code",
			code: { rich_text: [makeRichText("hello")], language: "plain text", caption: [] },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("```\nhello\n```");
	});

	it("converts quote", async () => {
		const blocks = [makeBlock({
			type: "quote",
			quote: { rich_text: [makeRichText("A quote")], color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("> A quote");
	});

	it("converts callout with emoji", async () => {
		const blocks = [makeBlock({
			type: "callout",
			callout: {
				rich_text: [makeRichText("Important note")],
				icon: { type: "emoji", emoji: "💡" },
				color: "default",
			},
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("> [!tip]\n> Important note");
	});

	it("converts callout without icon", async () => {
		const blocks = [makeBlock({
			type: "callout",
			callout: {
				rich_text: [makeRichText("Note")],
				icon: null,
				color: "default",
			},
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("> [!info]\n> Note");
	});

	it("converts equation block", async () => {
		const blocks = [makeBlock({
			type: "equation",
			equation: { expression: "E = mc^2" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("$$\nE = mc^2\n$$");
	});

	it("converts divider", async () => {
		const blocks = [makeBlock({ type: "divider", divider: {} })];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("---");
	});

	it("converts child_page", async () => {
		const blocks = [makeBlock({
			type: "child_page",
			child_page: { title: "Sub Page" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("[[Sub Page]]");
	});

	it("converts child_database", async () => {
		const blocks = [makeBlock({
			type: "child_database",
			child_database: { title: "My DB" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("<!-- child database: My DB -->");
	});

	it("converts image with external URL", async () => {
		const blocks = [makeBlock({
			type: "image",
			image: {
				type: "external",
				external: { url: "https://example.com/img.png" },
				caption: [],
			},
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("![image](https://example.com/img.png)");
	});

	it("converts image with caption", async () => {
		const blocks = [makeBlock({
			type: "image",
			image: {
				type: "external",
				external: { url: "https://example.com/img.png" },
				caption: [makeRichText("My photo")],
			},
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("![My photo](https://example.com/img.png)");
	});

	it("converts bookmark", async () => {
		const blocks = [makeBlock({
			type: "bookmark",
			bookmark: { url: "https://example.com", caption: [] },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("https://example.com");
	});

	it("converts bookmark with caption", async () => {
		const blocks = [makeBlock({
			type: "bookmark",
			bookmark: { url: "https://example.com", caption: [makeRichText("Example")] },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("[Example](https://example.com)");
	});

	it("converts embed", async () => {
		const blocks = [makeBlock({
			type: "embed",
			embed: { url: "https://youtube.com/watch?v=123", caption: [] },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("https://youtube.com/watch?v=123");
	});

	it("converts video", async () => {
		const blocks = [makeBlock({
			type: "video",
			video: {
				type: "external",
				external: { url: "https://youtube.com/watch?v=abc" },
				caption: [],
			},
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("https://youtube.com/watch?v=abc");
	});

	it("converts link_to_page (page)", async () => {
		const blocks = [makeBlock({
			type: "link_to_page",
			link_to_page: { type: "page_id", page_id: "linked-page-id" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("[[notion-id: linked-page-id]]");
	});

	it("converts link_to_page (database)", async () => {
		const blocks = [makeBlock({
			type: "link_to_page",
			link_to_page: { type: "database_id", database_id: "db-id" },
		})];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("<!-- linked database: db-id -->");
	});

	it("skips table_of_contents", async () => {
		const blocks = [makeBlock({ type: "table_of_contents", table_of_contents: { color: "default" } })];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("");
	});

	it("skips breadcrumb", async () => {
		const blocks = [makeBlock({ type: "breadcrumb", breadcrumb: {} })];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("");
	});

	it("handles nested bulleted list via indentation", async () => {
		const indentedCtx = { client: mockClient, indentLevel: 1 };
		const blocks = [makeBlock({
			type: "bulleted_list_item",
			bulleted_list_item: { rich_text: [makeRichText("Nested item")], color: "default" },
		})];
		const result = await convertBlocksToMarkdown(blocks, indentedCtx);
		expect(result).toBe("    - Nested item");
	});

	it("converts multiple block types in sequence", async () => {
		const blocks = [
			makeBlock({
				type: "heading_1",
				heading_1: { rich_text: [makeRichText("Title")], is_toggleable: false, color: "default" },
			}),
			makeBlock({
				type: "paragraph",
				paragraph: { rich_text: [makeRichText("Some text")], color: "default" },
			}),
			makeBlock({
				type: "bulleted_list_item",
				bulleted_list_item: { rich_text: [makeRichText("Point A")], color: "default" },
			}),
			makeBlock({
				type: "bulleted_list_item",
				bulleted_list_item: { rich_text: [makeRichText("Point B")], color: "default" },
			}),
		];
		const result = await convertBlocksToMarkdown(blocks, ctx);
		expect(result).toBe("# Title\nSome text\n- Point A\n- Point B");
	});
});

describe("fetchAllChildren", () => {
	it("paginates through children", async () => {
		const mockBlocks = [
			makeBlock({ type: "paragraph", paragraph: { rich_text: [makeRichText("A")], color: "default" } }),
			makeBlock({ type: "paragraph", paragraph: { rich_text: [makeRichText("B")], color: "default" } }),
		];

		const client = {
			blocks: {
				children: {
					list: vi.fn()
						.mockResolvedValueOnce({
							results: [mockBlocks[0]],
							has_more: true,
							next_cursor: "cursor-1",
						})
						.mockResolvedValueOnce({
							results: [mockBlocks[1]],
							has_more: false,
							next_cursor: null,
						}),
				},
			},
		} as unknown as Client;

		const result = await fetchAllChildren(client, "block-id");
		expect(result).toHaveLength(2);
		expect(client.blocks.children.list).toHaveBeenCalledTimes(2);
	});
});
