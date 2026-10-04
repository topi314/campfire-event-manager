/** Wall-clock datetime for `<input type="datetime-local">`: YYYY-MM-DDTHH:mm */

/** Minimum meetup length (Campfire create/edit + templates). */
export const MIN_MEETUP_DURATION_MINUTES = 15;

export type TimeOfDay = { hours: number; minutes: number };

export type CalendarDate = { year: number; month: number; day: number }; // month 1–12

export type LocalDateTime = CalendarDate & TimeOfDay & { seconds?: number };

const pad = (n: number) => String(n).padStart(2, "0");

function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

/** Format a Date's *browser-local* components as datetime-local. */
export function toLocalInput(d: Date): string {
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function formatLocalDateTimeParts(p: LocalDateTime): string {
  return `${p.year}-${pad(p.month)}-${pad(p.day)}T${pad(p.hours)}:${pad(p.minutes)}`;
}

/** Parse datetime-local / offset-less ISO as explicit wall-clock components (no TZ shift). */
export function parseLocalDateTime(value: string | undefined | null): LocalDateTime | null {
  if (!value?.trim()) return null;
  const trimmed = value.trim();
  if (/[zZ]$/.test(trimmed) || /[+-]\d{2}:?\d{2}$/.test(trimmed)) return null;
  const m = trimmed.match(
    /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?)?/,
  );
  if (!m) return null;
  return {
    year: Number(m[1]),
    month: Number(m[2]),
    day: Number(m[3]),
    hours: m[4] != null ? Number(m[4]) : 0,
    minutes: m[5] != null ? Number(m[5]) : 0,
    seconds: m[6] != null ? Number(m[6]) : 0,
  };
}

/** Wall-clock parts of an instant in a given IANA timezone. */
export function partsInTimeZone(date: Date, timeZone: string): LocalDateTime | null {
  if (Number.isNaN(date.getTime())) return null;
  try {
    const fmt = new Intl.DateTimeFormat("en-US", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    });
    const map: Record<string, string> = {};
    for (const p of fmt.formatToParts(date)) {
      if (p.type !== "literal") map[p.type] = p.value;
    }
    return {
      year: Number(map.year),
      month: Number(map.month),
      day: Number(map.day),
      hours: Number(map.hour),
      minutes: Number(map.minute),
      seconds: Number(map.second || "0"),
    };
  } catch {
    return null;
  }
}

/**
 * Convert wall-clock parts in `timeZone` to a UTC Date.
 * Uses a two-pass offset correction so DST edges stay accurate.
 */
export function zonedPartsToUtcDate(parts: LocalDateTime, timeZone: string): Date {
  const tz = timeZone || browserTimeZone();
  let utcMs = Date.UTC(
    parts.year,
    parts.month - 1,
    parts.day,
    parts.hours,
    parts.minutes,
    parts.seconds || 0,
    0,
  );
  for (let i = 0; i < 2; i++) {
    const seen = partsInTimeZone(new Date(utcMs), tz);
    if (!seen) break;
    const asIfUtc = Date.UTC(
      seen.year,
      seen.month - 1,
      seen.day,
      seen.hours,
      seen.minutes,
      seen.seconds || 0,
      0,
    );
    const desired = Date.UTC(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hours,
      parts.minutes,
      parts.seconds || 0,
      0,
    );
    utcMs += desired - asIfUtc;
  }
  return new Date(utcMs);
}

function fromBrowserLocalParts(p: LocalDateTime): Date {
  return new Date(
    p.year,
    p.month - 1,
    p.day,
    p.hours,
    p.minutes,
    p.seconds || 0,
    0,
  );
}

/**
 * Convert a datetime-local string to UTC ISO for Campfire.
 * When `timeZone` is set, the wall clock is interpreted in that zone;
 * otherwise the browser's local zone is used.
 */
export function toISO(local: string, timeZone?: string): string {
  const parts = parseLocalDateTime(local);
  if (parts) {
    const d = timeZone
      ? zonedPartsToUtcDate(parts, timeZone)
      : fromBrowserLocalParts(parts);
    return Number.isNaN(d.getTime()) ? "" : d.toISOString();
  }
  const d = new Date(local);
  return Number.isNaN(d.getTime()) ? "" : d.toISOString();
}

