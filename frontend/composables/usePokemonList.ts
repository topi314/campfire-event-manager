export type PokemonOption = { slug: string; name: string };

export function usePokemonList() {
  const species = useState<PokemonOption[]>("pokemonSpecies", () => []);
  const { api } = useApi();

  async function ensure() {
    if (species.value.length) return;
    const res = await api<{ pokemon: PokemonOption[] }>("/api/pokemon");
    species.value = res.pokemon || [];
  }

  function nameOf(slug: string) {
    return species.value.find((sp) => sp.slug === slug)?.name || slug.replace(/-/g, " ");
  }

  return { species, ensure, nameOf };
}
