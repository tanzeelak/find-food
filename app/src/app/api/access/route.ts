import { type NextRequest, NextResponse } from "next/server";
import { createClient } from "@/lib/supabase/server";

export async function GET() {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const { data: rows, error: accessError } = await supabase
    .from("profile_access")
    .select("owner_id")
    .eq("grantee_id", user.id);

  if (accessError) return NextResponse.json({ error: accessError.message }, { status: 500 });

  const ownerIds = (rows ?? []).map((r: { owner_id: string }) => r.owner_id);
  if (ownerIds.length === 0) return NextResponse.json([]);

  const { data: profiles, error: profileError } = await supabase
    .from("profiles")
    .select("id, email, display_name")
    .in("id", ownerIds);

  if (profileError) return NextResponse.json({ error: profileError.message }, { status: 500 });

  return NextResponse.json(profiles ?? []);
}

export async function POST(request: NextRequest) {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const { email } = await request.json();
  if (!email) return NextResponse.json({ error: "email required" }, { status: 400 });

  const { data: grantee, error: lookupError } = await supabase
    .from("profiles")
    .select("id")
    .eq("email", email)
    .single();

  if (lookupError || !grantee) {
    return NextResponse.json({ error: "User not found" }, { status: 404 });
  }

  const { error } = await supabase
    .from("profile_access")
    .insert({ owner_id: user.id, grantee_id: grantee.id });

  if (error) return NextResponse.json({ error: error.message }, { status: 500 });

  return NextResponse.json({ ok: true });
}

export async function DELETE(request: NextRequest) {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  if (!user) return NextResponse.json({ error: "Unauthorized" }, { status: 401 });

  const { granteeId } = await request.json();
  if (!granteeId) return NextResponse.json({ error: "granteeId required" }, { status: 400 });

  const { error } = await supabase
    .from("profile_access")
    .delete()
    .eq("owner_id", user.id)
    .eq("grantee_id", granteeId);

  if (error) return NextResponse.json({ error: error.message }, { status: 500 });

  return NextResponse.json({ ok: true });
}
