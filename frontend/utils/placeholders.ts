/** Match {{key}} tokens. Keys: letters, digits, underscore, hyphen. */
const PLACEHOLDER_RE = /\{\{\s*([a-zA-Z][a-zA-Z0-9_-]*)\s*\}\}/g;

export type PlaceholderDef = {
  key: string;
  label?: string;
  default?: string;
};

/** Built-in tokens filled automatically when creating a meetup from a template. */
export type BuiltinPlaceholder = {
  key: string;
  label: string;
  description: string;
  example: string;
};

export type BuiltinPlaceholderContext = {
  clubName?: string;
  liveEventName?: string;
  category?: string;
  /** Meetup calendar day (from live event or today). */
  date?: { year: number; month: number; day: number };
  /** Meetup or template clock times (HH:mm). */
  startTime?: string;
  endTime?: string;
  timeZone?: string;
  /** Meetup title / name field. */
  title?: string;
  /** Free-text address field. */
  address?: string;
  /** Map pin coordinates. */
  latitude?: number | null;
  longitude?: number | null;
};

export const BUILTIN_PLACEHOLDERS: BuiltinPlaceholder[] = [
  {
    key: "club",
    label: "Club",
    description: "Name of the Campfire club you selected.",
    example: "PoGo Offenbach",
  },
  {
    key: "liveEvent",
    label: "Live event",
    description: "Campfire live event title, or empty if none is linked.",
    example: "Community Day",
  },
  {
    key: "category",
    label: "Category",
    description: "Inferred live-event category (e.g. Community Day, Raid Hour).",
    example: "Community Day",
  },
  {
    key: "title",
    label: "Title",
    description: "Meetup title from the create form (useful in the description).",
    example: "Community Day — Downtown",
  },
  {
    key: "address",
    label: "Address",
    description: "Address field from the create form.",
    example: "Main Station, Platform 3",
  },
  {
    key: "lat",
    label: "Latitude",
    description: "Map pin latitude (before jitter).",
    example: "50.110922",
  },
  {
    key: "lng",
    label: "Longitude",
    description: "Map pin longitude (before jitter).",
    example: "8.682127",
  },
  {
    key: "date",
    label: "Date",
    description: "Meetup calendar day as a readable date (from the live event, or today).",
    example: "Sep 30, 2026",
  },
  {
    key: "dateShort",
    label: "Date (short)",
    description: "Meetup calendar day as YYYY-MM-DD.",
    example: "2026-09-30",
  },
  {
    key: "weekday",
    label: "Weekday",
    description: "Weekday name for the meetup day.",
    example: "Wednesday",
  },
  {
    key: "startTime",
    label: "Start time",
    description: "Meetup start clock time (HH:mm).",
    example: "14:00",
  },
  {
    key: "endTime",
    label: "End time",
    description: "Meetup end clock time (HH:mm).",
    example: "17:00",
  },
  {
    key: "timezone",
    label: "Timezone",
    description: "Your Settings timezone used for wall-clock times.",
    example: "Europe/Berlin",
  },
];

/** Case-insensitive identity for placeholder keys. */
export function normalizePlaceholderKey(key: string): string {
  return key.trim().toLowerCase();
}

const builtinByNorm = new Map(
  BUILTIN_PLACEHOLDERS.map((b) => [normalizePlaceholderKey(b.key), b.key]),
);

/**
 * Prefer the documented built-in spelling; otherwise keep the given casing.
 * Matching itself is always case-insensitive.
 */
export function canonicalPlaceholderKey(key: string): string {
  const trimmed = key.trim();
  return builtinByNorm.get(normalizePlaceholderKey(trimmed)) || trimmed;
}

export function isBuiltinPlaceholder(key: string): boolean {
  return builtinByNorm.has(normalizePlaceholderKey(key));
}

/** Look up a value by key, ignoring differences in letter case. */
export function lookupPlaceholderValue(
  values: Record<string, string>,
  key: string,
): string | undefined {
  if (Object.prototype.hasOwnProperty.call(values, key)) return values[key];
  const norm = normalizePlaceholderKey(key);
  for (const [k, v] of Object.entries(values)) {
    if (normalizePlaceholderKey(k) === norm) return v;
  }
  return undefined;
}

function pad(n: number) {
  return String(n).padStart(2, "0");
}

function formatReadableDate(day: { year: number; month: number; day: number }): string {
  try {
    const d = new Date(Date.UTC(day.year, day.month - 1, day.day, 12));
    return new Intl.DateTimeFormat("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      timeZone: "UTC",
    }).format(d);
  } catch {
    return `${day.year}-${pad(day.month)}-${pad(day.day)}`;
  }
}

function formatWeekday(day: { year: number; month: number; day: number }): string {
  try {
    const d = new Date(Date.UTC(day.year, day.month - 1, day.day, 12));
    return new Intl.DateTimeFormat("en-US", { weekday: "long", timeZone: "UTC" }).format(d);
  } catch {
    return "";
  }
}

