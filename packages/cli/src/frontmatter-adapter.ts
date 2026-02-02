import { parse } from "yaml";
import type { FrontmatterReader } from "@notion-sync/core";
import { nodeFs } from "./fs-adapter.js";

export const nodeFm: FrontmatterReader = {
	readFrontmatter: async (filePath) => {
		try {
			const content = await nodeFs.readFile(filePath);
			if (!content.startsWith("---\n")) return null;
			const endIdx = content.indexOf("\n---", 3);
			if (endIdx === -1) return null;
			const yamlBlock = content.slice(4, endIdx);
			return parse(yamlBlock) as Record<string, unknown>;
		} catch {
			return null;
		}
	},
};
