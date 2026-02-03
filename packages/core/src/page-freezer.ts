import { Client } from "@notionhq/client";
import {
	PageObjectResponse,
	RichTextItemResponse,
} from "@notionhq/client/build/src/api-endpoints";
import { FileSystem, FrontmatterReader, FreezeOptions, PageFreezeResult } from "./types.js";
import { notionRequest } from "./notion-client.js";
import { convertBlocksToMarkdown, convertRichText, fetchAllChildren } from "./block-converter.js";
import { sanitizeFileName, joinPath } from "./utils.js";

export async function freezePage(
	options: FreezeOptions
): Promise<PageFreezeResult> {
	const { client, fs, fm, notionId, outputFolder, databaseId } = options;

	// Use pre-fetched page if provided, otherwise fetch from API
	const page = options.page ?? (await notionRequest(() =>
		client.pages.retrieve({ page_id: notionId })
	)) as PageObjectResponse;

	const title = getPageTitle(page);
	const safeName = sanitizeFileName(title || "Untitled");
	const filePath = joinPath(outputFolder, safeName + ".md");

	// Check for re-freeze: compare last_edited_time
	const exists = await fs.fileExists(filePath);
	if (exists) {
		const frontmatter = await fm.readFrontmatter(filePath);
		const storedEdited = frontmatter?.["notion-last-edited"];
		if (storedEdited && storedEdited === page.last_edited_time) {
			return { status: "skipped", filePath, title: safeName };
		}
	}

	// Fetch all blocks
	const blocks = await fetchAllChildren(client, notionId);
	const markdown = await convertBlocksToMarkdown(blocks, {
		client,
		indentLevel: 0,
	});

	// Build frontmatter
	const frontmatter: Record<string, unknown> = {
		"notion-id": notionId,
		"notion-url": page.url,
		"notion-frozen-at": new Date().toISOString(),
		"notion-last-edited": page.last_edited_time,
	};
	if (databaseId) {
		frontmatter["notion-database-id"] = databaseId;
	}

	// Map database entry properties to frontmatter
	if (databaseId) {
		mapPropertiesToFrontmatter(page.properties, frontmatter);
	}

	const content = buildFileContent(frontmatter, markdown);

	// Write file
	if (exists) {
		await fs.writeFile(filePath, content);
		return { status: "updated", filePath, title: safeName };
	} else {
		await fs.mkdir(outputFolder, true);
		await fs.writeFile(filePath, content);
		return { status: "created", filePath, title: safeName };
	}
}

function getPageTitle(page: PageObjectResponse): string {
	for (const prop of Object.values(page.properties)) {
		if (prop.type === "title") {
			return convertRichText(prop.title as RichTextItemResponse[]);
		}
	}
	return "Untitled";
}

function mapPropertiesToFrontmatter(
	properties: PageObjectResponse["properties"],
	frontmatter: Record<string, unknown>
): void {
	for (const [key, prop] of Object.entries(properties)) {
		switch (prop.type) {
			case "title":
				// Already used as filename, skip
				break;
			case "rich_text":
				frontmatter[key] = convertRichText(
					prop.rich_text as RichTextItemResponse[]
				);
				break;
			case "number":
				frontmatter[key] = prop.number;
				break;
			case "select":
				frontmatter[key] = prop.select?.name ?? null;
				break;
			case "multi_select":
				frontmatter[key] = prop.multi_select.map(
					(s: { name: string }) => s.name
				);
				break;
			case "status":
				frontmatter[key] = prop.status?.name ?? null;
				break;
			case "date":
				if (prop.date) {
					frontmatter[key] = prop.date.end
						? `${prop.date.start} → ${prop.date.end}`
						: prop.date.start;
				} else {
					frontmatter[key] = null;
				}
				break;
			case "checkbox":
				frontmatter[key] = prop.checkbox;
				break;
			case "url":
				frontmatter[key] = prop.url;
				break;
			case "email":
				frontmatter[key] = prop.email;
				break;
			case "phone_number":
				frontmatter[key] = prop.phone_number;
				break;
			case "relation":
				frontmatter[key] = prop.relation.map(
					(r: { id: string }) => r.id
				);
				break;
			case "people":
				frontmatter[key] = prop.people.map(
					(p: { id: string; name?: string | null }) => p.name || p.id
				);
				break;
			case "files":
				frontmatter[key] = prop.files.map(
					(f: { name: string; type: string; file?: { url: string }; external?: { url: string } }) =>
						f.type === "file" ? f.file?.url : f.external?.url
				);
				break;
			case "created_time":
				frontmatter[key] = prop.created_time;
				break;
			case "last_edited_time":
				frontmatter[key] = prop.last_edited_time;
				break;
			// Skip formula, rollup, button, unique_id, verification — complex or non-user types
			default:
				break;
		}
	}
}

function buildFileContent(
	frontmatter: Record<string, unknown>,
	body: string
): string {
	const yamlLines: string[] = ["---"];
	for (const [key, value] of Object.entries(frontmatter)) {
		yamlLines.push(formatYamlEntry(key, value));
	}
	yamlLines.push("---");
	return yamlLines.join("\n") + "\n" + body;
}

function formatYamlEntry(key: string, value: unknown): string {
	const safeKey = key.includes(":") || key.includes(" ") ? `"${key}"` : key;

	if (value === null || value === undefined) {
		return `${safeKey}: null`;
	}
	if (typeof value === "boolean") {
		return `${safeKey}: ${value}`;
	}
	if (typeof value === "number") {
		return `${safeKey}: ${value}`;
	}
	if (Array.isArray(value)) {
		if (value.length === 0) return `${safeKey}: []`;
		const items = value.map((v) => `  - ${yamlEscapeString(String(v))}`);
		return `${safeKey}:\n${items.join("\n")}`;
	}
	return `${safeKey}: ${yamlEscapeString(String(value))}`;
}

function yamlEscapeString(str: string): string {
	if (
		str.includes(":") ||
		str.includes("#") ||
		str.includes("'") ||
		str.includes('"') ||
		str.includes("\n") ||
		str.startsWith(" ") ||
		str.startsWith("-") ||
		str.startsWith("[") ||
		str.startsWith("{") ||
		str === "true" ||
		str === "false" ||
		str === "null" ||
		/^\d+$/.test(str)
	) {
		return `"${str.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n")}"`;
	}
	return str;
}
