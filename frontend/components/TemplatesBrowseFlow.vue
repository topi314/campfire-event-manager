<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate } from "~/types";
import { TEMPLATE_CATEGORY_OPTIONS } from "~/utils/eventCategory";
import { TEMPLATE_LANGUAGES, templateLanguageLabel } from "~/utils/languages";
import { formatInstantDate } from "~/utils/datetime";

const props = defineProps<{
  /** Open this published template in the preview when present. */
  highlightId?: number | null;
}>();

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { timeZone } = usePreferences();
const route = useRoute();

const SORT_OPTIONS = ["published_desc", "updated_desc", "name_asc", "likes_desc"] as const;
type SortOption = (typeof SORT_OPTIONS)[number];

const templates = ref<MeetupTemplate[]>([]);
const error = ref("");
const success = ref("");
const loading = ref(false);
const busyId = ref<number | null>(null);
const likeBusyId = ref<number | null>(null);

const q = ref("");
const category = ref("");
const language = ref("");
const creator = ref("");
const sort = ref<SortOption>("published_desc");
const preview = ref<MeetupTemplate | null>(null);

let ignoreNextQueryWatch = false;
let applyingFromRoute = false;
let textFilterTimer: ReturnType<typeof setTimeout> | null = null;

function queryStr(v: unknown): string {
  if (typeof v === "string") return v;
  if (Array.isArray(v) && typeof v[0] === "string") return v[0];
  return "";
}

function readFiltersFromRoute() {
  applyingFromRoute = true;
  q.value = queryStr(route.query.q);
  category.value = queryStr(route.query.category);
  language.value = queryStr(route.query.language);
  creator.value = queryStr(route.query.creator);
  const s = queryStr(route.query.sort);
  sort.value = (SORT_OPTIONS as readonly string[]).includes(s)
    ? (s as SortOption)
    : "published_desc";
  nextTick(() => {
    applyingFromRoute = false;
  });
}

// Seed from the URL before watchers attach so the initial read doesn't push.
readFiltersFromRoute();

function filtersQuery(): Record<string, string> {
  const out: Record<string, string> = {};
  if (q.value.trim()) out.q = q.value.trim();
  if (creator.value.trim()) out.creator = creator.value.trim();
  if (category.value) out.category = category.value;
  if (language.value) out.language = language.value;
  if (sort.value && sort.value !== "published_desc") out.sort = sort.value;
  return out;
}

function queriesEqual(
  a: Record<string, unknown>,
  b: Record<string, string>,
): boolean {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  for (const key of keys) {
    if (queryStr(a[key]) !== (b[key] || "")) return false;
  }
  return true;
}

function browseListPath() {
  return "/templates/browse";
}

function browseTemplatePath(id: number) {
  return `/templates/browse/${id}`;
}

async function pushFiltersToRoute(opts?: { path?: string; replace?: boolean }) {
  const query = filtersQuery();
  const path = opts?.path || route.path;
  if (path === route.path && queriesEqual(route.query as Record<string, unknown>, query)) {
    await load();
    return;
  }
  ignoreNextQueryWatch = true;
  try {
    await navigateTo(
      { path, query },
      { replace: opts?.replace !== false },
    );
    await load();
  } finally {
    ignoreNextQueryWatch = false;
  }
}

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  readFiltersFromRoute();
  await load();
  openHighlight();
});

watch(
  () => props.highlightId,
  () => openHighlight(),
);

watch(
  () => route.query,
  async () => {
    if (ignoreNextQueryWatch) return;
    readFiltersFromRoute();
    await load();
    openHighlight();
  },
);

watch([category, language, sort], () => {
  if (applyingFromRoute) return;
  void pushFiltersToRoute();
});

watch([q, creator], () => {
  if (applyingFromRoute) return;
  if (textFilterTimer) clearTimeout(textFilterTimer);
  textFilterTimer = setTimeout(() => {
    textFilterTimer = null;
    void pushFiltersToRoute();
  }, 300);
});

onBeforeUnmount(() => {
  if (textFilterTimer) clearTimeout(textFilterTimer);
});

function flushTextFilters() {
  if (textFilterTimer) {
    clearTimeout(textFilterTimer);
    textFilterTimer = null;
  }
  void pushFiltersToRoute();
}

function openHighlight() {
  const id = props.highlightId;
  if (id == null || id <= 0) return;
  const t = templates.value.find((x) => x.id === id);
  if (t) preview.value = t;
}

function openPreview(t: MeetupTemplate) {
  preview.value = t;
  void pushFiltersToRoute({ path: browseTemplatePath(t.id), replace: true });
}

