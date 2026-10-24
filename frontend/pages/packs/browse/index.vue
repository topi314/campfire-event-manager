<script setup lang="ts">
import { coverImageSrc } from "~/utils/covers";
import { sortLibraries, type LibrarySort } from "~/utils/iconLibraries";

type LibraryItem = {
  id: number;
  ownerId: string;
  ownerName?: string;
  ownerUsername?: string;
  name: string;
  description?: string;
  isDefault: boolean;
  published: boolean;
  mine: boolean;
  saved?: boolean;
  previews?: string[];
  createdAt?: string;
  updatedAt?: string;
  publishedAt?: string | null;
};

type IconItem = {
  id: number;
  libraryId: number;
  libraryName: string;
  filename: string;
  name: string;
  category: string;
  pokemon: string[];
  url: string;
  ownerId?: string;
  libraryPublished?: boolean;
  createdAt?: string;
};

type IconSort = "newest" | "name_asc" | "library_asc";

const route = useRoute();
const router = useRouter();
const { api } = useApi();
const { apiBase } = useRuntimeConfig().public;
const { user, loaded, ensureAuth } = useAuth();
const { options: categoryOptions } = useEventCategories();
const toast = useToast();
const { ensure, nameOf } = usePokemonList();

const view = computed<"libraries" | "icons">(() =>
  route.query.view === "icons" ? "icons" : "libraries",
);

const catalog = ref<LibraryItem[]>([]);
const libraries = ref<LibraryItem[]>([]);
const icons = ref<IconItem[]>([]);
const error = ref("");
const iconsLoading = ref(false);
const busyId = ref<number | null>(null);

const filterQ = ref("");
const filterCreator = ref("");
const sort = ref<LibrarySort>("published_desc");

const iconQ = ref("");
const iconCategory = ref("");
const iconLibrary = ref("");
const iconSort = ref<IconSort>("newest");
const iconPokemon = ref("");

