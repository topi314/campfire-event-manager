export function useCartoApiKey() {
  const { cartoApiKey: apiKey } = useClientConfig();
  return { apiKey };
}
