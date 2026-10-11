import type { EventCategoryCatalogEntry } from "~/utils/eventCategory";

export function useClientConfig() {
  const config = useRuntimeConfig();
  const cartoApiKey = useState("cartoApiKey", () => "");
  const bugHexeUserIds = useState<string[]>("bugHexeUserIds", () => []);
  const eventCategories = useState<EventCategoryCatalogEntry[]>("eventCategories", () => []);
  const requested = useState("clientConfigRequested", () => false);

  onMounted(async () => {
    if (requested.value) return;
    requested.value = true;
    try {
      const res = await fetch(`${config.public.apiBase}/api/config`, {
        headers: { Accept: "application/json" },
        credentials: "include",
      });
      if (!res.ok) return;
      const data = (await res.json()) as {
        cartoApiKey?: string;
        bugHexeUserIds?: string[] | null;
        eventCategories?: EventCategoryCatalogEntry[] | null;
      };
      cartoApiKey.value = (data.cartoApiKey || "").trim();
      bugHexeUserIds.value = Array.isArray(data.bugHexeUserIds)
        ? data.bugHexeUserIds.map((id) => String(id).trim()).filter(Boolean)
        : [];
      eventCategories.value = Array.isArray(data.eventCategories)
        ? data.eventCategories
            .filter((c) => c && typeof c.name === "string" && c.name.trim())
            .map((c) => ({
              name: String(c.name).trim(),
              fixedLocalHour: !!c.fixedLocalHour,
              fallbacks: Array.isArray(c.fallbacks)
                ? c.fallbacks.map((f) => String(f).trim()).filter(Boolean)
                : [],
            }))
        : [];
    } catch {
      /* backend unreachable — optional client features stay off */
    }
  });

  return { cartoApiKey, bugHexeUserIds, eventCategories };
}
