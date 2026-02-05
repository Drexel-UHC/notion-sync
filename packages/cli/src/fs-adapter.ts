import { readFile, writeFile, stat, readdir, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { FileSystem } from "@notion-sync/core";

export const nodeFs: FileSystem = {
	readFile: (p) => readFile(resolve(p), "utf-8"),

	writeFile: async (p, content) => {
		const resolved = resolve(p);
		await mkdir(dirname(resolved), { recursive: true });
		await writeFile(resolved, content, "utf-8");
	},

	fileExists: async (p) => {
		try {
			await stat(resolve(p));
			return true;
		} catch {
			return false;
		}
	},

	mkdir: (p, recursive) => mkdir(resolve(p), { recursive }).then(() => {}),

	listMarkdownFiles: async (dir: string) => {
		const entries = await readdir(resolve(dir));
		return entries.filter((e: string) => e.endsWith(".md"));
	},

	listDirectories: async (dir: string) => {
		const entries = await readdir(resolve(dir), { withFileTypes: true });
		return entries.filter((e) => e.isDirectory()).map((e) => e.name);
	},
};
