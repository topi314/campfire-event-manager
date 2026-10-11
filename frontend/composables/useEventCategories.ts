import {
  TEMPLATE_CATEGORY_OPTIONS_FALLBACK,
  type EventCategoryCatalogEntry,
  categoryFallbacks,
  categoryMatchOrder,
  isFixedLocalHourCategory,
  isPresetCategory,
  isTemplateCategoryOption,
} from "~/utils/eventCategory";

/** Built-in live-event categories from `/api/config` (backend `eventcategory.Ordered`). */
export function useEventCategories() {
  const { eventCategories } = useClientConfig();

  const catalog = computed<EventCategoryCatalogEntry[]>(() =>
    eventCategories.value.length
      ? eventCategories.value
      : TEMPLATE_CATEGORY_OPTIONS_FALLBACK.map((name) => ({
          name,
          fixedLocalHour: isFixedLocalHourCategory(name),
          fallbacks: categoryFallbacks(name),
        })),
  );

  const options = computed(() => catalog.value.map((c) => c.name));

  function isOption(name: string) {
    return isTemplateCategoryOption(name, options.value);
  }

  function isPreset(name: string) {
    return isPresetCategory(name, options.value);
  }

  function fallbacks(category: string) {
    return categoryFallbacks(category, catalog.value);
  }

  function matchOrder(liveCategory: string) {
    return categoryMatchOrder(liveCategory, catalog.value);
  }

  function fixedLocalHour(category: string) {
    return isFixedLocalHourCategory(category, catalog.value);
  }

  return {
    catalog,
    options,
    isOption,
    isPreset,
    fallbacks,
    matchOrder,
    fixedLocalHour,
  };
}
