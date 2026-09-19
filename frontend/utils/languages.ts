/** Languages offered when publishing templates (ISO 639-1 codes). */
export const TEMPLATE_LANGUAGES = [
  { code: "en", label: "English" },
  { code: "de", label: "German" },
  { code: "fr", label: "French" },
  { code: "es", label: "Spanish" },
  { code: "nl", label: "Dutch" },
  { code: "it", label: "Italian" },
  { code: "pt", label: "Portuguese" },
  { code: "pl", label: "Polish" },
  { code: "cs", label: "Czech" },
  { code: "sk", label: "Slovak" },
  { code: "hu", label: "Hungarian" },
  { code: "ro", label: "Romanian" },
  { code: "sv", label: "Swedish" },
  { code: "da", label: "Danish" },
  { code: "nb", label: "Norwegian" },
  { code: "fi", label: "Finnish" },
  { code: "tr", label: "Turkish" },
  { code: "uk", label: "Ukrainian" },
  { code: "ru", label: "Russian" },
  { code: "ja", label: "Japanese" },
  { code: "ko", label: "Korean" },
  { code: "zh", label: "Chinese" },
] as const;

export type TemplateLanguageCode = (typeof TEMPLATE_LANGUAGES)[number]["code"];

const byCode = new Map(TEMPLATE_LANGUAGES.map((l) => [l.code, l.label]));

export function isTemplateLanguageCode(code: string): code is TemplateLanguageCode {
  return byCode.has(code as TemplateLanguageCode);
}

export function templateLanguageLabel(code: string | null | undefined): string {
  const c = (code || "").trim().toLowerCase();
  if (!c) return "";
  return byCode.get(c as TemplateLanguageCode) || c.toUpperCase();
}

/** Detect a reasonable default from the browser locale (e.g. de-DE → de). */
export function guessTemplateLanguage(): TemplateLanguageCode {
  if (typeof navigator === "undefined") return "en";
  const raw = (navigator.language || "en").trim().toLowerCase();
  const primary = raw.split("-")[0] || "en";
  if (isTemplateLanguageCode(primary)) return primary;
  return "en";
}
