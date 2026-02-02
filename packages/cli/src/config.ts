import { readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { homedir } from "node:os";

export interface Config {
	apiKey: string;
	defaultOutputFolder: string;
}

const DEFAULT_CONFIG: Config = {
	apiKey: "",
	defaultOutputFolder: "./notion",
};

function getConfigPath(): string {
	const xdgConfig = process.env["XDG_CONFIG_HOME"];
	if (xdgConfig) {
		return join(xdgConfig, "notion-sync", "config.json");
	}
	return join(homedir(), ".notion-sync.json");
}

export async function loadConfig(): Promise<Config> {
	const configPath = getConfigPath();
	let fileConfig: Partial<Config> = {};

	try {
		const raw = await readFile(configPath, "utf-8");
		fileConfig = JSON.parse(raw);
	} catch {
		// No config file, use defaults
	}

	// Env vars override file config
	const apiKey = process.env["NOTION_SYNC_API_KEY"] || fileConfig.apiKey || DEFAULT_CONFIG.apiKey;
	const defaultOutputFolder = fileConfig.defaultOutputFolder || DEFAULT_CONFIG.defaultOutputFolder;

	return { apiKey, defaultOutputFolder };
}

export async function saveConfig(key: string, value: string): Promise<void> {
	const configPath = getConfigPath();
	let existing: Record<string, unknown> = {};

	try {
		const raw = await readFile(configPath, "utf-8");
		existing = JSON.parse(raw);
	} catch {
		// Start fresh
	}

	existing[key] = value;

	await mkdir(dirname(configPath), { recursive: true });
	await writeFile(configPath, JSON.stringify(existing, null, 2) + "\n", "utf-8");
}
