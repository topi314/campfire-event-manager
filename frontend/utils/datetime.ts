import { guessTimeFormat, isTimeFormatId, type TimeFormatId } from "~/utils/timeFormats";

/** Wall-clock datetime for `<input type="datetime-local">`: YYYY-MM-DDTHH:mm */

/** Minimum meetup length (Campfire create/edit + templates). */
export const MIN_MEETUP_DURATION_MINUTES = 15;

/** Maximum meetup length (create/edit). Exactly 24 hours is allowed. */
export const MAX_MEETUP_DURATION_MINUTES = 24 * 60;

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

const hasExplicitOffset = (value: string) =>
  /[zZ]$/.test(value) || /[+-]\d{2}:?\d{2}$/.test(value);

/**
 * Parse a Campfire absolute timestamp.
 * Offset-less datetimes are treated as UTC (Campfire / Go convention).
 * ECMAScript would otherwise read them as browser-local wall clocks.
 */
export function parseCampfireInstant(value: string | undefined | null): Date | null {
  if (!value?.trim()) return null;
  const trimmed = value.trim();
  let raw = trimmed;
  if (!hasExplicitOffset(trimmed) && /T|\s\d{2}:/.test(trimmed)) {
    // "2026-10-15T08:00:00.000" → UTC instant, not local 8am.
    raw = trimmed.replace(/\s+/, "T").replace(/[Zz]$/, "") + "Z";
  }
  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? null : d;
}

