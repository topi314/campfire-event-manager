<script setup lang="ts">
import { coverImageSrc } from "~/utils/covers";

type IconItem = {
  id: number;
  libraryId: number;
  libraryName: string;
  filename: string;
  category: string;
  pokemon: string[];
  url: string;
  rank?: number;
};

type LibraryItem = {
  id: number;
  name: string;
  mine: boolean;
  saved?: boolean;
};

const props = defineProps<{
  modelValue: string;
  fallback: string;
  libraryId?: number | null;
  category?: string;
  eventName?: string;
  autoToken: number;
}>();

const emit = defineEmits<{
  "update:modelValue": [string];
}>();

const { api } = useApi();
const { apiBase } = useRuntimeConfig().public;

const matches = ref<IconItem[]>([]);
const browseOpen = ref(false);
const libraries = ref<LibraryItem[]>([]);
const browseIcons = ref<IconItem[]>([]);
const filterLibrary = ref("");
const filterCategory = ref("");
const filterPokemon = ref("");
const filterQuery = ref("");
const browseError = ref("");
const loadingBrowse = ref(false);

const selectedUrl = computed(() => props.modelValue || "");

function src(url: string) {
  return coverImageSrc(url, apiBase as string);
}

watch(
  () => props.autoToken,
  async (token, prev) => {
    if (!token || token === prev) return;
    const libraryId = Number(props.libraryId || 0);
    if (!libraryId) {
      matches.value = [];
      if ((props.fallback || "") !== props.modelValue) {
        emit("update:modelValue", props.fallback || "");
      }
      return;
    }
    try {
      const params = new URLSearchParams({
        libraryId: String(libraryId),
        category: props.category || "",
        eventName: props.eventName || "",
      });
      const res = await api<{ icons: IconItem[] }>(`/api/icons/match?${params}`);
      matches.value = res.icons || [];
      const next = matches.value[0]?.url || props.fallback || "";
      if (next !== props.modelValue) emit("update:modelValue", next);
    } catch {
      matches.value = [];
    }
  },
);

function pick(url: string) {
  emit("update:modelValue", url);
}

async function openBrowse() {
  browseOpen.value = !browseOpen.value;
  if (!browseOpen.value) return;
  if (!libraries.value.length) {
    try {
      const res = await api<{ libraries: LibraryItem[] }>("/api/icon-libraries");
      libraries.value = (res.libraries || []).filter((lib) => lib.mine || lib.saved);
    } catch {
      libraries.value = [];
    }
  }
  await loadBrowse();
}

async function loadBrowse() {
  loadingBrowse.value = true;
  browseError.value = "";
  try {
    const params = new URLSearchParams();
    params.set("scope", "mine");
    if (filterLibrary.value) params.set("libraryId", filterLibrary.value);
    if (filterCategory.value.trim()) params.set("category", filterCategory.value.trim());
    if (filterPokemon.value.trim()) params.set("pokemon", filterPokemon.value.trim());
    if (filterQuery.value.trim()) params.set("q", filterQuery.value.trim());
    const res = await api<{ icons: IconItem[] }>(`/api/icons?${params}`);
    browseIcons.value = res.icons || [];
  } catch (e) {
    browseError.value = e instanceof Error ? e.message : "Failed to load icons";
    browseIcons.value = [];
  } finally {
    loadingBrowse.value = false;
  }
}
</script>

<template>
  <div class="icon-tools">
    <div v-if="matches.length" class="match-strip">
      <p class="hint muted">Matching icons</p>
      <div class="icon-row">
        <button
          v-for="icon in matches"
          :key="icon.id"
          type="button"
          class="icon-choice"
          :class="{ selected: selectedUrl === icon.url }"
          :title="icon.filename"
          @click="pick(icon.url)"
        >
          <img :src="src(icon.url)" :alt="icon.filename" />
        </button>
      </div>
    </div>

    <button type="button" class="btn" @click="openBrowse">
      {{ browseOpen ? "Hide packs" : "Browse icons" }}
    </button>

    <div v-if="browseOpen" class="browse">
      <div class="filters">
        <select v-model="filterLibrary" @change="loadBrowse">
          <option value="">My packs</option>
          <option v-for="lib in libraries" :key="lib.id" :value="String(lib.id)">
            {{ lib.name }}
          </option>
        </select>
        <input
          v-model="filterCategory"
          type="text"
          placeholder="Category"
          @change="loadBrowse"
        />
        <input
          v-model="filterPokemon"
          type="text"
          placeholder="Pokémon slug"
          @change="loadBrowse"
        />
        <input v-model="filterQuery" type="search" placeholder="Search" @change="loadBrowse" />
      </div>
      <p v-if="browseError" class="error">{{ browseError }}</p>
      <p v-else-if="loadingBrowse" class="hint muted">Loading…</p>
      <p v-else-if="!browseIcons.length" class="hint muted">No icons.</p>
      <div v-else class="icon-grid">
        <button
          v-for="icon in browseIcons"
          :key="icon.id"
          type="button"
          class="icon-card"
          :class="{ selected: selectedUrl === icon.url }"
          @click="pick(icon.url)"
        >
          <img :src="src(icon.url)" :alt="icon.filename" />
          <span class="icon-meta">{{ icon.libraryName }}</span>
          <span class="icon-meta">{{ icon.filename }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.icon-tools {
  margin-top: 0.75rem;
}
.icon-row,
.icon-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.icon-choice,
.icon-card {
  border: 1px solid var(--border, #ccc);
  background: transparent;
  padding: 0.25rem;
  cursor: pointer;
  border-radius: 6px;
}
.icon-choice img,
.icon-card img {
  width: 64px;
  height: 64px;
  object-fit: cover;
  display: block;
}
.icon-card {
  width: 96px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
}
.icon-meta {
  font-size: 0.7rem;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.selected {
  outline: 2px solid var(--accent, #3b82f6);
}
.filters {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0.6rem 0;
}
.filters input,
.filters select {
  min-width: 8rem;
}
.browse {
  margin-top: 0.5rem;
}
</style>
