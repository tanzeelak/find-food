import { randomUUID } from "node:crypto";
import { createInterface } from "node:readline";
import { runFindFoodTurn } from "./mastra/run.js";
import { normalizeStream } from "./mastra/events.js";

const DIM = "\x1b[2m";
const RESET = "\x1b[0m";

const resourceId = process.env.FIND_FOOD_USER ?? "cli-user";
let threadId = randomUUID();

const rl = createInterface({ input: process.stdin, output: process.stdout });

let closed = false;
rl.on("close", () => {
  closed = true;
});

function prompt(): void {
  if (closed) {
    return;
  }
  rl.question("\nyou > ", handleLine);
}

async function handleLine(raw: string): Promise<void> {
  const message = raw.trim();

  if (message === "") {
    prompt();
    return;
  }
  if (message === "quit" || message === "exit") {
    rl.close();
    return;
  }
  if (message === "reset") {
    threadId = randomUUID();
    process.stdout.write("\n[started a new conversation]\n");
    prompt();
    return;
  }

  try {
    const stream = await runFindFoodTurn({ message, resourceId, threadId });
    let mode: "none" | "text" | "reasoning" = "none";

    const endReasoning = () => {
      if (mode === "reasoning") {
        process.stdout.write(RESET);
      }
    };

    for await (const event of normalizeStream(stream.fullStream)) {
      if (event.kind === "tool-start") {
        endReasoning();
        process.stdout.write(`\n${DIM}  · ${event.label}…${RESET}\n`);
        mode = "none";
      } else if (event.kind === "tool-end") {
        if (event.isError) {
          process.stdout.write(`${DIM}  · ${event.label}: failed${RESET}\n`);
        }
        mode = "none";
      } else if (event.kind === "reasoning") {
        if (mode !== "reasoning") {
          process.stdout.write(`\n${DIM}thinking › `);
          mode = "reasoning";
        }
        process.stdout.write(event.text);
      } else if (event.kind === "text") {
        endReasoning();
        if (mode !== "text") {
          process.stdout.write("\nfind-food > ");
          mode = "text";
        }
        process.stdout.write(event.text);
      } else if (event.kind === "error") {
        endReasoning();
        process.stdout.write(`\n[error] ${event.message}\n`);
        mode = "none";
      }
    }
    endReasoning();
    process.stdout.write("\n");
  } catch (error) {
    process.stdout.write(`\n[error] ${error instanceof Error ? error.message : String(error)}\n`);
  }

  prompt();
}

process.stdout.write(`Find Food CLI. Thread ${threadId}.\nType "reset" for a new conversation, "quit" to exit.\n`);
prompt();