function closePreview() {
  preview.value = null;
  void pushFiltersToRoute({ path: browseListPath(), replace: true });
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (q.value.trim()) params.set("q", q.value.trim());
    if (category.value) params.set("category", category.value);
    if (language.value) params.set("language", language.value);
    if (creator.value.trim()) params.set("creator", creator.value.trim());
    if (sort.value) params.set("sort", sort.value);
    const qs = params.toString();
    templates.value = await api<MeetupTemplate[]>(
      `/api/templates/shared${qs ? `?${qs}` : ""}`,
    );
    openHighlight();
  } catch (e: any) {
    error.value = e.message || "Failed to load templates";
    templates.value = [];
  } finally {
    loading.value = false;
  }
}

function onSearchSubmit() {
  flushTextFilters();
}

function parsePayload(t: MeetupTemplate): MeetupPayload {
  return (typeof t.payload === "string" ? JSON.parse(t.payload) : t.payload) as MeetupPayload;
}

function categoryOf(t: MeetupTemplate) {
  return (parsePayload(t).category || "").trim();
}

function coverOf(t: MeetupTemplate) {
  return (parsePayload(t).coverPhotoUrl || "").trim();
}

async function addToMine(t: MeetupTemplate) {
  busyId.value = t.id;
  error.value = "";
  success.value = "";
  try {
    await api(`/api/templates/${t.id}/clone`, { method: "POST" });
    success.value = `Added “${t.name}” to My templates — it stays synced with the original until you edit it`;
    closePreview();
  } catch (e: any) {
    error.value = e.message || "Failed to add template";
  } finally {
    busyId.value = null;
  }
}

type LikeState = {
  likeCount: number;
  likedByMe: boolean;
  likers?: MeetupTemplate["likers"];
};

function applyLikeState(id: number, state: LikeState) {
  const patch = {
    likeCount: state.likeCount,
    likedByMe: state.likedByMe,
    likers: state.likers || [],
  };
  const idx = templates.value.findIndex((x) => x.id === id);
  if (idx >= 0) {
    templates.value[idx] = { ...templates.value[idx], ...patch };
  }
  if (preview.value?.id === id) {
    preview.value = { ...preview.value, ...patch };
  }
}

async function toggleLike(t: MeetupTemplate) {
  likeBusyId.value = t.id;
  error.value = "";
  try {
    const liked = !!t.likedByMe;
    const state = await api<LikeState>(`/api/templates/${t.id}/like`, {
      method: liked ? "DELETE" : "POST",
    });
    applyLikeState(t.id, state);
  } catch (e: any) {
    error.value = e.message || "Failed to update like";
  } finally {
    likeBusyId.value = null;
  }
}