/** Resolve built-in placeholder values from create-flow context. */
export function resolveBuiltinPlaceholderValues(
  ctx: BuiltinPlaceholderContext,
): Record<string, string> {
  const lat =
    ctx.latitude != null && Number.isFinite(ctx.latitude) ? String(ctx.latitude) : "";
  const lng =
    ctx.longitude != null && Number.isFinite(ctx.longitude) ? String(ctx.longitude) : "";

  const byCanonical: Record<string, string> = {
    club: (ctx.clubName || "").trim(),
    liveEvent: (ctx.liveEventName || "").trim(),
    category: (ctx.category || "").trim(),
    title: (ctx.title || "").trim(),
    address: (ctx.address || "").trim(),
    lat,
    lng,
    startTime: (ctx.startTime || "").trim(),
    endTime: (ctx.endTime || "").trim(),
    timezone: (ctx.timeZone || "").trim(),
  };
  if (ctx.date) {
    byCanonical.date = formatReadableDate(ctx.date);
    byCanonical.dateShort = `${ctx.date.year}-${pad(ctx.date.month)}-${pad(ctx.date.day)}`;
    byCanonical.weekday = formatWeekday(ctx.date);
  } else {
    byCanonical.date = "";
    byCanonical.dateShort = "";
    byCanonical.weekday = "";
  }

  const out: Record<string, string> = {};
  for (const b of BUILTIN_PLACEHOLDERS) {
    out[b.key] = byCanonical[b.key] ?? "";
  }
  return out;
}

/** Collect unique placeholder keys from a string, in order of first appearance. */
export function extractPlaceholderKeys(text: string): string[] {
  const keys: string[] = [];
  const seen = new Set<string>();
  PLACEHOLDER_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = PLACEHOLDER_RE.exec(text)) !== null) {
    const key = canonicalPlaceholderKey(m[1]);
    const norm = normalizePlaceholderKey(key);
    if (!seen.has(norm)) {
      seen.add(norm);
      keys.push(key);
    }
  }
  return keys;
}

/** Walk a JSON-like value and collect placeholder keys from all strings. */
export function extractPlaceholdersFromValue(value: unknown): string[] {
  const keys: string[] = [];
  const seen = new Set<string>();

  function walk(v: unknown, skipKeys: Set<string>) {
    if (typeof v === "string") {
      for (const k of extractPlaceholderKeys(v)) {
        const norm = normalizePlaceholderKey(k);
        if (!seen.has(norm)) {
          seen.add(norm);
          keys.push(k);
        }
      }
      return;
    }
    if (Array.isArray(v)) {
      for (const item of v) walk(item, skipKeys);
      return;
    }
    if (v && typeof v === "object") {
      for (const [k, child] of Object.entries(v as Record<string, unknown>)) {
        if (skipKeys.has(k)) continue;
        walk(child, skipKeys);
      }
    }
  }

  walk(value, new Set(["placeholders"]));
  return keys;
}

/** Merge explicit defs with keys found in the payload strings. */
export function resolvePlaceholderDefs(
  payload: Record<string, unknown>,
  explicit?: PlaceholderDef[],
): PlaceholderDef[] {
  const found = extractPlaceholdersFromValue(payload);
  const byNorm = new Map<string, PlaceholderDef>();
  for (const d of explicit || []) {
    if (!d?.key) continue;
    const key = canonicalPlaceholderKey(d.key);
    byNorm.set(normalizePlaceholderKey(key), {
      key,
      label: d.label,
      default: d.default ?? "",
    });
  }
  const ordered: PlaceholderDef[] = [];
  const used = new Set<string>();
  for (const d of explicit || []) {
    if (!d?.key) continue;
    const key = canonicalPlaceholderKey(d.key);
    const norm = normalizePlaceholderKey(key);
    if (used.has(norm)) continue;
    if (!found.some((f) => normalizePlaceholderKey(f) === norm)) continue;
    ordered.push(byNorm.get(norm)!);
    used.add(norm);
  }
  for (const key of found) {
    const norm = normalizePlaceholderKey(key);
    if (used.has(norm)) continue;
    ordered.push(byNorm.get(norm) || { key });
    used.add(norm);
  }
  return ordered;
}

/** Custom (user-filled) placeholders only — excludes built-ins. */
export function resolveCustomPlaceholderDefs(
  payload: Record<string, unknown>,
  explicit?: PlaceholderDef[],
): PlaceholderDef[] {
  return resolvePlaceholderDefs(payload, explicit).filter((d) => !isBuiltinPlaceholder(d.key));
}

export function applyPlaceholders(text: string, values: Record<string, string>): string {
  return text.replace(PLACEHOLDER_RE, (_, key: string) => {
    const v = lookupPlaceholderValue(values, key);
    return v != null ? v : `{{${key}}}`;
  });
}

/** Deep-clone payload and substitute placeholders in every string field. */
export function applyPlaceholdersToPayload<T>(payload: T, values: Record<string, string>): T {
  function walk(v: unknown): unknown {
    if (typeof v === "string") return applyPlaceholders(v, values);
    if (Array.isArray(v)) return v.map(walk);
    if (v && typeof v === "object") {
      const out: Record<string, unknown> = {};
      for (const [k, child] of Object.entries(v as Record<string, unknown>)) {
        if (k === "placeholders") {
          out[k] = child;
          continue;
        }
        out[k] = walk(child);
      }
      return out;
    }
    return v;
  }
  return walk(payload) as T;
}

export function humanizePlaceholderKey(key: string): string {
  return key
    .replace(/[-_]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}
