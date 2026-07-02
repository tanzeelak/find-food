import { randomUUID } from "node:crypto";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { getEnv } from "./mastra/env.js";
import { runFindFoodTurn } from "./mastra/run.js";
import { normalizeStream } from "./mastra/events.js";

const host = getEnv("HOST", "127.0.0.1");
const port = Number.parseInt(getEnv("PORT", "3000"), 10);

function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk) => chunks.push(Buffer.from(chunk)));
    req.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    req.on("error", reject);
  });
}

function sse(res: ServerResponse, event: Record<string, unknown>): void {
  res.write(`data: ${JSON.stringify(event)}\n\n`);
}

async function handleChat(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const raw = await readBody(req);
  let body: { message?: string; threadId?: string; resourceId?: string } = {};
  try {
    body = raw ? JSON.parse(raw) : {};
  } catch {
    res.writeHead(400, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "invalid_json" }));
    return;
  }

  const message = (body.message ?? "").trim();
  if (message === "") {
    res.writeHead(400, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ error: "message_required" }));
    return;
  }

  const resourceId = body.resourceId?.trim() || "web-user";
  const threadId = body.threadId?.trim() || randomUUID();

  res.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    Connection: "keep-alive"
  });
  sse(res, { type: "thread", threadId, resourceId });

  try {
    const stream = await runFindFoodTurn({ message, resourceId, threadId });
    for await (const event of normalizeStream(stream.fullStream)) {
      if (event.kind === "text") {
        sse(res, { type: "delta", text: event.text });
      } else if (event.kind === "reasoning") {
        sse(res, { type: "reasoning", text: event.text });
      } else if (event.kind === "tool-start") {
        sse(res, { type: "tool", status: "start", label: event.label });
      } else if (event.kind === "tool-end") {
        sse(res, { type: "tool", status: "end", label: event.label, isError: event.isError });
      } else if (event.kind === "error") {
        sse(res, { type: "error", message: event.message });
      }
    }
    sse(res, { type: "done" });
  } catch (error) {
    sse(res, { type: "error", message: error instanceof Error ? error.message : String(error) });
  }
  res.end();
}

const server = createServer((req, res) => {
  if (req.method === "GET" && req.url === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }
  if (req.method === "POST" && req.url === "/api/chat") {
    handleChat(req, res).catch((error) => {
      if (!res.headersSent) {
        res.writeHead(500, { "Content-Type": "application/json" });
      }
      res.end(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
    });
    return;
  }
  res.writeHead(404, { "Content-Type": "application/json" });
  res.end(JSON.stringify({ error: "not_found" }));
});

server.listen(port, host, () => {
  process.stdout.write(`Find Food web server on http://${host}:${port}\n  POST /api/chat  (SSE stream)\n  GET  /health\n`);
});
