import { parse } from "yaml";
import type { FileSystem, FrontmatterReader } from "@notion-sync/core";

export function vscodeFm(fs: FileSystem): FrontmatterReader {
	return {
		readFrontmatter: async (filePath) => {
			try {
				const content = await fs.readFile(filePath);
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
}