/** Campfire instant → datetime-local wall clock in `timeZone`. */
export function instantToLocalInput(
  iso: string | undefined | null,
  timeZone?: string,
): string {
  const d = parseCampfireInstant(iso);
  if (!d) return "";
  const tz = timeZone || browserTimeZone();
  const parts = partsInTimeZone(d, tz);
  return parts ? formatLocalDateTimeParts(parts) : "";
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

/** True when end is between the min and max meetup length after start. */
export function isMeetupDurationValid(start: string, end: string): boolean {
  return meetupDurationError(start, end) == null;
}

/**
 * Why a meetup length is rejected, or null when it is within min and max.
 * Unparseable times are treated as too short so callers keep a hard failure.
 */
export function meetupDurationError(start: string, end: string): string | null {
  const diff = localDateTimeDiffMs(start, end);
  if (diff == null || diff < MIN_MEETUP_DURATION_MINUTES * 60_000) {
    return `End must be at least ${MIN_MEETUP_DURATION_MINUTES} minutes after start`;
  }
  if (diff > MAX_MEETUP_DURATION_MINUTES * 60_000) {
    return "Meetup cannot be longer than 24 hours";
  }
  return null;
}

/**
 * Parse Campfire live-event start/end timestamps into UTC instants.
 * Returns null when either bound is missing or end is before start.
 */
export function liveEventWindowBounds(ev: {
  startTimestamp?: string | null;
  endTimestamp?: string | null;
}): { start: Date; end: Date } | null {
  const start = parseCampfireInstant(ev.startTimestamp);
  const end = parseCampfireInstant(ev.endTimestamp);
  if (!start || !end) return null;
  if (end.getTime() < start.getTime()) return null;
  return { start, end };
}

/**
 * True when meetup start/end (wall clocks in `timeZone`) fall inside the live
 * event's Campfire timestamp window. Missing/unparseable window → true (skip).
 */
export function isMeetupWithinLiveEventWindow(
  eventTime: string,
  eventEndTime: string,
  live: { startTimestamp?: string | null; endTimestamp?: string | null },
  timeZone?: string,
): boolean {
  const window = liveEventWindowBounds(live);
  if (!window) return true;
  const startIso = toISO(eventTime, timeZone);
  const endIso = toISO(eventEndTime, timeZone);
  if (!startIso || !endIso) return true;
  const startMs = new Date(startIso).getTime();
  const endMs = new Date(endIso).getTime();
  if (Number.isNaN(startMs) || Number.isNaN(endMs)) return true;
  return startMs >= window.start.getTime() && endMs <= window.end.getTime();
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

const MONTH_ABBR = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

function clock12(hours: number, minutes: number, upper: boolean): string {
  const pm = hours >= 12;
  let h = hours % 12;
  if (h === 0) h = 12;
  const mark = upper ? (pm ? "PM" : "AM") : pm ? "pm" : "am";
  return `${h}:${pad(minutes)} ${mark}`;
}

function clock24(hours: number, minutes: number, sep: ":" | "." | "h"): string {
  if (sep === "h") return `${pad(hours)} h ${pad(minutes)}`;
  return `${pad(hours)}${sep}${pad(minutes)}`;
}

export type LayoutPieces = {
  date: string;
  time: string;
  join: "comma" | "space" | "time-first";
};

/** Date and clock pieces for a Settings time-format preset. */
export function layoutPieces(
  id: TimeFormatId,
  p: { year: number; month: number; day: number; hours: number; minutes: number },
): LayoutPieces {
  const mon = MONTH_ABBR[p.month - 1] || pad(p.month);
  const day = String(p.day);
  const dd = pad(p.day);
  const mm = pad(p.month);
  const yyyy = String(p.year);
  let date = `${day} ${mon} ${yyyy}`;
  let time = clock24(p.hours, p.minutes, ":");
  let join: LayoutPieces["join"] = "comma";
  switch (id) {
    case "mdy-12":
      date = `${mon} ${day}, ${yyyy}`;
      time = clock12(p.hours, p.minutes, true);
      break;
    case "mdy-slash-12":
      date = `${mm}/${dd}/${yyyy}`;
      time = clock12(p.hours, p.minutes, true);
      break;
    case "dmy-12":
      date = `${day} ${mon} ${yyyy}`;
      time = clock12(p.hours, p.minutes, false);
      break;
    case "dmy-slash-12":
      date = `${dd}/${mm}/${yyyy}`;
      time = clock12(p.hours, p.minutes, false);
      break;
    case "dmy-24":
      date = `${day} ${mon} ${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      break;
    case "dmy-dot-24":
      date = `${dd}.${mm}.${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      break;
    case "dmy-slash-24":
      date = `${dd}/${mm}/${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      break;
    case "dmy-dash-24":
      date = `${dd}-${mm}-${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      break;
    case "dmy-dot-time":
      date = `${dd}.${mm}.${yyyy}`;
      time = clock24(p.hours, p.minutes, ".");
      break;
    case "dmy-h":
      date = `${day} ${mon} ${yyyy}`;
      time = clock24(p.hours, p.minutes, "h");
      break;
    case "ymd-24":
      date = `${yyyy}-${mm}-${dd}`;
      time = clock24(p.hours, p.minutes, ":");
      join = "space";
      break;
    case "ymd-slash-24":
      date = `${yyyy}/${mm}/${dd}`;
      time = clock24(p.hours, p.minutes, ":");
      join = "space";
      break;
    case "ymd-dot-24":
      date = `${yyyy}.${mm}.${dd}`;
      time = clock24(p.hours, p.minutes, ":");
      join = "space";
      break;
    case "hm-dmy":
      date = `${day} ${mon} ${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      join = "time-first";
      break;
    case "hm-dmy-slash":
      date = `${dd}/${mm}/${yyyy}`;
      time = clock24(p.hours, p.minutes, ":");
      join = "time-first";
      break;
  }
  return { date, time, join };
}

/** Human-readable wall clock. `format` is a time-format preset id. */
export function formatDateTimeParts(
  p: LocalDateTime,
  format?: string | null,
  dateOnly = false,
): string {
  const id = isTimeFormatId(format) ? format : guessTimeFormat();
  const pieces = layoutPieces(id, p);
  if (dateOnly) return pieces.date;
  if (pieces.join === "time-first") return `${pieces.time}, ${pieces.date}`;
  if (pieces.join === "space") return `${pieces.date} ${pieces.time}`;
  return `${pieces.date}, ${pieces.time}`;
}

/** Display a datetime-local value (wall clock, no zone conversion). */
export function formatLocalDateTime(
  value: string | undefined | null,
  timeFormat?: string | null,
): string {
  if (!value?.trim()) return "—";
  const parts = parseLocalDateTime(value);
  if (parts) return formatDateTimeParts(parts, timeFormat);
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  const zoned = partsInTimeZone(d, browserTimeZone());
  return zoned ? formatDateTimeParts(zoned, timeFormat) : value;
}

/** Absolute timestamps → formatted string in an optional IANA zone. */
export function formatInstant(
  value: string | undefined | null,
  timeZone?: string,
  timeFormat?: string | null,
): string {
  if (!value?.trim()) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  const parts = partsInTimeZone(d, timeZone || browserTimeZone());
  return parts ? formatDateTimeParts(parts, timeFormat) : value;
}

/** End of a range. Same local calendar day as `start` keeps only the clock time. */
export function formatInstantRangeEnd(
  start: string | undefined | null,
  end: string | undefined | null,
  timeZone?: string,
  timeFormat?: string | null,
): string {
  if (!end?.trim()) return "";
  const endDate = new Date(end);
  if (Number.isNaN(endDate.getTime())) return end;
  const zone = timeZone || browserTimeZone();
  const endParts = partsInTimeZone(endDate, zone);
  if (!endParts) return end;
  const startDate = start?.trim() ? new Date(start) : null;
  const startParts =
    startDate && !Number.isNaN(startDate.getTime()) ? partsInTimeZone(startDate, zone) : null;
  const sameDay =
    !!startParts &&
    startParts.year === endParts.year &&
    startParts.month === endParts.month &&
    startParts.day === endParts.day;
  if (!sameDay) return formatDateTimeParts(endParts, timeFormat);
  const id = isTimeFormatId(timeFormat) ? timeFormat : guessTimeFormat();
  return layoutPieces(id, endParts).time;
}

export function formatInstantDate(
  value: string | undefined | null,
  timeZone?: string,
  timeFormat?: string | null,
): string {
  if (!value?.trim()) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  const parts = partsInTimeZone(d, timeZone || browserTimeZone());
  return parts ? formatDateTimeParts(parts, timeFormat, true) : value;
}