/** Parse "HH:mm", offset-less datetime, or an instant into hours/minutes. */
export function parseTimeOfDay(
  value: string | undefined | null,
  timeZone?: string,
): TimeOfDay | null {
  if (!value?.trim()) return null;
  const trimmed = value.trim();

  const hm = trimmed.match(/^(\d{1,2}):(\d{2})(?::\d{2})?$/);
  if (hm) {
    return { hours: Number(hm[1]), minutes: Number(hm[2]) };
  }

  const localParts = parseLocalDateTime(trimmed);
  if (localParts && /T|\s\d{2}:/.test(trimmed)) {
    return { hours: localParts.hours, minutes: localParts.minutes };
  }

  if (/[zZ]|[+-]\d{2}:?\d{2}/.test(trimmed)) {
    const d = new Date(trimmed);
    if (Number.isNaN(d.getTime())) return null;
    if (timeZone) {
      const z = partsInTimeZone(d, timeZone);
      if (z) return { hours: z.hours, minutes: z.minutes };
    }
    return { hours: d.getHours(), minutes: d.getMinutes() };
  }

  const d = new Date(trimmed);
  if (Number.isNaN(d.getTime())) return null;
  return { hours: d.getHours(), minutes: d.getMinutes() };
}

/**
 * Clock time only — HH:mm or offset-less local datetime.
 * Rejects absolute ISO timestamps (Campfire worldwide windows become early morning locally).
 */
export function parseWallClockTimeOfDay(value: string | undefined | null): TimeOfDay | null {
  if (!value?.trim()) return null;
  const trimmed = value.trim();
  if (/[zZ]|[+-]\d{2}:?\d{2}/.test(trimmed)) return null;

  const hm = trimmed.match(/^(\d{1,2}):(\d{2})(?::\d{2})?$/);
  if (hm) {
    return { hours: Number(hm[1]), minutes: Number(hm[2]) };
  }

  const localParts = parseLocalDateTime(trimmed);
  if (localParts && /T|\s\d{2}:/.test(trimmed)) {
    return { hours: localParts.hours, minutes: localParts.minutes };
  }
  return null;
}

export function formatTimeOfDay(t: TimeOfDay): string {
  return `${pad(t.hours)}:${pad(t.minutes)}`;
}

/** Read YYYY-MM-DD from the start of a string without timezone conversion. */
export function parseCalendarDate(value: string | undefined | null): CalendarDate | null {
  if (!value?.trim()) return null;
  const m = value.trim().match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!m) return null;
  const year = Number(m[1]);
  const month = Number(m[2]);
  const day = Number(m[3]);
  if (month < 1 || month > 12 || day < 1 || day > 31) return null;
  return { year, month, day };
}

/**
 * Calendar day for applying template clock times.
 * Prefers Campfire `localStartTime`, then the instant in `timeZone`,
 * then the UTC date prefix of `startTimestamp` as a last resort.
 */
export function calendarDateFromLiveEvent(
  ev: {
    startTimestamp: string;
    localStartTime?: string;
  },
  timeZone?: string,
): CalendarDate {
  const fromLocal = parseCalendarDate(ev.localStartTime);
  if (fromLocal) return fromLocal;

  const d = new Date(ev.startTimestamp);
  if (!Number.isNaN(d.getTime())) {
    if (timeZone) {
      const z = partsInTimeZone(d, timeZone);
      if (z) return { year: z.year, month: z.month, day: z.day };
    }
    return {
      year: d.getFullYear(),
      month: d.getMonth() + 1,
      day: d.getDate(),
    };
  }

  return (
    parseCalendarDate(ev.startTimestamp) || {
      year: new Date().getFullYear(),
      month: new Date().getMonth() + 1,
      day: new Date().getDate(),
    }
  );
}

