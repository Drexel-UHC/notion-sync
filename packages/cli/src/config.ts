import { readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { homedir } from "node:os";
import { Entry } from "@napi-rs/keyring";

export interface Config {
	apiKey: string;
	defaultOutputFolder: string;
}

const DEFAULT_CONFIG: Config = {
	apiKey: "",
	defaultOutputFolder: "./notion",
};

const KEYRING_SERVICE = "notion-sync";
const KEYRING_ACCOUNT = "api-key";

function getConfigPath(): string {
	const xdgConfig = process.env["XDG_CONFIG_HOME"];
	if (xdgConfig) {
		return join(xdgConfig, "notion-sync", "config.json");
	}
	return join(homedir(), ".notion-sync.json");
}

function tryKeychainGet(): string | null {
	try {
		const entry = new Entry(KEYRING_SERVICE, KEYRING_ACCOUNT);
		return entry.getPassword();
	} catch {
		return null;
	}
}

function tryKeychainSet(value: string): boolean {
	try {
		const entry = new Entry(KEYRING_SERVICE, KEYRING_ACCOUNT);
		entry.setPassword(value);
		return true;
	} catch {
		return false;
	}
}

function tryKeychainDelete(): void {
	try {
		const entry = new Entry(KEYRING_SERVICE, KEYRING_ACCOUNT);
		entry.deletePassword();
	} catch {
		// Ignore — key may not exist or keychain unavailable
	}
}

/**
 * Migrate API key from config file to OS keychain.
 * Idempotent: skips if keychain already has a key or config file has none.
 * Silent on failure (keychain unavailable).
 */
export async function migrateApiKeyToKeychain(): Promise<void> {
	// If keychain already has a key, nothing to do
	if (tryKeychainGet()) return;

	const configPath = getConfigPath();
	let fileConfig: Record<string, unknown> = {};

	try {
		const raw = await readFile(configPath, "utf-8");
		fileConfig = JSON.parse(raw);
	} catch {
		return; // No config file
	}

	const apiKey = fileConfig["apiKey"];
	if (typeof apiKey !== "string" || !apiKey) return;

	if (tryKeychainSet(apiKey)) {
		// Remove apiKey from config file
		delete fileConfig["apiKey"];
		await mkdir(dirname(configPath), { recursive: true });
		await writeFile(configPath, JSON.stringify(fileConfig, null, 2) + "\n", "utf-8");
		console.log("Migrated API key from config file to OS keychain.");
	}
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

	// Priority: env var > keychain > config file
	const envKey = process.env["NOTION_SYNC_API_KEY"];
	let apiKey = "";

	if (envKey) {
		apiKey = envKey;
	} else {
		const keychainKey = tryKeychainGet();
		if (keychainKey) {
			apiKey = keychainKey;
		} else if (fileConfig.apiKey) {
			console.warn(
				"Warning: Reading API key from plaintext config file. " +
				"Run `notion-sync config set apiKey <key>` to store it in the OS keychain."
			);
			apiKey = fileConfig.apiKey;
		}
	}

	const defaultOutputFolder = fileConfig.defaultOutputFolder || DEFAULT_CONFIG.defaultOutputFolder;

	return { apiKey, defaultOutputFolder };
}

export async function saveConfig(key: string, value: string): Promise<void> {
	if (key === "apiKey") {
		if (tryKeychainSet(value)) {
			// Also remove from config file if present
			const configPath = getConfigPath();
			try {
				const raw = await readFile(configPath, "utf-8");
				const existing = JSON.parse(raw) as Record<string, unknown>;
				if ("apiKey" in existing) {
					delete existing["apiKey"];
					await writeFile(configPath, JSON.stringify(existing, null, 2) + "\n", "utf-8");
				}
			} catch {
				// No config file to clean up
			}
			return;
		}
		console.warn(
			"Warning: OS keychain unavailable. Storing API key in plaintext config file."
		);
	}

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
