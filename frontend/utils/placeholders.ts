/** Match {{key}} or {{key[1]}} tokens. */
export const PLACEHOLDER_RE = /\{\{\s*([a-zA-Z][a-zA-Z0-9_-]*(?:\[[0-9]+\])?)\s*\}\}/g;

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
  /** Help section grouping. */
  group?: "core" | "eventPokemon";
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
  /** Extra values from backend event-pokemon resolve. */
  eventPokemonValues?: Record<string, string>;
};

export const EVENT_POKEMON_BASE_KEYS = [
  "eventPokemon",
  "eventPokemonCeilingResearch",
  "eventPokemonCeilingRaid",
  "eventPokemonCeilingEgg",
  "eventPokemonCeilingRaidWeather",
  "eventPokemonFloorResearch",
  "eventPokemonFloorRaid",
  "eventPokemonFloorEgg",
  "eventPokemonFloorRaidWeather",
] as const;

export const BUILTIN_PLACEHOLDERS: BuiltinPlaceholder[] = [
  {
    key: "club",
    label: "Club",
    description: "Name of the Campfire club you selected.",
    example: "PoGo Offenbach",
    group: "core",
  },
  {
    key: "liveEvent",
    label: "Live event",
    description: "Campfire live event title, or empty if none is linked.",
    example: "Community Day",
    group: "core",
  },
  {
    key: "category",
    label: "Category",
    description: "Inferred live-event category (e.g. Community Day, Raid Hour).",
    example: "Community Day",
    group: "core",
  },
  {
    key: "title",
    label: "Title",
    description: "Meetup title from the create form (useful in the description).",
    example: "Community Day — Downtown",
    group: "core",
  },
  {
    key: "address",
    label: "Address",
    description: "Address field from the create form.",
    example: "Main Station, Platform 3",
    group: "core",
  },
  {
    key: "lat",
    label: "Latitude",
    description: "Map pin latitude (before jitter).",
    example: "50.110922",
    group: "core",
  },
  {
    key: "lng",
    label: "Longitude",
    description: "Map pin longitude (before jitter).",
    example: "8.682127",
    group: "core",
  },
  {
    key: "date",
    label: "Date",
    description: "Meetup calendar day as a readable date (from the live event, or today).",
    example: "Sep 30, 2026",
    group: "core",
  },
  {
    key: "dateShort",
    label: "Date (short)",
    description: "Meetup calendar day as YYYY-MM-DD.",
    example: "2026-09-30",
    group: "core",
  },
  {
    key: "weekday",
    label: "Weekday",
    description: "Weekday name for the meetup day.",
    example: "Wednesday",
    group: "core",
  },
  {
    key: "startTime",
    label: "Start time",
    description: "Meetup start clock time (HH:mm).",
    example: "14:00",
    group: "core",
  },
  {
    key: "endTime",
    label: "End time",
    description: "Meetup end clock time (HH:mm).",
    example: "17:00",
    group: "core",
  },
  {
    key: "timezone",
    label: "Timezone",
    description: "Your Settings timezone used for wall-clock times.",
    example: "Europe/Berlin",
    group: "core",
  },
  {
    key: "eventPokemon",
    label: "Event Pokémon",
    description:
      "Featured species extracted from the live event title (translated to the template language). Editable on create when used. Multiple names joined with commas.",
    example: "Squirtle, Wartortle",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonCeilingResearch",
    label: "Ceiling IV research CP/WP",
    description:
      "Ceiling IV (15/15/15) Combat Power at research level 15. Not editable. Unit follows template language (CP / WP / PC).",
    example: "1199 WP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonCeilingRaid",
    label: "Ceiling IV raid CP/WP",
    description: "Ceiling IV at raid catch level 20. Not editable.",
    example: "1598 CP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonCeilingEgg",
    label: "Ceiling IV egg CP/WP",
    description: "Ceiling IV at egg hatch level 20 (same number as raid). Not editable.",
    example: "1598 CP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonCeilingRaidWeather",
    label: "Ceiling IV weather raid CP/WP",
    description: "Ceiling IV at weather-boosted raid level 25. Not editable.",
    example: "1998 CP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonFloorResearch",
    label: "Floor IV research CP/WP",
    description: "Floor IV (10/10/10) at research level 15. Not editable.",
    example: "1142 WP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonFloorRaid",
    label: "Floor IV raid CP/WP",
    description: "Floor IV at raid level 20. Not editable.",
    example: "1522 CP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonFloorEgg",
    label: "Floor IV egg CP/WP",
    description: "Floor IV at egg level 20. Not editable.",
    example: "1522 CP",
    group: "eventPokemon",
  },
  {
    key: "eventPokemonFloorRaidWeather",
    label: "Floor IV weather raid CP/WP",
    description: "Floor IV at weather-boosted raid level 25. Not editable.",
    example: "1903 CP",
    group: "eventPokemon",
  },
];

/** Case-insensitive identity for placeholder keys. */
export function normalizePlaceholderKey(key: string): string {
  return key.trim().toLowerCase();
}

const INDEX_SUFFIX_RE = /\[([0-9]+)\]$/;

/** Split eventPokemon[2] → { base: "eventPokemon", index: 2 }. */
export function splitPlaceholderIndex(key: string): {
  base: string;
  index: number | null;
} {
  const trimmed = key.trim();
  const m = INDEX_SUFFIX_RE.exec(trimmed);
  if (!m) return { base: trimmed, index: null };
  return { base: trimmed.slice(0, m.index), index: Number(m[1]) };
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
  const { base, index } = splitPlaceholderIndex(trimmed);
  const canonBase = builtinByNorm.get(normalizePlaceholderKey(base));
  if (canonBase) {
    return index != null ? `${canonBase}[${index}]` : canonBase;
  }
  return trimmed;
}

export function isBuiltinPlaceholder(key: string): boolean {
  const { base } = splitPlaceholderIndex(key);
  return builtinByNorm.has(normalizePlaceholderKey(base));
}

/** True for eventPokemon / eventPokemon[i] (editable name builtins). */
export function isEventPokemonNameKey(key: string): boolean {
  const { base } = splitPlaceholderIndex(key);
  return normalizePlaceholderKey(base) === "eventpokemon";
}

export function isEventPokemonBaseKey(key: string): boolean {
  const { base } = splitPlaceholderIndex(key);
  const n = normalizePlaceholderKey(base);
  return EVENT_POKEMON_BASE_KEYS.some((k) => normalizePlaceholderKey(k) === n);
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
  // Event-pokemon values (and indexes) come from the backend resolve API.
  if (ctx.eventPokemonValues) {
    for (const [k, v] of Object.entries(ctx.eventPokemonValues)) {
      out[canonicalPlaceholderKey(k)] = v;
    }
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
    if (v != null) return v;
    // Event-pokemon builtins (incl. OOB indexes) resolve to empty when missing.
    if (isEventPokemonBaseKey(key)) return "";
    return `{{${key}}}`;
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
