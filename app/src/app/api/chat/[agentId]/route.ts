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
  // For guests, trust the client-supplied random UUID (fresh each page load = no persistence).
  if (session?.user?.id) {
    try {
      const parsed = JSON.parse(body);
      if (parsed.memory) {
        parsed.memory.resource = session.user.id;
        body = JSON.stringify(parsed);
      }
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
