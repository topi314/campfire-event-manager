/** Languages offered when publishing templates (ISO 639-1 codes). */
export const TEMPLATE_LANGUAGES = [
  { code: "af", label: "Afrikaans" },
  { code: "sq", label: "Albanian" },
  { code: "am", label: "Amharic" },
  { code: "ar", label: "Arabic" },
  { code: "hy", label: "Armenian" },
  { code: "az", label: "Azerbaijani" },
  { code: "eu", label: "Basque" },
  { code: "be", label: "Belarusian" },
  { code: "bn", label: "Bengali" },
  { code: "bs", label: "Bosnian" },
  { code: "bg", label: "Bulgarian" },
  { code: "my", label: "Burmese" },
  { code: "ca", label: "Catalan" },
  { code: "zh", label: "Chinese" },
  { code: "hr", label: "Croatian" },
  { code: "cs", label: "Czech" },
  { code: "da", label: "Danish" },
  { code: "nl", label: "Dutch" },
  { code: "en", label: "English" },
  { code: "eo", label: "Esperanto" },
  { code: "et", label: "Estonian" },
  { code: "fo", label: "Faroese" },
  { code: "fi", label: "Finnish" },
  { code: "fr", label: "French" },
  { code: "gl", label: "Galician" },
  { code: "ka", label: "Georgian" },
  { code: "de", label: "German" },
  { code: "el", label: "Greek" },
  { code: "gu", label: "Gujarati" },
  { code: "he", label: "Hebrew" },
  { code: "hi", label: "Hindi" },
  { code: "hu", label: "Hungarian" },
  { code: "is", label: "Icelandic" },
  { code: "id", label: "Indonesian" },
  { code: "ga", label: "Irish" },
  { code: "it", label: "Italian" },
  { code: "ja", label: "Japanese" },
  { code: "kn", label: "Kannada" },
  { code: "kk", label: "Kazakh" },
  { code: "km", label: "Khmer" },
  { code: "ko", label: "Korean" },
  { code: "ku", label: "Kurdish" },
  { code: "lo", label: "Lao" },
  { code: "lv", label: "Latvian" },
  { code: "lt", label: "Lithuanian" },
  { code: "lb", label: "Luxembourgish" },
  { code: "mk", label: "Macedonian" },
  { code: "ms", label: "Malay" },
  { code: "ml", label: "Malayalam" },
  { code: "mt", label: "Maltese" },
  { code: "mr", label: "Marathi" },
  { code: "mn", label: "Mongolian" },
  { code: "ne", label: "Nepali" },
  { code: "nb", label: "Norwegian" },
  { code: "nn", label: "Norwegian Nynorsk" },
  { code: "fa", label: "Persian" },
  { code: "pl", label: "Polish" },
  { code: "pt", label: "Portuguese" },
  { code: "pa", label: "Punjabi" },
  { code: "ro", label: "Romanian" },
  { code: "ru", label: "Russian" },
  { code: "sr", label: "Serbian" },
  { code: "si", label: "Sinhala" },
  { code: "sk", label: "Slovak" },
  { code: "sl", label: "Slovenian" },
  { code: "es", label: "Spanish" },
  { code: "sw", label: "Swahili" },
  { code: "sv", label: "Swedish" },
  { code: "tl", label: "Tagalog" },
  { code: "ta", label: "Tamil" },
  { code: "te", label: "Telugu" },
  { code: "th", label: "Thai" },
  { code: "tr", label: "Turkish" },
  { code: "uk", label: "Ukrainian" },
  { code: "ur", label: "Urdu" },
  { code: "uz", label: "Uzbek" },
  { code: "vi", label: "Vietnamese" },
  { code: "cy", label: "Welsh" },
  { code: "yi", label: "Yiddish" },
  { code: "zu", label: "Zulu" },
] as const;

export type TemplateLanguageCode = (typeof TEMPLATE_LANGUAGES)[number]["code"];

const byCode = new Map(TEMPLATE_LANGUAGES.map((l) => [l.code, l.label]));

const aliases: Record<string, TemplateLanguageCode> = {
  no: "nb",
  iw: "he",
  in: "id",
  ji: "yi",
  fil: "tl",
};

function resolveCode(code: string): TemplateLanguageCode | "" {
  const c = code.trim().toLowerCase();
  if (!c) return "";
  const mapped = aliases[c] || c;
  if (byCode.has(mapped as TemplateLanguageCode)) {
    return mapped as TemplateLanguageCode;
  }
  return "";
}

export function isTemplateLanguageCode(code: string): code is TemplateLanguageCode {
  return resolveCode(code) !== "";
}

export function templateLanguageLabel(code: string | null | undefined): string {
  const resolved = resolveCode(code || "");
  if (!resolved) {
    const c = (code || "").trim().toLowerCase();
    return c ? c.toUpperCase() : "";
  }
  return byCode.get(resolved) || resolved.toUpperCase();
}

/** Detect a reasonable default from the browser locale (e.g. de-DE → de). */
export function guessTemplateLanguage(): TemplateLanguageCode {
  if (typeof navigator === "undefined") return "en";
  const raw = (navigator.language || "en").trim().toLowerCase();
  const primary = raw.split("-")[0] || "en";
  return resolveCode(primary) || "en";
}
