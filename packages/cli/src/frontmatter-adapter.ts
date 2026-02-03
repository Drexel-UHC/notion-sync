import { createFrontmatterReader } from "@notion-sync/core";
import { nodeFs } from "./fs-adapter.js";

export const nodeFm = createFrontmatterReader(nodeFs);
