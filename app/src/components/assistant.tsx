"use client";

import { useRef, useCallback, useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  AssistantRuntimeProvider,
  ThreadPrimitive,
  ComposerPrimitive,
  MessagePrimitive,
  useComposer,
  useComposerRuntime,
  useThread,
} from "@assistant-ui/react";
import {
  useChatRuntime,
  AssistantChatTransport,
} from "@assistant-ui/react-ai-sdk";
import type { User } from "@supabase/supabase-js";
import { createClient } from "@/lib/supabase/client";

const AGENT_ID = "findFood";

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

function ChatPanelContent({
  pendingMessage,
  setPendingMessage,
}: {
  pendingMessage: string | null;
  setPendingMessage: (msg: string | null) => void;
}) {
  const composerRuntime = useComposerRuntime();
  const isRunning = useThread((t) => t.isRunning);
  const isEmpty = useComposer((c) => c.isEmpty);
  const composerText = useComposer((c) => c.text);

  // When the agent finishes, flush the queued message
  useEffect(() => {
    if (!isRunning && pendingMessage !== null) {
      composerRuntime.setText(pendingMessage);
      composerRuntime.send();
      setPendingMessage(null);
    }
  }, [isRunning, pendingMessage, composerRuntime, setPendingMessage]);

  const handleSend = useCallback(() => {
    if (isRunning) {
      if (!composerText.trim()) return;
      setPendingMessage(composerText);
      composerRuntime.setText("");
    } else {
      composerRuntime.send();
    }
  }, [composerRuntime, composerText, isRunning, setPendingMessage]);

  return (
    <ThreadPrimitive.Root className="flex h-full w-full flex-col">
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

      <div className="border-t border-zinc-200 dark:border-zinc-800">
        {pendingMessage && (
          <p className="px-4 pt-2 text-xs text-zinc-400">
            Queued: &ldquo;{pendingMessage}&rdquo;
          </p>
        )}
        <ComposerPrimitive.Root className="flex items-end gap-2 p-4">
          <ComposerPrimitive.Input
            rows={1}
            autoFocus
            placeholder="gluten-free fish tacos in the Mission, SF…"
            className="flex-1 resize-none rounded-xl border border-zinc-300 bg-transparent px-3 py-2 outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700"
          />
          <button
            onClick={handleSend}
            disabled={isEmpty && !isRunning}
            className="rounded-xl bg-blue-600 px-4 py-2 font-medium text-white disabled:opacity-50"
          >
            Send
          </button>
        </ComposerPrimitive.Root>
      </div>
    </ThreadPrimitive.Root>
  );
}

function ChatPanel({ resourceId }: { resourceId: string }) {
  const threadId = useRef(crypto.randomUUID()).current;
  const [pendingMessage, setPendingMessage] = useState<string | null>(null);

  const runtime = useChatRuntime({
    transport: new AssistantChatTransport({
      api: `/api/chat/${AGENT_ID}`,
      body: () => ({ memory: { thread: threadId, resource: resourceId } }),
    }),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ChatPanelContent
        pendingMessage={pendingMessage}
        setPendingMessage={setPendingMessage}
      />
    </AssistantRuntimeProvider>
  );
}

export function Assistant({ user }: { user: User | null }) {
  const router = useRouter();
  const guestResourceId = useRef(crypto.randomUUID()).current;
  const resourceId = user?.id ?? guestResourceId;

  const signIn = useCallback(async () => {
    const supabase = createClient();
    await supabase.auth.signInWithOAuth({
      provider: "google",
      options: { redirectTo: `${window.location.origin}/auth/callback` },
    });
  }, []);

  const signOut = useCallback(async () => {
    const supabase = createClient();
    await supabase.auth.signOut();
    router.refresh();
  }, [router]);

  return (
    <div className="flex h-full w-full max-w-3xl flex-col">
      <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <span className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
          {user ? (user.user_metadata?.full_name ?? user.email) : "Guest"}
        </span>
        {user ? (
          <button
            onClick={signOut}
            className="text-sm text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
          >
            Sign out
          </button>
        ) : (
          <button
            onClick={signIn}
            className="text-sm text-blue-600 hover:underline"
          >
            Sign in with Google
          </button>
        )}
      </div>
      <ChatPanel key={user?.id ?? "guest"} resourceId={resourceId} />
    </div>
  );
}
