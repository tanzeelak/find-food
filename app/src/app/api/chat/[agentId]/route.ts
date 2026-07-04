import { type NextRequest } from "next/server";
import { createClient } from "@/lib/supabase/server";

const MASTRA_URL = process.env.MASTRA_URL ?? "http://localhost:4111";

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ agentId: string }> }
) {
  const { agentId } = await params;
  const supabase = await createClient();
  const {
    data: { session },
  } = await supabase.auth.getSession();

  let body = await request.text();

  const headers: HeadersInit = { "Content-Type": "application/json" };
  if (session?.access_token) {
    headers["Authorization"] = `Bearer ${session.access_token}`;
  }

  // /chat/ is a public Mastra route so mapUserToResourceId never runs.
  // For authenticated users, override resource with the verified server-side user ID.
  // If targetResourceId is supplied, verify the grant and use the owner's memory instead.
  // For guests, trust the client-supplied random UUID (fresh each page load = no persistence).
  if (session?.user?.id) {
    try {
      const parsed = JSON.parse(body);
      const targetResourceId = parsed.targetResourceId as string | undefined;

      if (targetResourceId && targetResourceId !== session.user.id) {
        const { data, error } = await supabase
          .from("profile_access")
          .select("owner_id")
          .eq("owner_id", targetResourceId)
          .eq("grantee_id", session.user.id)
          .single();

        if (error || !data) {
          return new Response(JSON.stringify({ error: "Forbidden" }), {
            status: 403,
            headers: { "Content-Type": "application/json" },
          });
        }

        // Keep memory scoped to the grantee (read-only access to owner's profile).
        // Fetch the owner's working memory and inject it as context instead.
        if (parsed.memory) parsed.memory.resource = session.user.id;

        const wmRes = await fetch(`${MASTRA_URL}/working-memory/${targetResourceId}`);
        if (wmRes.ok) {
          const { workingMemory } = await wmRes.json() as { workingMemory: string | null };
          if (workingMemory) {
            parsed.messages = [
              {
                role: "user",
                content: `[You are finding food on behalf of someone else. Use only their dietary profile below — do not update memory. Their profile:\n${workingMemory}]`,
              },
              ...(parsed.messages ?? []),
            ];
          }
        }
      } else if (parsed.memory) {
        parsed.memory.resource = session.user.id;
      }

      delete parsed.targetResourceId;
      body = JSON.stringify(parsed);
    } catch {}
  }

  const upstream = await fetch(`${MASTRA_URL}/chat/${agentId}`, {
    method: "POST",
    headers,
    body,
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "text/plain",
      "Cache-Control": "no-cache",
    },
  });
}
