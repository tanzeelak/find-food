import { existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

let loaded = false;

export function loadEnv(): void {
  if (loaded) {
    return;
  }
  loaded = true;
  // Search upward from both the working dir and this module's location so
  // env loading is cwd-independent (e.g. `mastra dev` runs from .mastra/output).
  const seen = new Set<string>();
  for (const start of [process.cwd(), dirname(fileURLToPath(import.meta.url))]) {
    let dir = start;
    while (dir && !seen.has(dir)) {
      seen.add(dir);
      loadDotEnv(join(dir, ".env"));
      const parent = dirname(dir);
      if (parent === dir) {
        break;
      }
      dir = parent;
    }
  }
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

let cachedProjectDir: string | undefined;

// The project root is the nearest ancestor with a tsconfig.json. This is stable
// regardless of cwd (e.g. `mastra dev` runs the bundle from .mastra/output and
// from a public/ cwd), so data files always land in one predictable place.
export function projectDir(): string {
  if (cachedProjectDir) {
    return cachedProjectDir;
  }
  for (const start of [dirname(fileURLToPath(import.meta.url)), process.cwd()]) {
    let dir = start;
    while (dir) {
      if (existsSync(join(dir, "tsconfig.json"))) {
        cachedProjectDir = dir;
        return dir;
      }
      const parent = dirname(dir);
      if (parent === dir) {
        break;
      }
      dir = parent;
    }
  }
  cachedProjectDir = process.cwd();
  return cachedProjectDir;
}

// Resolve a path relative to the project root and ensure its directory exists.
export function resolveDataPath(relativePath: string): string {
  const absolutePath = resolve(projectDir(), relativePath);
  const dir = dirname(absolutePath);
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  return absolutePath;
}

export function ensureFileUrlDir(url: string): string {
  if (!url.startsWith("file:")) {
    return url;
  }
  const filePath = url.slice("file:".length);
  const dir = dirname(filePath);
  if (dir && dir !== "." && !existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
  }
  return url;
}

export type LibSQLConnection = { url: string; authToken?: string };

export function resolveLibSQLConnection(
  urlKey: string,
  tokenKey: string,
  fallbackFileUrl: string,
  inheritFrom?: { urlKey: string; tokenKey: string }
): LibSQLConnection {
  const ownUrl = process.env[urlKey];
  if (ownUrl) {
    return buildLibSQLConnection(ownUrl, process.env[tokenKey]);
  }
  if (inheritFrom) {
    const sharedUrl = process.env[inheritFrom.urlKey];
    if (sharedUrl) {
      return buildLibSQLConnection(sharedUrl, process.env[inheritFrom.tokenKey]);
    }
  }
  return { url: ensureFileUrlDir(fallbackFileUrl) };
}

function buildLibSQLConnection(url: string, token: string | undefined): LibSQLConnection {
  const connection: LibSQLConnection = { url: ensureFileUrlDir(url) };
  if (token) {
    connection.authToken = token;
  }
  return connection;
}

export function requireEnv(key: string): string {
  const value = process.env[key];
  if (value === undefined || value === "") {
    throw new Error(`Missing required environment variable: ${key}`);
  }
  return value;
}
