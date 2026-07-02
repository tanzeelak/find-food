"use client";

import { useState, useRef } from "react";
import {
  AssistantRuntimeProvider,
  ThreadPrimitive,
  ComposerPrimitive,
  MessagePrimitive,
} from "@assistant-ui/react";
import {
  useChatRuntime,
  AssistantChatTransport,
} from "@assistant-ui/react-ai-sdk";

const MASTRA_URL =
  process.env.NEXT_PUBLIC_MASTRA_URL ?? "http://localhost:4111";
const AGENT_ID = "findFood";

const PROFILES = [
  { id: "tanzeela", label: "Tanzeela" },
  { id: "guest", label: "Guest" },
] as const;

type ProfileId = (typeof PROFILES)[number]["id"];

function UserMessage() {
  return (
    <MessagePrimitive.Root className="flex justify-end">
      <div className="max-w-[80%] whitespace-pre-wrap rounded-2xl bg-blue-600 px-4 py-2 text-white">
        <MessagePrimitive.Parts />
      </div>
    </MessagePrimitive.Root>
  );
}

function AssistantMessage() {
  return (
    <MessagePrimitive.Root className="flex justify-start">
      <div className="max-w-[80%] whitespace-pre-wrap rounded-2xl bg-zinc-100 px-4 py-2 text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100">
        <MessagePrimitive.Parts />
      </div>
    </MessagePrimitive.Root>
  );
}

function ChatPanel({ resourceId }: { resourceId: ProfileId }) {
  const resourceIdRef = useRef(resourceId);
  resourceIdRef.current = resourceId;

  // Stable thread ID for this session — resets when profile changes (via key={resourceId})
  const threadId = useRef(crypto.randomUUID()).current;

  const runtime = useChatRuntime({
    transport: new AssistantChatTransport({
      api: `${MASTRA_URL}/chat/${AGENT_ID}`,
      body: () => ({
        memory: {
          thread: threadId,
          resource: resourceIdRef.current,
        },
      }),
    }),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ThreadPrimitive.Root className="flex h-full w-full max-w-3xl flex-col">
        <ThreadPrimitive.Viewport className="flex flex-1 flex-col gap-4 overflow-y-auto p-4">
          <ThreadPrimitive.Empty>
            <div className="m-auto text-center text-zinc-500">
              <p className="text-lg font-medium">Find Food</p>
              <p className="text-sm">
                Ask for dietary-friendly menu items near a location.
              </p>
            </div>
          </ThreadPrimitive.Empty>
          <ThreadPrimitive.Messages
            components={{ UserMessage, AssistantMessage }}
          />
        </ThreadPrimitive.Viewport>

        <ComposerPrimitive.Root className="flex items-end gap-2 border-t border-zinc-200 p-4 dark:border-zinc-800">
          <ComposerPrimitive.Input
            rows={1}
            autoFocus
            placeholder="gluten-free fish tacos in the Mission, SF…"
            className="flex-1 resize-none rounded-xl border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
          />
          <ComposerPrimitive.Send className="rounded-xl bg-blue-600 px-4 py-2 font-medium text-white disabled:opacity-50">
            Send
          </ComposerPrimitive.Send>
        </ComposerPrimitive.Root>
      </ThreadPrimitive.Root>
    </AssistantRuntimeProvider>
  );
}

export function Assistant() {
  const [resourceId, setResourceId] = useState<ProfileId>("tanzeela");

  return (
    <div className="flex h-full w-full max-w-3xl flex-col">
      <div className="flex items-center gap-2 border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <span className="text-sm text-zinc-500 dark:text-zinc-400">Profile:</span>
        {PROFILES.map((p) => (
          <button
            key={p.id}
            onClick={() => setResourceId(p.id)}
            className={`rounded-full px-3 py-1 text-sm font-medium transition-colors ${
              resourceId === p.id
                ? "bg-blue-600 text-white"
                : "bg-zinc-100 text-zinc-700 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-300 dark:hover:bg-zinc-700"
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>
      <ChatPanel key={resourceId} resourceId={resourceId} />
    </div>
  );
}
