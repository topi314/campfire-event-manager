const PREFS_KEY = "campfire-event-manager.preferences";

type StoredPrefs = {
  /** Legacy browser-wide choice, adopted by the next account that has none. */
  timeZone?: string;
  byUser?: Record<string, string>;
  /** Discord user id → preferred Campfire club id for create/edit auto-select. */
  preferredClubByUser?: Record<string, string>;
};

function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

function normalizeStringMap(raw: unknown): Record<string, string> {
  const out: Record<string, string> = {};
  if (!raw || typeof raw !== "object") return out;
  for (const [id, value] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof value === "string" && value.trim()) out[id] = value.trim();
  }
  return out;
}

function normalizeStored(raw: unknown): StoredPrefs {
  if (!raw || typeof raw !== "object") return {};
  const parsed = raw as StoredPrefs;
  const byUser = normalizeStringMap(parsed.byUser);
  const preferredClubByUser = normalizeStringMap(parsed.preferredClubByUser);
  const legacy = typeof parsed.timeZone === "string" ? parsed.timeZone.trim() : "";
  return {
    timeZone: legacy || undefined,
    byUser,
    preferredClubByUser,
  };
}

function readStored(): StoredPrefs {
  if (!import.meta.client) return {};
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (!raw) return {};
    return normalizeStored(JSON.parse(raw));
  } catch {
    return {};
  }
}

function writeStored(prefs: StoredPrefs) {
  if (!import.meta.client) return;
  try {
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
  } catch {
    /* private mode */
  }
}

/** Full IANA list when available; otherwise browser zone (+ UTC). */
export function listTimeZones(): string[] {
  try {
    const supported = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] })
      .supportedValuesOf;
    if (typeof supported === "function") {
      return supported.call(Intl, "timeZone");
    }
  } catch {
    /* ignore */
  }
  const fallback = new Set(["UTC", browserTimeZone()]);
  return [...fallback].sort((a, b) => a.localeCompare(b));
}

export function usePreferences() {
  const stored = useState<StoredPrefs>("userPreferences", () => readStored());
  const timezonePromptOpen = useState("timezonePromptOpen", () => false);
  const { user } = useAuth();
  const { api } = useApi();

  onMounted(() => {
    stored.value = readStored();
  });

  watch(user, (current) => {
    if (!current) timezonePromptOpen.value = false;
  });

  const timeZone = computed(() => {
    const id = user.value?.id;
    if (id) {
      const saved = stored.value.byUser?.[id];
      if (saved) return saved;
      const fromAccount = user.value?.timeZone?.trim();
      if (fromAccount) return fromAccount;
    }
    return browserTimeZone();
  });

  const hasTimeZonePreference = computed(() => {
    const id = user.value?.id;
    if (!id) return false;
    return !!(stored.value.byUser?.[id] || user.value?.timeZone?.trim());
  });

  function snapshot(partial: Partial<StoredPrefs>): StoredPrefs {
    return {
      timeZone: "timeZone" in partial ? partial.timeZone : stored.value.timeZone,
      byUser: partial.byUser ?? { ...(stored.value.byUser || {}) },
      preferredClubByUser:
        partial.preferredClubByUser ?? { ...(stored.value.preferredClubByUser || {}) },
    };
  }

  function remember(userId: string, tz: string) {
    const next = snapshot({
      byUser: { ...(stored.value.byUser || {}), [userId]: tz },
    });
    stored.value = next;
    writeStored(next);
  }

  function clearLegacy() {
    if (!stored.value.timeZone) return;
    const next = snapshot({ timeZone: undefined });
    stored.value = next;
    writeStored(next);
  }

  const preferredClubId = computed(() => {
    const id = user.value?.id;
    if (!id) return "";
    return stored.value.preferredClubByUser?.[id] || "";
  });

  function setPreferredClubId(clubId: string) {
    const id = user.value?.id;
    if (!id) return;
    const nextMap = { ...(stored.value.preferredClubByUser || {}) };
    const trimmed = clubId.trim();
    if (trimmed) nextMap[id] = trimmed;
    else delete nextMap[id];
    const next = snapshot({ preferredClubByUser: nextMap });
    stored.value = next;
    writeStored(next);
  }

  const persistInflight = useState<Promise<void> | null>("timezonePersistInflight", () => null);

  async function persistTimeZone(tz: string) {
    const current = user.value;
    if (!current?.id) return;
    if ((current.timeZone || "").trim() === tz) return;
    if (persistInflight.value) return persistInflight.value;

    const run = (async () => {
      try {
        const latest = user.value;
        if (!latest?.id || (latest.timeZone || "").trim() === tz) return;
        await api("/api/me", {
          method: "PATCH",
          body: JSON.stringify({ timeZone: tz }),
        });
        if (user.value?.id === latest.id) {
          user.value = { ...user.value, timeZone: tz };
        }
      } finally {
        persistInflight.value = null;
      }
    })();
    persistInflight.value = run;
    return run;
  }

  function setTimeZone(next: string) {
    const tz = next.trim() || browserTimeZone();
    const id = user.value?.id;
    if (id) {
      remember(id, tz);
      clearLegacy();
    }
    timezonePromptOpen.value = false;
    void persistTimeZone(tz).catch(() => {
      /* Retried the next time this account loads. */
    });
  }

  /**
   * Prompt only when this account has never saved a timezone.
   * A choice already stored for the account (server or this browser) is reused.
   */
  function maybePromptTimezone() {
    if (!import.meta.client) return;
    const current = user.value;
    if (!current?.id) return;

    const accountTz = (current.timeZone || "").trim();
    if (accountTz) {
      remember(current.id, accountTz);
      timezonePromptOpen.value = false;
      return;
    }

    const local = stored.value.byUser?.[current.id];
    if (local) {
      timezonePromptOpen.value = false;
      void persistTimeZone(local).catch(() => {});
      return;
    }

    const legacy = stored.value.timeZone?.trim();
    if (legacy) {
      remember(current.id, legacy);
      clearLegacy();
      timezonePromptOpen.value = false;
      void persistTimeZone(legacy).catch(() => {});
      return;
    }

    timezonePromptOpen.value = true;
  }

  function closeTimezonePrompt() {
    if (!hasTimeZonePreference.value) {
      setTimeZone(browserTimeZone());
    } else {
      timezonePromptOpen.value = false;
    }
  }

  return {
    prefs: stored,
    timeZone,
    hasTimeZonePreference,
    timezonePromptOpen,
    setTimeZone,
    preferredClubId,
    setPreferredClubId,
    maybePromptTimezone,
    closeTimezonePrompt,
    browserTimeZone,
    listTimeZones,
  };
}
