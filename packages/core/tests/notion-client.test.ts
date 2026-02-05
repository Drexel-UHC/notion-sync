import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { normalizeNotionId, notionRequest } from "../src/notion-client.js";
import { Client } from "@notionhq/client";

describe("normalizeNotionId", () => {
	it("formats a 32-char hex string as UUID", () => {
		const result = normalizeNotionId("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6");
		expect(result).toBe("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
	});

	it("accepts a UUID with dashes", () => {
		const result = normalizeNotionId("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
		expect(result).toBe("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
	});

	it("extracts ID from a Notion page URL", () => {
		const url = "https://www.notion.so/workspace/My-Page-a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6";
		const result = normalizeNotionId(url);
		expect(result).toBe("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
	});

	it("extracts ID from a Notion database URL", () => {
		const url = "https://www.notion.so/a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6?v=abc";
		const result = normalizeNotionId(url);
		expect(result).toBe("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
	});

	it("handles uppercase hex", () => {
		const result = normalizeNotionId("A1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6");
		expect(result).toBe("A1B2C3D4-E5F6-A7B8-C9D0-E1F2A3B4C5D6");
	});

	it("trims whitespace", () => {
		const result = normalizeNotionId("  a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6  ");
		expect(result).toBe("a1b2c3d4-e5f6-a7b8-c9d0-e1f2a3b4c5d6");
	});

	it("throws on invalid ID (too short)", () => {
		expect(() => normalizeNotionId("abc123")).toThrow("Invalid Notion ID");
	});

	it("throws on invalid ID (non-hex)", () => {
		expect(() => normalizeNotionId("g1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6")).toThrow("Invalid Notion ID");
	});

	it("throws on URL without 32-char hex", () => {
		expect(() => normalizeNotionId("https://www.notion.so/short-id")).toThrow("Could not extract Notion ID from URL");
	});
});

describe("notionRequest", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it("returns result on success", async () => {
		const result = await notionRequest(() => Promise.resolve("ok"));
		expect(result).toBe("ok");
	});

	it("throws non-retryable errors immediately", async () => {
		const err = Object.assign(new Error("Not Found"), { status: 404 });
		// Attach a catch handler before advancing timers to prevent unhandled rejection
		const promise = notionRequest(() => Promise.reject(err)).catch((e) => e);
		await vi.advanceTimersByTimeAsync(1000);
		const result = await promise;
		expect(result).toBeInstanceOf(Error);
		expect((result as Error).message).toBe("Not Found");
	});

	it("retries on 429 errors", async () => {
		let calls = 0;
		const fn = () => {
			calls++;
			if (calls === 1) {
				return Promise.reject(Object.assign(new Error("Rate limited"), { status: 429 }));
			}
			return Promise.resolve("success");
		};

		// Run notionRequest in background, advance timers to resolve sleeps
		const promise = notionRequest(fn);
		// Advance past throttle + retry delay
		await vi.advanceTimersByTimeAsync(5000);
		const result = await promise;

		expect(result).toBe("success");
		expect(calls).toBe(2);
	});

	it("retries on 500 errors", async () => {
		let calls = 0;
		const fn = () => {
			calls++;
			if (calls === 1) {
				return Promise.reject(Object.assign(new Error("Server error"), { status: 500 }));
			}
			return Promise.resolve("recovered");
		};

		const promise = notionRequest(fn);
		await vi.advanceTimersByTimeAsync(5000);
		const result = await promise;

		expect(result).toBe("recovered");
		expect(calls).toBe(2);
	});
});

