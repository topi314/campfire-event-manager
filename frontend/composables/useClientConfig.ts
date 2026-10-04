export function useClientConfig() {
  const config = useRuntimeConfig();
  const cartoApiKey = useState("cartoApiKey", () => "");
  const bugHexeUserIds = useState<string[]>("bugHexeUserIds", () => []);
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
      };
      cartoApiKey.value = (data.cartoApiKey || "").trim();
      bugHexeUserIds.value = Array.isArray(data.bugHexeUserIds)
        ? data.bugHexeUserIds.map((id) => String(id).trim()).filter(Boolean)
        : [];
    } catch {
      /* backend unreachable — optional client features stay off */
    }
  });

  return { cartoApiKey, bugHexeUserIds };
}
