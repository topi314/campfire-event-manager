import type { DiscordUser } from "~/types";

/** Module-level so concurrent callers share one in-flight request (useState is a poor fit for Promises). */
let authRefreshInflight: Promise<void> | null = null;

export function useApi() {
  const config = useRuntimeConfig();
  const base = (config.public.apiBase as string) || "";

  async function api<T>(path: string, opts: RequestInit = {}): Promise<T> {
    const headers = new Headers(opts.headers || {});
    if (!headers.has("Accept")) headers.set("Accept", "application/json");
    if (opts.body && !headers.has("Content-Type") && !(opts.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }
    const res = await fetch(`${base}${path}`, {
      ...opts,
      headers,
      credentials: "include",
    });
    if (!res.ok) {
      let msg = res.statusText;
      try {
        const body = await res.json();
        if (body?.error) msg = body.error;
      } catch {
        /* ignore */
      }
      if (res.status === 401 && !msg) msg = "unauthorized";
      throw new Error(msg || "request failed");
    }
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  return { api, base };
}

export function useAuth() {
  const user = useState<DiscordUser | null>("discordUser", () => null);
  const loaded = useState("authLoaded", () => false);
  const { api } = useApi();

  function refresh(opts?: { force?: boolean }): Promise<void> {
    if (!opts?.force && loaded.value) return Promise.resolve();
    if (authRefreshInflight) return authRefreshInflight;

    authRefreshInflight = (async () => {
      try {
        user.value = await api<DiscordUser>("/api/me");
      } catch {
        user.value = null;
      } finally {
        loaded.value = true;
        authRefreshInflight = null;
      }
    })();
    return authRefreshInflight;
  }

  /** Wait for AppShell (or another caller) to finish the initial /api/me load. */
  async function ensureAuth(): Promise<boolean> {
    if (!loaded.value) await refresh();
    return !!user.value;
  }

  async function logout() {
    await api("/logout", { method: "POST" });
    user.value = null;
    loaded.value = true;
  }

  return { user, loaded, refresh, ensureAuth, logout };
}