function isHighlight(t: MeetupTemplate) {
  return props.highlightId != null && t.id === props.highlightId;
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel">
      <div class="page-head">
        <h2>Browse templates</h2>
        <p class="muted">
          Published templates from other Community Ambassadors. Add a copy to
          <NuxtLink to="/templates">My templates</NuxtLink>
          — it stays synced with the original until you edit it. Location, invites,
          and comments are not included. Filters stay in the URL so you can share them.
        </p>
      </div>

      <form class="filters" @submit.prevent="onSearchSubmit">
        <div class="field" style="margin-bottom: 0">
          <label for="browse-q">Search</label>
          <input id="browse-q" v-model="q" type="search" placeholder="Template or listing…" />
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="browse-creator">Creator</label>
          <input
            id="browse-creator"
            v-model="creator"
            type="search"
            placeholder="Name or username…"
          />
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="browse-cat">Category</label>
          <select id="browse-cat" v-model="category">
            <option value="">Any</option>
            <option v-for="c in TEMPLATE_CATEGORY_OPTIONS" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="browse-lang">Language</label>
          <select id="browse-lang" v-model="language">
            <option value="">Any</option>
            <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
              {{ l.label }}
            </option>
          </select>
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="browse-sort">Sort</label>
          <select id="browse-sort" v-model="sort">
            <option value="published_desc">Newest published</option>
            <option value="updated_desc">Recently updated</option>
            <option value="likes_desc">Most liked</option>
            <option value="name_asc">Name A–Z</option>
          </select>
        </div>
      </form>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="success" class="success">{{ success }}</p>
      <p v-if="loading" class="muted">Loading…</p>

      <ul v-else-if="templates.length" class="browse-list">
        <li
          v-for="t in templates"
          :id="`tpl-${t.id}`"
          :key="t.id"
          :class="{ highlight: isHighlight(t) }"
          role="button"
          tabindex="0"
          @click="openPreview(t)"
          @keydown.enter.prevent="openPreview(t)"
        >
          <div class="browse-cover" aria-hidden="true">
            <img v-if="coverOf(t)" :src="coverOf(t)" alt="" />
            <span v-else class="cover-fallback">{{ (t.name || "?").slice(0, 1).toUpperCase() }}</span>
          </div>
          <div class="browse-body">
            <div class="browse-title-row">
              <strong class="browse-name">{{ t.name }}</strong>
              <span v-if="categoryOf(t)" class="cat-badge">{{ categoryOf(t) }}</span>
              <span v-if="t.language" class="cat-badge">{{ templateLanguageLabel(t.language) }}</span>
            </div>
            <p v-if="t.publishDescription" class="listing-desc muted">
              {{ t.publishDescription }}
            </p>
            <div class="browse-meta muted">
              <div v-if="t.publisher" class="publisher">
                <span>by</span>
                <NuxtLink :to="`/profiles/${t.publisher.id}`" @click.stop>
                  <img
                    v-if="t.publisher.avatarUrl"
                    :src="t.publisher.avatarUrl"
                    alt=""
                    class="avatar"
                  />
                  <span>{{ t.publisher.displayName || t.publisher.username }}</span>
                </NuxtLink>
              </div>
              <span v-if="t.publishedAt" class="meta-sep">
                {{ formatInstantDate(t.publishedAt, timeZone) }}
              </span>
            </div>
          </div>
          <div class="row-actions" @click.stop>
            <TemplateLikes
              compact
              :show-likers="false"
              :template="t"
              :creator-id="t.publisher?.id || t.discordUserId"
              :busy="likeBusyId === t.id"
              @toggle="toggleLike(t)"
            />
            <button
              type="button"
              class="icon-btn icon-btn-accent"
              title="Add to my templates"
              aria-label="Add to my templates"
              :disabled="busyId === t.id"
              @click="addToMine(t)"
            >
              <Icon name="add" />
            </button>
          </div>
        </li>
      </ul>
      <p v-else-if="!loading" class="muted">No published templates match these filters.</p>

      <TemplatePreview
        v-if="preview"
        :template="preview"
        :busy="busyId === preview.id"
        :like-busy="likeBusyId === preview.id"
        @close="closePreview"
        @add="addToMine(preview)"
        @toggle-like="toggleLike(preview)"
      />
    </div>
  </AppShell>
</template>

<style scoped>
.page-head h2 {
  margin: 0 0 0.35rem;
  font-size: 1.15rem;
}
.page-head .muted {
  margin: 0;
}
.filters {
  display: grid;
  grid-template-columns: minmax(7rem, 1.3fr) minmax(6.5rem, 1fr) minmax(6.5rem, 0.9fr) minmax(6.5rem, 0.9fr) minmax(6.5rem, 0.9fr);
  gap: 0.65rem 0.75rem;
  margin: 1rem 0 1.15rem;
  align-items: end;
}
@media (max-width: 900px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
.browse-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.browse-list li {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 0.85rem;
  align-items: center;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: calc(var(--radius) + 2px);
  background: color-mix(in srgb, var(--bg-input) 55%, var(--bg-elevated));
  cursor: pointer;
  transition: border-color 0.12s ease, background 0.12s ease;
}
.browse-list li:hover {
  border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
  background: color-mix(in srgb, var(--bg-input) 80%, var(--bg-elevated));
}
.browse-list li.highlight {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 35%, transparent);
}
.browse-cover {
  width: 56px;
  height: 56px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  overflow: hidden;
  flex-shrink: 0;
  background: var(--bg);
  display: grid;
  place-items: center;
}
.browse-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.cover-fallback {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--muted);
  line-height: 1;
}
.browse-body {
  min-width: 0;
}
.browse-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.55rem;
}
.browse-name {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.25;
}
.cat-badge {
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  color: var(--muted);
  background: var(--bg);
}
.listing-desc {
  margin: 0.35rem 0 0;
  font-size: 0.88rem;
  line-height: 1.4;
  white-space: pre-wrap;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.browse-meta {
  margin-top: 0.4rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.75rem;
  font-size: 0.82rem;
}
.meta-sep::before {
  content: "·";
  margin-right: 0.75rem;
  opacity: 0.6;
}
.publisher {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  line-height: 1.2;
}
.publisher a {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 18px;
}
.avatar {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  object-fit: cover;
  flex-shrink: 0;
  display: block;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
  justify-content: flex-end;
}
@media (max-width: 560px) {
  .browse-list li {
    grid-template-columns: auto 1fr;
    grid-template-areas:
      "cover body"
      "actions actions";
  }
  .browse-cover {
    grid-area: cover;
  }
  .browse-body {
    grid-area: body;
  }
  .row-actions {
    grid-area: actions;
    justify-self: end;
  }
}
</style>
