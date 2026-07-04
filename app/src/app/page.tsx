import { Assistant } from "@/components/assistant";
import { createClient } from "@/lib/supabase/server";

export default async function Home() {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  return (
    <div className="flex flex-1">
      <Assistant user={user} />
    </div>
  );
}
