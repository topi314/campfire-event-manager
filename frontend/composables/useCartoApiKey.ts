export function useCartoApiKey() {
  const config = useRuntimeConfig();
  const apiKey = useState("cartoApiKey", () => "");
  const requested = useState("cartoApiKeyRequested", () => false);

  onMounted(async () => {
    if (requested.value) return;
    requested.value = true;
    try {
      const res = await fetch(`${config.public.apiBase}/api/config`, {
        headers: { Accept: "application/json" },
        credentials: "include",
      });
      if (!res.ok) return;
      const data = (await res.json()) as { cartoApiKey?: string };
      apiKey.value = (data.cartoApiKey || "").trim();
    } catch {
      /* backend unreachable — Carto tiles stay watermarked */
    }
  });

  return { apiKey };
}
