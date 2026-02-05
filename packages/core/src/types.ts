import { Client } from "@notionhq/client";
import { PageObjectResponse } from "@notionhq/client/build/src/api-endpoints";

export interface FileSystem {
	readFile(path: string): Promise<string>;
	writeFile(path: string, content: string): Promise<void>;
	fileExists(path: string): Promise<boolean>;
	mkdir(path: string, recursive?: boolean): Promise<void>;
	listMarkdownFiles(dir: string): Promise<string[]>;
	listDirectories(dir: string): Promise<string[]>;
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

export interface FreezeOptions {
	client: Client;
	fs: FileSystem;
	fm: FrontmatterReader;
	outputFolder: string;
	notionId: string;
	databaseId?: string;
	page?: PageObjectResponse;
}

export interface PageFreezeResult {
	status: "created" | "updated" | "skipped";
	filePath: string;
	title: string;
}

export interface DatabaseFreezeResult {
	title: string;
	folderPath: string;
	total: number;
	created: number;
	updated: number;
	skipped: number;
	deleted: number;
	failed: number;
	errors: string[];
}

export type ProgressPhase =
	| { phase: "querying" }
	| { phase: "diffing"; total: number }
	| { phase: "stale-detected"; stale: number; total: number }
	| { phase: "importing"; current: number; total: number; title: string }
	| { phase: "complete" };

export type ProgressCallback = (progress: ProgressPhase) => void;

export interface FrozenDatabase {
	databaseId: string;
	title: string;
	url: string;
	folderPath: string;
	lastSyncedAt: string;
	entryCount: number;
}

export const DATABASE_METADATA_FILE = "_database.json";
