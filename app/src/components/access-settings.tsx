"use client";

import { useState, useEffect, useCallback } from "react";

type Profile = { id: string; email: string; display_name: string | null };

export function AccessSettings({ onClose }: { onClose: () => void }) {
  const [grantees, setGrantees] = useState<Profile[]>([]);
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchGrantees = useCallback(async () => {
    const res = await fetch("/api/access?as=owner");
    const data = await res.json();
    if (Array.isArray(data)) setGrantees(data);
  }, []);

  useEffect(() => {
    fetchGrantees();
  }, [fetchGrantees]);

  const grant = useCallback(async () => {
    if (!email.trim()) return;
    setLoading(true);
    setError(null);
    const res = await fetch("/api/access", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: email.trim() }),
    });
    const data = await res.json();
    if (!res.ok) {
      setError(data.error ?? "Failed to grant access");
    } else {
      setEmail("");
      await fetchGrantees();
    }
    setLoading(false);
  }, [email, fetchGrantees]);

  const revoke = useCallback(async (granteeId: string) => {
    const res = await fetch("/api/access", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ granteeId }),
    });
    if (res.ok) {
      setGrantees((prev) => prev.filter((g) => g.id !== granteeId));
    }
  }, []);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-zinc-900"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-5 flex items-center justify-between">
          <h2 className="text-base font-semibold text-zinc-900 dark:text-zinc-100">
            Manage access
          </h2>
          <button
            onClick={onClose}
            className="text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300"
          >
            ✕
          </button>
        </div>

        <p className="mb-4 text-sm text-zinc-500 dark:text-zinc-400">
          People you grant access to can view your dietary profile and use it to find food on your behalf.
        </p>

        <div className="mb-5 flex gap-2">
          <input
            type="email"
            placeholder="their@email.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && grant()}
            className="flex-1 rounded-xl border border-zinc-300 bg-transparent px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 dark:border-zinc-700 dark:text-zinc-100"
          />
          <button
            onClick={grant}
            disabled={loading || !email.trim()}
            className="rounded-xl bg-blue-600 px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            Grant
          </button>
        </div>

        {error && (
          <p className="mb-3 text-sm text-red-500">{error}</p>
        )}

        <div className="space-y-2">
          {grantees.length === 0 ? (
            <p className="text-sm text-zinc-400">No one has access yet.</p>
          ) : (
            grantees.map((g) => (
              <div
                key={g.id}
                className="flex items-center justify-between rounded-xl border border-zinc-200 px-3 py-2 dark:border-zinc-700"
              >
                <div>
                  <p className="text-sm font-medium text-zinc-800 dark:text-zinc-200">
                    {g.display_name ?? g.email}
                  </p>
                  {g.display_name && (
                    <p className="text-xs text-zinc-400">{g.email}</p>
                  )}
                </div>
                <button
                  onClick={() => revoke(g.id)}
                  className="text-xs text-red-500 hover:text-red-700"
                >
                  Revoke
                </button>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
