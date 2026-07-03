"use client";

import { createClient } from "@/lib/supabase/client";

export function LoginForm() {
  async function signIn() {
    const supabase = createClient();
    await supabase.auth.signInWithOAuth({
      provider: "google",
      options: { redirectTo: `${window.location.origin}/auth/callback` },
    });
  }

  return (
    <div className="flex flex-col gap-4 w-72">
      <div>
        <h1 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
          Sign in
        </h1>
        <p className="text-sm text-zinc-500 mt-1">
          Sign in to access your food profile.
        </p>
      </div>
      <button
        onClick={signIn}
        className="rounded-xl bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700"
      >
        Continue with Google
      </button>
    </div>
  );
}
