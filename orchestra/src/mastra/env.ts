import { existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname } from "node:path";

let loaded = false;

export function loadEnv(): void {
  if (loaded) {
    return;
  }
  loaded = true;
  loadDotEnv(".env");
  loadDotEnv("../.env");
}

function loadDotEnv(path: string): void {
  if (!existsSync(path)) {
    return;
  }
  for (const rawLine of readFileSync(path, "utf8").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === "" || line.startsWith("#")) {
      continue;
    }
    const separator = line.indexOf("=");
    if (separator === -1) {
      continue;
    }
    const key = line.slice(0, separator).trim();
    if (key === "" || process.env[key] !== undefined) {
      continue;
    }
    process.env[key] = line
      .slice(separator + 1)
      .trim()
      .replace(/^['"]|['"]$/g, "");
  }
}

loadEnv();

export function getEnv(key: string, fallback: string): string {
  const value = process.env[key];
  return value === undefined || value === "" ? fallback : value;
}

export function ensureFileUrlDir(url: string): string {
  const filePath = url.startsWith("file:") ? url.slice("file:".length) : url;
  const dir = dirname(filePath);
  if (dir && dir !== "." && !existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  return url;
}

export function requireEnv(key: string): string {
  const value = process.env[key];
  if (value === undefined || value === "") {
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}
