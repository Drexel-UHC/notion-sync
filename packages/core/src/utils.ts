export function sanitizeFileName(name: string): string {
	return name.replace(/[\\/:*?"<>|]/g, "-").trim() || "Untitled";
}

export function joinPath(...parts: string[]): string {
	return parts.join("/");
}
