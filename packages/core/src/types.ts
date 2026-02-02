import { Client } from "@notionhq/client";

export interface FileSystem {
	readFile(path: string): Promise<string>;
	writeFile(path: string, content: string): Promise<void>;
	fileExists(path: string): Promise<boolean>;
	mkdir(path: string, recursive?: boolean): Promise<void>;
	listMarkdownFiles(dir: string): Promise<string[]>;
}

export interface FrontmatterReader {
	readFrontmatter(filePath: string): Promise<Record<string, unknown> | null>;
}

export interface FreezeFrontmatter {
	"notion-id": string;
	"notion-url": string;
	"notion-frozen-at": string;
	"notion-last-edited": string;
	"notion-database-id"?: string;
	"notion-deleted"?: boolean;
	[key: string]: unknown;
}

export type DetectionResult =
	| { type: "page"; id: string }
	| { type: "database"; id: string };

export interface FreezeOptions {
	client: Client;
	fs: FileSystem;
	fm: FrontmatterReader;
	outputFolder: string;
	notionId: string;
	databaseId?: string;
}

export interface PageFreezeResult {
	status: "created" | "updated" | "skipped";
	filePath: string;
	title: string;
}

export interface DatabaseFreezeResult {
	title: string;
	folderPath: string;
	created: number;
	updated: number;
	skipped: number;
	deleted: number;
	failed: number;
	errors: string[];
}

export type ProgressCallback = (current: number, total: number, title: string) => void;
