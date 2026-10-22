/**
 * Template category helpers. Live-event name → category matching runs on the
 * backend only (`eventcategory.FromName`); live events arrive with `category`
 * already set. Built-in preset names come from `/api/config` → `eventCategories`
 * (see `useEventCategories`); the list below is a fallback until that loads.
 */

export const EVENT_CATEGORY_OTHER = "Other";
export const EVENT_CATEGORY_NO_EVENT = "No Event";

export type EventCategoryCatalogEntry = {
  name: string;
  fixedLocalHour: boolean;
  fallbacks: string[];
};

/** Fallback presets if `/api/config` has not loaded yet (keep in sync with backend Ordered). */
export const TEMPLATE_CATEGORY_OPTIONS_FALLBACK = [
  "GO Wild Area",
  "GO Fest",
  "GO Tour",
  "Community Day Classic",
  "Community Day",
  "Max Battle Day",
  "Research Day",
  "Catch Mastery",
  "Hatch Day",
  "Friendship Friday",
  "Super Mega Raid Day",
  "Shadow Raid Day",
  "Raid Day",
  "Raid Hour",
  "Max Monday",
  "Spotlight Hour",
] as const;

/** @deprecated Prefer `useEventCategories().options` (backend catalog). */
export const TEMPLATE_CATEGORY_OPTIONS = [...TEMPLATE_CATEGORY_OPTIONS_FALLBACK];

export type EventCategoryPreset = (typeof TEMPLATE_CATEGORY_OPTIONS_FALLBACK)[number];

export const CATEGORY_SELECT_CUSTOM = "__custom__";

export function isTemplateCategoryOption(
  name: string,
  options: readonly string[] = TEMPLATE_CATEGORY_OPTIONS_FALLBACK,
): boolean {
  return options.includes(name);
}

export function isPresetCategory(
  name: string,
  options: readonly string[] = TEMPLATE_CATEGORY_OPTIONS_FALLBACK,
): boolean {
  if (name === EVENT_CATEGORY_OTHER || name === EVENT_CATEGORY_NO_EVENT) return true;
  return options.includes(name);
}

/** Alternate categories when no template matches exactly. */
export function categoryFallbacks(
  category: string,
  catalog?: EventCategoryCatalogEntry[],
): string[] {
  const cat = category.trim();
  if (catalog?.length) {
    const hit = catalog.find((c) => c.name === cat);
    if (hit) return hit.fallbacks || [];
  }
  switch (cat) {
    case "Community Day Classic":
      return ["Community Day"];
    case "Community Day":
      return ["Community Day Classic"];
    case "Super Mega Raid Day":
    case "Shadow Raid Day":
      return ["Raid Day"];
    case "Catch Mastery":
      return ["Research Day"];
    default:
      return [];
  }
}

/** Exact category first, then fallbacks. */
export function categoryMatchOrder(
  liveCategory: string,
  catalog?: EventCategoryCatalogEntry[],
): string[] {
  const cat = liveCategory.trim();
  if (!cat) return [];
  return [cat, ...categoryFallbacks(cat, catalog)];
}

/** Raid Hour / Spotlight / Max Monday are always 18:00–19:00 local. */
export function isFixedLocalHourCategory(
  category: string,
  catalog?: EventCategoryCatalogEntry[],
): boolean {
  const cat = category.trim();
  if (catalog?.length) {
    const hit = catalog.find((c) => c.name === cat);
    if (hit) return hit.fixedLocalHour;
  }
  switch (cat) {
    case "Raid Hour":
    case "Spotlight Hour":
    case "Max Monday":
      return true;
    default:
      return false;
  }
}

/** Preset names first, then extra (custom) categories not already listed. */
export function categoryFilterOptions(presets: readonly string[], extras: readonly string[]): string[] {
  const seen = new Set(presets.map((name) => name.trim().toLowerCase()));
  const custom: string[] = [];
  for (const raw of extras) {
    const name = raw.trim();
    if (!name) continue;
    const key = name.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    custom.push(name);
  }
  custom.sort((a, b) => a.localeCompare(b));
  return [...presets, ...custom];
}
