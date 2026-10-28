const STORAGE_KEY = "campfire-event-manager.sessionToken";

export function normalizeSessionToken(raw: string) {
  return raw.trim().replace(/^Bearer\s+/i, "").replace(/^["']|["']$/g, "");
}

function readStoredToken(): string {
  if (!import.meta.client) return "";
  try {
    return normalizeSessionToken(localStorage.getItem(STORAGE_KEY) || "");
  } catch {
    return "";
  }
}

function writeStoredToken(value: string) {
  if (!import.meta.client) return;
  try {
    if (value) {
      localStorage.setItem(STORAGE_KEY, value);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    /* private mode / blocked storage */
  }
}

type CampfireTokenSync = { sync: boolean; token: string };

let accountTokenWatchStarted = false;
let appliedAccountTokenKey = "";

export function useSessionToken() {
  const token = useState("sessionToken", () => readStoredToken());
  const settingsOpen = useState("settingsOpen", () => false);
  /** When true, settings should highlight the JWT section (outdated/missing token). */
  const focusToken = useState("settingsFocusToken", () => false);
  const tokenError = useState<string | null>("campfireTokenError", () => null);
  /** Account setting from Postgres. Not stored in localStorage. */
  const tokenSync = useState("campfireTokenSync", () => false);
  const { api } = useApi();
  const { user, loaded } = useAuth();

  let pushChain: Promise<void> = Promise.resolve();

  function applyLocal(next: string) {
    token.value = next;
    writeStoredToken(next);
  }

  function pushAccountToken() {
    if (!tokenSync.value) return;
    const snapshot = token.value;
    pushChain = pushChain
      .then(async () => {
        await api<CampfireTokenSync>("/api/me/campfire-token", {
          method: "PUT",
          body: JSON.stringify({ sync: true, token: snapshot }),
        });
      })
      .catch(() => {
        tokenError.value = "Could not sync the Campfire token to your account.";
      });
  }

  if (import.meta.client && !accountTokenWatchStarted) {
    accountTokenWatchStarted = true;
    watch(
      [user, loaded],
      () => {
        if (!loaded.value || !user.value) {
          tokenSync.value = false;
          appliedAccountTokenKey = "";
          return;
        }
        const remoteToken = user.value.campfireToken || "";
        const key = `${user.value.id}:${user.value.campfireTokenSync}:${remoteToken}`;
        tokenSync.value = !!user.value.campfireTokenSync;
        if (appliedAccountTokenKey === key) return;
        appliedAccountTokenKey = key;
        if (!user.value.campfireTokenSync) return;
        if (remoteToken) {
          applyLocal(normalizeSessionToken(remoteToken));
          tokenError.value = null;
          return;
        }
        if (token.value) pushAccountToken();
      },
      { immediate: true },
    );
  }

  onMounted(() => {
    const saved = readStoredToken();
    if (saved && !token.value) token.value = saved;
  });

  function save(next: string) {
    const cleaned = normalizeSessionToken(next);
    if (!cleaned) return false;
    applyLocal(cleaned);
    tokenError.value = null;
    focusToken.value = false;
    pushAccountToken();
    return true;
  }

  async function setTokenSync(enabled: boolean) {
    const body: CampfireTokenSync = {
      sync: enabled,
      token: enabled ? token.value : "",
    };
    const saved = await api<CampfireTokenSync>("/api/me/campfire-token", {
      method: "PUT",
      body: JSON.stringify(body),
    });
    tokenSync.value = !!saved.sync;
    if (!saved.sync) return;
    if (saved.token) applyLocal(normalizeSessionToken(saved.token));
  }

  function openSettings(opts?: { focusToken?: boolean }) {
    focusToken.value = !!opts?.focusToken;
    settingsOpen.value = true;
  }

  function closeSettings() {
    settingsOpen.value = false;
    focusToken.value = false;
  }

  function clearToken() {
    applyLocal("");
    tokenError.value = null;
    pushAccountToken();
  }

  function invalidateToken(message?: string) {
    applyLocal("");
    pushAccountToken();
    tokenError.value =
      message?.trim() ||
      "Your Campfire token is invalid or expired. Paste a fresh one in Settings.";
    openSettings({ focusToken: true });
  }

  function authHeaders(extra: Record<string, string> = {}) {
    if (!token.value) return extra;
    return { ...extra, Authorization: `Bearer ${token.value}` };
  }

  return {
    token,
    settingsOpen,
    focusToken,
    tokenError,
    tokenSync,
    save,
    setTokenSync,
    openSettings,
    closeSettings,
    clearToken,
    invalidateToken,
    authHeaders,
  };
}

export function isCampfireTokenError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err || "");
  return /invalid Campfire token|missing Campfire Authorization|unauthorized|unauthenticated|not authenticated|jwt.*(expired|invalid)|token.*(expired|invalid)/i.test(
    msg,
  );
}