const creators = computed(() => {
  const map = new Map<string, string>();
  for (const lib of catalog.value) {
    if (!lib.published) continue;
    const label = lib.mine ? "You" : lib.ownerName || lib.ownerUsername || "Unknown";
    map.set(lib.ownerId, label);
  }
  return [...map.entries()]
    .map(([id, label]) => ({ id, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
});

const publishedLibraries = computed(() => {
  const list = libraries.value.filter((lib) => lib.published);
  return sortLibraries(list, sort.value);
});

const libraryChoices = computed(() =>
  catalog.value
    .filter((lib) => lib.published)
    .sort((a, b) => a.name.localeCompare(b.name)),
);

const visibleIcons = computed(() => {
  const list = icons.value.filter((icon) => icon.libraryPublished);
  return [...list].sort((a, b) => {
    if (iconSort.value === "name_asc") {
      return (a.name || a.filename).localeCompare(b.name || b.filename) || b.id - a.id;
    }
    if (iconSort.value === "library_asc") {
      return a.libraryName.localeCompare(b.libraryName) || (a.name || a.filename).localeCompare(b.name || b.filename);
    }
    return (b.createdAt || "").localeCompare(a.createdAt || "") || b.id - a.id;
  });
});

function setView(next: "libraries" | "icons") {
  const query = { ...route.query };
  if (next === "icons") query.view = "icons";
  else delete query.view;
  void router.replace({ query });
}

onMounted(async () => {
  if (!(await ensureAuth())) return;
  void ensure();
  await Promise.all([loadCatalog(), loadLibraries()]);
  if (view.value === "icons") await loadIcons();
});

let packTimer: ReturnType<typeof setTimeout> | null = null;
watch(filterQ, () => {
  if (view.value !== "libraries") return;
  if (packTimer) clearTimeout(packTimer);
  packTimer = setTimeout(() => {
    void loadLibraries();
  }, 300);
});
watch([filterCreator], () => {
  if (view.value !== "libraries") return;
  void loadLibraries();
});

watch(view, (next) => {
  if (next === "icons" && !icons.value.length && !iconsLoading.value) void loadIcons();
});

async function loadCatalog() {
  const res = await api<{ libraries: LibraryItem[] }>("/api/icon-libraries");
  catalog.value = res.libraries || [];
}

async function loadLibraries() {
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (filterQ.value.trim()) params.set("q", filterQ.value.trim());
    if (filterCreator.value) params.set("owner", filterCreator.value);
    const qs = params.toString();
    const res = await api<{ libraries: LibraryItem[] }>(
      `/api/icon-libraries${qs ? `?${qs}` : ""}`,
    );
    libraries.value = res.libraries || [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to load packs";
  }
}

function markSaved(id: number, saved: boolean) {
  for (const list of [libraries.value, catalog.value]) {
    const row = list.find((lib) => lib.id === id);
    if (row) row.saved = saved;
  }
}

async function addPack(lib: LibraryItem) {
  busyId.value = lib.id;
  error.value = "";
  try {
    await api(`/api/icon-libraries/${lib.id}/save`, { method: "POST" });
    markSaved(lib.id, true);
    toast.success("Added to My packs", lib.name);
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to add pack";
  } finally {
    busyId.value = null;
  }
}

let iconTimer: ReturnType<typeof setTimeout> | null = null;
watch([iconQ, iconCategory, iconLibrary, iconPokemon], () => {
  if (view.value !== "icons") return;
  if (iconTimer) clearTimeout(iconTimer);
  iconTimer = setTimeout(() => {
    void loadIcons();
  }, 250);
});

async function loadIcons() {
  iconsLoading.value = true;
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (iconQ.value.trim()) params.set("q", iconQ.value.trim());
    if (iconCategory.value) params.set("category", iconCategory.value);
    if (iconLibrary.value) params.set("libraryId", iconLibrary.value);
    if (iconPokemon.value) params.set("pokemon", iconPokemon.value);
    const res = await api<{ icons: IconItem[] }>(`/api/icons?${params}`);
    icons.value = res.icons || [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : "Failed to load icons";
    icons.value = [];
  } finally {
    iconsLoading.value = false;
  }
}

function src(url: string) {
  return coverImageSrc(url, apiBase as string);
}
</script>

<template>
  <AppShell>
    <div v-if="!loaded" class="panel">
      <p class="muted">Loading…</p>
    </div>
    <div v-else-if="user" class="panel">
      <div class="page-head">
        <div>
          <h2>Browse packs</h2>
          <p class="muted">
            Published packs you can add to
            <NuxtLink to="/packs">My packs</NuxtLink>.
            An added pack stays read-only and follows the original.
          </p>
        </div>
        <div class="view-switch" role="tablist" aria-label="Browse view">
          <button
            type="button"
            role="tab"
            :aria-selected="view === 'libraries'"
            :class="{ active: view === 'libraries' }"
            @click="setView('libraries')"
          >
            Packs
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="view === 'icons'"
            :class="{ active: view === 'icons' }"
            @click="setView('icons')"
          >
            Icons
          </button>
        </div>
      </div>

      <p v-if="error" class="error">{{ error }}</p>

      <template v-if="view === 'libraries'">
        <div class="filters">
          <div class="field" style="margin-bottom: 0">
            <label for="browse-pack-q">Search</label>
            <input id="browse-pack-q" v-model="filterQ" type="search" placeholder="Pack, description, or creator…" />
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="browse-pack-creator">Creator</label>
            <select id="browse-pack-creator" v-model="filterCreator">
              <option value="">Any</option>
              <option v-for="creator in creators" :key="creator.id" :value="creator.id">
                {{ creator.label }}
              </option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="browse-pack-sort">Sort</label>
            <select id="browse-pack-sort" v-model="sort">
              <option value="published_desc">Newest published</option>
              <option value="updated_desc">Recently updated</option>
              <option value="created_desc">Newest created</option>
              <option value="name_asc">Name A–Z</option>
            </select>
          </div>
        </div>

        <div v-if="publishedLibraries.length" class="lib-grid">
          <IconLibraryCard
            v-for="lib in publishedLibraries"
            :key="lib.id"
            :name="lib.name"
            :description="lib.description"
            :detail="lib.mine ? '' : lib.ownerName"
            :to="`/packs/${lib.id}`"
            :previews="lib.previews"
            :badge="lib.mine ? 'Yours' : lib.saved ? 'Added' : 'Published'"
          >
            <template v-if="!lib.mine && !lib.saved" #action>
              <button
                type="button"
                class="icon-btn icon-btn-accent"
                title="Add to my packs"
                aria-label="Add to my packs"
                :disabled="busyId === lib.id"
                @click.stop.prevent="addPack(lib)"
              >
                <Icon name="add" />
              </button>
            </template>
          </IconLibraryCard>
        </div>
        <p v-else class="muted">No published packs match these filters.</p>
      </template>

      <template v-else>
        <div class="filters icon-filters">
          <div class="field" style="margin-bottom: 0">
            <label for="icons-q">Search</label>
            <input id="icons-q" v-model="iconQ" type="search" placeholder="Name or pack…" />
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="icons-cat">Category</label>
            <select id="icons-cat" v-model="iconCategory">
              <option value="">Any</option>
              <option v-for="opt in categoryOptions" :key="opt" :value="opt">{{ opt }}</option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="icons-poke">Pokémon</label>
            <PokemonSelect id="icons-poke" v-model="iconPokemon" placeholder="Any" />
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="icons-lib">Pack</label>
            <select id="icons-lib" v-model="iconLibrary">
              <option value="">Any</option>
              <option v-for="lib in libraryChoices" :key="lib.id" :value="String(lib.id)">
                {{ lib.name }}
              </option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="icons-sort">Sort</label>
            <select id="icons-sort" v-model="iconSort">
              <option value="newest">Newest</option>
              <option value="name_asc">Name A–Z</option>
              <option value="library_asc">Pack A–Z</option>
            </select>
          </div>
        </div>

        <p v-if="iconsLoading" class="muted">Loading…</p>
        <div v-else-if="visibleIcons.length" class="icon-grid">
          <NuxtLink
            v-for="icon in visibleIcons"
            :key="icon.id"
            class="icon-card"
            :to="`/packs/${icon.libraryId}`"
          >
            <img :src="src(icon.url)" :alt="icon.name || icon.filename" />
            <strong>{{ icon.name || icon.filename }}</strong>
            <span class="muted">{{ icon.libraryName }}</span>
            <span v-if="icon.category" class="tag">{{ icon.category }}</span>
            <span v-if="icon.pokemon.length" class="tag">
              {{ icon.pokemon.map(nameOf).join(", ") }}
            </span>
          </NuxtLink>
        </div>
        <p v-else class="muted">No icons match these filters.</p>
      </template>
    </div>
  </AppShell>
</template>

<style scoped>
.page-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
  align-items: flex-start;
  margin-bottom: 1rem;
}
.page-head h2 {
  margin: 0 0 0.35rem;
  font-size: 1.15rem;
}
.page-head .muted {
  margin: 0;
}
.view-switch {
  display: flex;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}
.view-switch button {
  border: 0;
  border-radius: 0;
  background: transparent;
}
.view-switch button.active {
  background: var(--accent, #3b82f6);
  color: #fff;
}
.filters {
  display: grid;
  grid-template-columns: minmax(10rem, 1.4fr) minmax(8rem, 1fr) minmax(10rem, 1fr);
  gap: 0.65rem 0.75rem;
  margin: 0 0 1rem;
  align-items: end;
}
@media (max-width: 980px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
.icon-filters {
  grid-template-columns: minmax(8rem, 1.2fr) minmax(7rem, 0.9fr) minmax(8rem, 1fr) minmax(8rem, 0.9fr) minmax(7rem, 0.8fr);
}
.lib-grid,
.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 0.75rem;
}
.icon-grid {
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
}
.icon-card {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  text-decoration: none;
  color: inherit;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.55rem;
  background: var(--bg);
}
.icon-card:hover {
  border-color: var(--accent, #7aa2ff);
}
.icon-card img {
  width: 100%;
  height: 96px;
  object-fit: cover;
  border-radius: 4px;
}
.icon-card strong,
.icon-card .muted,
.icon-card .tag {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tag {
  font-size: 0.75rem;
  color: var(--muted);
}
.hits {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-top: 0.35rem;
}
.linkish {
  margin-top: 0.3rem;
  padding: 0;
  border: 0;
  background: none;
  color: var(--accent, #7aa2ff);
  cursor: pointer;
}
</style>
