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

type Profile = { id: string; email: string; display_name: string | null };

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

function ChatPanel({
  resourceId,
  targetResourceId,
  label,
}: {
  resourceId: string;
  targetResourceId?: string;
  label: string;
}) {
  const threadId = useRef(crypto.randomUUID()).current;
  const [pendingMessage, setPendingMessage] = useState<string | null>(null);

  const runtime = useChatRuntime({
    transport: new AssistantChatTransport({
      api: `/api/chat/${AGENT_ID}`,
      body: () => ({
        memory: { thread: threadId, resource: resourceId },
        ...(targetResourceId ? { targetResourceId } : {}),
      }),
    }),
  });

  return (
    <div className="flex min-w-0 flex-1 flex-col border-r border-zinc-200 last:border-r-0 dark:border-zinc-800">
      <div className="border-b border-zinc-200 px-4 py-2 text-xs font-medium text-zinc-500 dark:border-zinc-800 dark:text-zinc-400">
        {label}
      </div>
      <AssistantRuntimeProvider runtime={runtime}>
        <ChatPanelContent
          pendingMessage={pendingMessage}
          setPendingMessage={setPendingMessage}
        />
      </AssistantRuntimeProvider>
    </div>
  );
}

export function Assistant({ user }: { user: User | null }) {
  const router = useRouter();
  const guestResourceId = useRef(crypto.randomUUID()).current;
  const resourceId = user?.id ?? guestResourceId;
  const [grantedProfiles, setGrantedProfiles] = useState<Profile[]>([]);
  const [visiblePanes, setVisiblePanes] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (!user) return;
    fetch("/api/access")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setGrantedProfiles(data);
      })
      .catch(() => {});
  }, [user]);

  const togglePane = useCallback((id: string) => {
    setVisiblePanes((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

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

  const myLabel = user ? (user.user_metadata?.full_name ?? user.email ?? "Me") : "Guest";

  return (
    <div className="flex h-full w-full flex-col">
      <div className="flex items-center gap-3 border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <span className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
          {myLabel}
        </span>

        {grantedProfiles.map((profile) => {
          const active = visiblePanes.has(profile.id);
          const name = profile.display_name ?? profile.email;
          return (
            <button
              key={profile.id}
              onClick={() => togglePane(profile.id)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                active
                  ? "bg-blue-600 text-white"
                  : "bg-zinc-100 text-zinc-600 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-700"
              }`}
            >
              {active ? `− ${name}` : `+ ${name}`}
            </button>
          );
        })}

        <div className="ml-auto">
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
      </div>

      <div className="flex min-h-0 flex-1">
        <ChatPanel
          key={user?.id ?? "guest"}
          resourceId={resourceId}
          label={myLabel}
        />
        {grantedProfiles
          .filter((p) => visiblePanes.has(p.id))
          .map((profile) => (
            <ChatPanel
              key={profile.id}
              resourceId={resourceId}
              targetResourceId={profile.id}
              label={profile.display_name ?? profile.email}
            />
          ))}
      </div>
    </div>
  );
}
