/** Same categorization as campfire-tools/internal/eventcategory. */

export const EVENT_CATEGORY_OTHER = "Other";
export const EVENT_CATEGORY_NO_EVENT = "No Event";

export const EVENT_CATEGORY_PATTERNS: Record<string, string[]> = {
  "Raid Day": ["Raid Day", "Mega Raid"],
  "Raid Hour": ["Raid Hour"],
  "Max Monday": ["Max Monday"],
  "Research Day": ["Research Day"],
  "Hatch Day": ["Hatch Day"],
  "Community Day": ["Community Day"],
  "Community Day Classic": [
    "Community Day Classic",
    "Community Classic Day",
    "Community Classic",
  ],
  "Spotlight Hour": ["Spotlight Hour"],
  "Max Battle Day": ["Max Battle Day", "Max Battle Weekend", "Max Battle", "Max Weekend", "Gigantamax", "GMAX"],
  "GO Tour": ["GO Tour"],
  "GO Fest": ["GO Fest"],
  "GO Wild Area": ["GOWA", "GO Wild Area"],
  "Friendship Friday": ["Friendship Friday"],
};

/** Match priority (first hit wins). */
export const EVENT_CATEGORY_ORDERED = [
  "GO Wild Area",
  "GO Fest",
  "GO Tour",
  "Community Day Classic",
  "Community Day",
  "Max Battle Day",
  "Research Day",
  "Hatch Day",
  "Friendship Friday",
  "Raid Day",
  "Raid Hour",
  "Max Monday",
  "Spotlight Hour",
] as const;

export type EventCategoryPreset = (typeof EVENT_CATEGORY_ORDERED)[number];

/** Preset categories for template dropdown (excludes Other / No Event). */
export const TEMPLATE_CATEGORY_OPTIONS = [...EVENT_CATEGORY_ORDERED];

export const CATEGORY_SELECT_CUSTOM = "__custom__";

export function isTemplateCategoryOption(name: string): boolean {
  return (TEMPLATE_CATEGORY_OPTIONS as readonly string[]).includes(name);
}

/** Infer category from a Campfire live event name. */
export function categoryFromLiveEventName(eventName: string): string {
  const name = eventName.trim().toLowerCase();
  if (!name) return EVENT_CATEGORY_NO_EVENT;
  for (const category of EVENT_CATEGORY_ORDERED) {
    for (const pattern of EVENT_CATEGORY_PATTERNS[category] || []) {
      if (name.includes(pattern.toLowerCase())) return category;
    }
  }
  return EVENT_CATEGORY_OTHER;
}

export function isPresetCategory(name: string): boolean {
  if (name === EVENT_CATEGORY_OTHER || name === EVENT_CATEGORY_NO_EVENT) return true;
  return Object.prototype.hasOwnProperty.call(EVENT_CATEGORY_PATTERNS, name);
}

/**
 * Alternate categories to try when no template is tagged with `category`.
 * Community Day and Community Day Classic fall back to each other.
 */
export function categoryFallbacks(category: string): string[] {
  const cat = category.trim();
  if (cat === "Community Day Classic") return ["Community Day"];
  if (cat === "Community Day") return ["Community Day Classic"];
  return [];
}

/** Categories that count as a match for `liveCategory` (exact first, then fallbacks). */
export function categoryMatchOrder(liveCategory: string): string[] {
  const cat = liveCategory.trim();
  if (!cat) return [];
  return [cat, ...categoryFallbacks(cat)];
}
