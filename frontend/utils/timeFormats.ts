export const TIME_FORMAT_IDS = [
  "mdy-12",
  "mdy-slash-12",
  "dmy-12",
  "dmy-slash-12",
  "dmy-24",
  "dmy-dot-24",
  "dmy-slash-24",
  "dmy-dash-24",
  "dmy-dot-time",
  "dmy-h",
  "ymd-24",
  "ymd-slash-24",
  "ymd-dot-24",
  "hm-dmy",
  "hm-dmy-slash",
] as const;

export type TimeFormatId = (typeof TIME_FORMAT_IDS)[number];

export type TimeFormatPreset = {
  id: TimeFormatId;
  /** Example for 18 Oct 2026, 15:45. Also the Settings option label. */
  example: string;
  hour12: boolean;
};

export const TIME_FORMATS: TimeFormatPreset[] = [
  { id: "mdy-12", example: "Oct 18, 2026, 3:45 PM", hour12: true },
  { id: "mdy-slash-12", example: "10/18/2026, 3:45 PM", hour12: true },
  { id: "dmy-12", example: "18 Oct 2026, 3:45 pm", hour12: true },
  { id: "dmy-slash-12", example: "18/10/2026, 3:45 pm", hour12: true },
  { id: "dmy-24", example: "18 Oct 2026, 15:45", hour12: false },
  { id: "dmy-dot-24", example: "18.10.2026, 15:45", hour12: false },
  { id: "dmy-slash-24", example: "18/10/2026, 15:45", hour12: false },
  { id: "dmy-dash-24", example: "18-10-2026, 15:45", hour12: false },
  { id: "dmy-dot-time", example: "18.10.2026, 15.45", hour12: false },
  { id: "dmy-h", example: "18 Oct 2026, 15 h 45", hour12: false },
  { id: "ymd-24", example: "2026-10-18 15:45", hour12: false },
  { id: "ymd-slash-24", example: "2026/10/18 15:45", hour12: false },
  { id: "ymd-dot-24", example: "2026.10.18 15:45", hour12: false },
  { id: "hm-dmy", example: "15:45, 18 Oct 2026", hour12: false },
  { id: "hm-dmy-slash", example: "15:45, 18/10/2026", hour12: false },
];

const ID_SET = new Set<string>(TIME_FORMAT_IDS);

export function isTimeFormatId(value: string | null | undefined): value is TimeFormatId {
  return !!value && ID_SET.has(value);
}

export function timeFormatExample(id: TimeFormatId): string {
  return TIME_FORMATS.find((p) => p.id === id)?.example || TIME_FORMATS[0].example;
}

function browserUses12Hour(): boolean {
  try {
    const cycle = Intl.DateTimeFormat().resolvedOptions().hourCycle;
    if (cycle === "h12" || cycle === "h11") return true;
    if (cycle === "h23" || cycle === "h24") return false;
  } catch {
    /* ignore */
  }
  try {
    const sample = new Intl.DateTimeFormat(undefined, { hour: "numeric" }).format(new Date(2026, 0, 1, 15, 0));
    return /am|pm/i.test(sample);
  } catch {
    return false;
  }
}

/** Browser hour cycle, with month-first only for English 12-hour clocks. */
export function guessTimeFormat(): TimeFormatId {
  if (!browserUses12Hour()) return "dmy-24";
  let lang = "";
  try {
    lang = (navigator.language || "").toLowerCase();
  } catch {
    lang = "";
  }
  if (lang.startsWith("en")) return "mdy-12";
  return "dmy-12";
}
