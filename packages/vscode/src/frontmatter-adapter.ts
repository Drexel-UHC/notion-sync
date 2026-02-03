import type { FileSystem } from "@notion-sync/core";
import { createFrontmatterReader } from "@notion-sync/core";

export const vscodeFm = (fs: FileSystem) => createFrontmatterReader(fs);