/** @deprecated Prefer calendarDateFromLiveEvent */
export function calendarDateFromIso(iso: string): Date {
  const cal = parseCalendarDate(iso);
  if (cal) return new Date(cal.year, cal.month - 1, cal.day);
  const d = new Date(iso);
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

/** Apply template hours onto a live-event calendar day (naive wall clock). */
export function combineDayAndTime(
  day: CalendarDate | string,
  time: TimeOfDay,
): string {
  const cal =
    typeof day === "string"
      ? parseCalendarDate(day) ||
        (() => {
          const d = new Date(day);
          return {
            year: d.getFullYear(),
            month: d.getMonth() + 1,
            day: d.getDate(),
          };
        })()
      : day;
  return formatLocalDateTimeParts({
    year: cal.year,
    month: cal.month,
    day: cal.day,
    hours: time.hours,
    minutes: time.minutes,
  });
}

export function addMinutesToLocal(local: string, minutes: number): string {
  const parts = parseLocalDateTime(local);
  if (!parts) {
    const d = new Date(local);
    if (Number.isNaN(d.getTime())) return local;
    d.setMinutes(d.getMinutes() + minutes);
    return toLocalInput(d);
  }
  // Add on the wall clock via UTC math on the date components (no TZ — naive).
  const utc = Date.UTC(
    parts.year,
    parts.month - 1,
    parts.day,
    parts.hours,
    parts.minutes,
    parts.seconds || 0,
  );
  const next = new Date(utc + minutes * 60_000);
  return formatLocalDateTimeParts({
    year: next.getUTCFullYear(),
    month: next.getUTCMonth() + 1,
    day: next.getUTCDate(),
    hours: next.getUTCHours(),
    minutes: next.getUTCMinutes(),
  });
}

/** Compare two datetime-local strings as naive wall clocks (ms). */
export function localDateTimeDiffMs(start: string, end: string): number | null {
  const a = parseLocalDateTime(start);
  const b = parseLocalDateTime(end);
  if (!a || !b) return null;
  const startMs = Date.UTC(a.year, a.month - 1, a.day, a.hours, a.minutes, a.seconds || 0);
  const endMs = Date.UTC(b.year, b.month - 1, b.day, b.hours, b.minutes, b.seconds || 0);
  return endMs - startMs;
}

/** True when end is at least MIN_MEETUP_DURATION_MINUTES after start. */
export function isMeetupDurationValid(start: string, end: string): boolean {
  const diff = localDateTimeDiffMs(start, end);
  if (diff == null) return false;
  return diff >= MIN_MEETUP_DURATION_MINUTES * 60_000;
}

/**
 * Return an end datetime-local that is at least `minutes` after start.
 * If `end` already satisfies that, it is returned unchanged.
 */
export function ensureMinMeetupEnd(
  start: string,
  end?: string | null,
  minutes = MIN_MEETUP_DURATION_MINUTES,
): string {
  const minEnd = addMinutesToLocal(start, minutes);
  if (!end?.trim()) return minEnd;
  const diff = localDateTimeDiffMs(start, end);
  if (diff == null || diff < minutes * 60_000) return minEnd;
  return end;
}

function minutesOfDay(t: TimeOfDay): number {
  return t.hours * 60 + t.minutes;
}

/** Add minutes to an HH:mm clock (wraps at midnight). */
export function addMinutesToTimeOfDay(time: string, minutes: number): string {
  const t = parseTimeOfDay(time) ?? parseWallClockTimeOfDay(time);
  if (!t) return time;
  const total = ((minutesOfDay(t) + minutes) % (24 * 60) + 24 * 60) % (24 * 60);
  return formatTimeOfDay({ hours: Math.floor(total / 60), minutes: total % 60 });
}

/**
 * Ensure template end clock is at least `minutes` after start on the same day.
 * Overnight (end before start) is treated as too short and bumped to start+minutes
 * (clamped to 23:59 if that would cross midnight).
 */
export function ensureMinMeetupEndTime(
  start: string,
  end?: string | null,
  minutes = MIN_MEETUP_DURATION_MINUTES,
): string {
  const s = parseTimeOfDay(start) ?? parseWallClockTimeOfDay(start);
  if (!s) return (end || "").trim() || addMinutesToTimeOfDay("00:00", minutes);
  const total = minutesOfDay(s) + minutes;
  const minEnd =
    total >= 24 * 60
      ? "23:59"
      : formatTimeOfDay({ hours: Math.floor(total / 60), minutes: total % 60 });
  const e = end ? parseTimeOfDay(end) ?? parseWallClockTimeOfDay(end) : null;
  if (!e) return minEnd;
  if (minutesOfDay(e) < minutesOfDay(s) + minutes) return minEnd;
  return formatTimeOfDay(e);
}

export function isMeetupTimeOfDayDurationValid(
  start: string,
  end: string,
  minutes = MIN_MEETUP_DURATION_MINUTES,
): boolean {
  const s = parseTimeOfDay(start) ?? parseWallClockTimeOfDay(start);
  const e = parseTimeOfDay(end) ?? parseWallClockTimeOfDay(end);
  if (!s || !e) return false;
  return minutesOfDay(e) - minutesOfDay(s) >= minutes;
}

/** Display a datetime-local value (wall clock, no zone conversion). */
export function formatLocalDateTime(value: string | undefined | null): string {
  if (!value?.trim()) return "—";
  const parts = parseLocalDateTime(value);
  if (parts) {
    const label = `${parts.year}-${pad(parts.month)}-${pad(parts.day)} ${pad(parts.hours)}:${pad(parts.minutes)}`;
    return label;
  }
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

/** Absolute timestamps → locale string in optional IANA zone. */
export function formatInstant(
  value: string | undefined | null,
  timeZone?: string,
): string {
  if (!value?.trim()) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: timeZone || undefined,
  });
}

export function formatInstantDate(
  value: string | undefined | null,
  timeZone?: string,
): string {
  if (!value?.trim()) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleDateString(undefined, {
    dateStyle: "medium",
    timeZone: timeZone || undefined,
  });
}
