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

export function useSessionToken() {
  const token = useState("sessionToken", () => readStoredToken());
  const settingsOpen = useState("settingsOpen", () => false);
  /** When true, settings should highlight the JWT section (outdated/missing token). */
  const focusToken = useState("settingsFocusToken", () => false);
  const tokenError = useState<string | null>("campfireTokenError", () => null);

  onMounted(() => {
    const saved = readStoredToken();
    if (saved && saved !== token.value) {
      token.value = saved;
    }
  });

  function save(next: string) {
    const cleaned = normalizeSessionToken(next);
    if (!cleaned) return false;
    token.value = cleaned;
    writeStoredToken(cleaned);
    tokenError.value = null;
    focusToken.value = false;
    return true;
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
    token.value = "";
    writeStoredToken("");
    tokenError.value = null;
  }

  function invalidateToken(message?: string) {
    token.value = "";
    writeStoredToken("");
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
    save,
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
