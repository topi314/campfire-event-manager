<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate, TemplatePublisher } from "~/types";
import { TEMPLATE_CATEGORY_OPTIONS } from "~/utils/eventCategory";
import { TEMPLATE_LANGUAGES, templateLanguageLabel } from "~/utils/languages";
import { formatInstantDate } from "~/utils/datetime";

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { timeZone } = usePreferences();
const route = useRoute();

const profile = ref<TemplatePublisher | null>(null);
const templates = ref<MeetupTemplate[]>([]);
const error = ref("");
const success = ref("");
const loading = ref(false);
const busyId = ref<number | null>(null);
const likeBusyId = ref<number | null>(null);

const q = ref("");
const category = ref("");
const language = ref("");
const sort = ref("published_desc");
const view = ref<"published" | "liked">("published");
const preview = ref<MeetupTemplate | null>(null);

const userId = computed(() => String(route.params.id || ""));
const isOwnProfile = computed(() => !!user.value && user.value.id === userId.value);

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  readViewFromRoute();
  await load();
});

watch(
  () => route.query.view,
  () => {
    readViewFromRoute();
  },
);

watch([category, language, sort, userId, view], () => {
  if (userId.value) load();
});

function readViewFromRoute() {
  const v = typeof route.query.view === "string" ? route.query.view : "";
  view.value = v === "liked" ? "liked" : "published";
  if (view.value === "liked" && sort.value === "published_desc") {
    sort.value = "liked_desc";
  } else if (view.value === "published" && sort.value === "liked_desc") {
    sort.value = "published_desc";
  }
}

async function setView(next: "published" | "liked") {
  if (view.value === next) return;
  view.value = next;
  if (next === "liked") sort.value = "liked_desc";
  else if (sort.value === "liked_desc") sort.value = "published_desc";
  closePreview();
  await navigateTo(
    {
      path: route.path,
      query: next === "liked" ? { view: "liked" } : {},
    },
    { replace: true },
  );
}

function openPreview(t: MeetupTemplate) {
  preview.value = {
    ...t,
    publisher: t.publisher || (view.value === "published" ? profile.value || undefined : t.publisher),
  };
}

function closePreview() {
  preview.value = null;
}

async function load() {
  if (!userId.value) return;
  loading.value = true;
  error.value = "";
  try {
    const params = new URLSearchParams();
    if (view.value === "liked") params.set("view", "liked");
    if (q.value.trim()) params.set("q", q.value.trim());
    if (category.value) params.set("category", category.value);
    if (language.value) params.set("language", language.value);
    if (sort.value) params.set("sort", sort.value);
    const qs = params.toString();
    const data = await api<{ user: TemplatePublisher; templates: MeetupTemplate[] }>(
      `/api/profiles/${encodeURIComponent(userId.value)}${qs ? `?${qs}` : ""}`,
    );
    profile.value = data.user;
    templates.value = data.templates || [];
  } catch (e: any) {
    error.value = e.message || "Failed to load profile";
    profile.value = null;
    templates.value = [];
  } finally {
    loading.value = false;
  }
}

function onSearchSubmit() {
  load();
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
    if (view.value === "liked" && isOwnProfile.value && !state.likedByMe) {
      templates.value = templates.value.filter((x) => x.id !== t.id);
      if (preview.value?.id === t.id) closePreview();
    }
  } catch (e: any) {
    error.value = e.message || "Failed to update like";
  } finally {
    likeBusyId.value = null;
  }
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel">
      <p class="muted" style="margin-top: 0">
        <NuxtLink to="/templates/browse">← Browse templates</NuxtLink>
      </p>

      <div v-if="profile" class="profile-header">
        <img
          v-if="profile.avatarUrl"
          :src="profile.avatarUrl"
          alt=""
          class="profile-avatar"
          width="64"
          height="64"
        />
        <div>
          <h2 style="margin: 0; font-size: 1.2rem">
            {{ profile.displayName || profile.username }}
          </h2>
          <p class="muted" style="margin: 0.2rem 0 0">@{{ profile.username }}</p>
        </div>
      </div>

      <p class="muted">
        <template v-if="view === 'liked'">
          Templates {{ isOwnProfile ? "you’ve" : "this Community Ambassador has" }} liked.
        </template>
        <template v-else>
          Published meetup templates from this Community Ambassador.
        </template>
      </p>

      <div class="view-tabs" role="tablist" aria-label="Profile templates">
        <button
          type="button"
          role="tab"
          :aria-selected="view === 'published'"
          :class="{ active: view === 'published' }"
          @click="setView('published')"
        >
          Published
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="view === 'liked'"
          :class="{ active: view === 'liked' }"
          @click="setView('liked')"
        >
          Liked
        </button>
      </div>

      <form class="filters" @submit.prevent="onSearchSubmit">
        <div class="field" style="margin-bottom: 0">
          <label for="profile-q">Search</label>
          <input id="profile-q" v-model="q" type="search" placeholder="Template name…" />
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="profile-cat">Category</label>
          <select id="profile-cat" v-model="category">
            <option value="">Any</option>
            <option v-for="c in TEMPLATE_CATEGORY_OPTIONS" :key="c" :value="c">{{ c }}</option>
          </select>
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="profile-lang">Language</label>
          <select id="profile-lang" v-model="language">
            <option value="">Any</option>
            <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
              {{ l.label }}
            </option>
          </select>
        </div>
        <div class="field" style="margin-bottom: 0">
          <label for="profile-sort">Sort</label>
          <select id="profile-sort" v-model="sort">
            <option v-if="view === 'liked'" value="liked_desc">Recently liked</option>
            <option value="published_desc">Newest published</option>
            <option value="updated_desc">Recently updated</option>
            <option value="likes_desc">Most liked</option>
            <option value="name_asc">Name A–Z</option>
          </select>
        </div>
        <button type="submit" class="primary" style="align-self: end">Search</button>
      </form>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="success" class="success">{{ success }}</p>
      <p v-if="loading" class="muted">Loading…</p>

      <ul v-else-if="templates.length" class="shared-list">
        <li
          v-for="t in templates"
          :key="t.id"
          role="button"
          tabindex="0"
          @click="openPreview(t)"
          @keydown.enter.prevent="openPreview(t)"
        >
          <div class="shared-cover" aria-hidden="true">
            <img v-if="coverOf(t)" :src="coverOf(t)" alt="" />
            <span v-else class="cover-fallback">{{ (t.name || "?").slice(0, 1).toUpperCase() }}</span>
          </div>
          <div class="shared-body">
            <div class="shared-title-row">
              <strong class="shared-name">{{ t.name }}</strong>
              <span v-if="categoryOf(t)" class="cat-badge">{{ categoryOf(t) }}</span>
              <span v-if="t.language" class="cat-badge">{{ templateLanguageLabel(t.language) }}</span>
            </div>
            <p v-if="t.publishDescription" class="listing-desc muted">
              {{ t.publishDescription }}
            </p>
            <div class="shared-meta muted">
              <div v-if="view === 'liked' && t.publisher" class="publisher">
                <span>by</span>
                <NuxtLink :to="`/profiles/${t.publisher.id}`" @click.stop>
                  <img
                    v-if="t.publisher.avatarUrl"
                    :src="t.publisher.avatarUrl"
                    alt=""
                    class="meta-avatar"
                  />
                  <span>{{ t.publisher.displayName || t.publisher.username }}</span>
                </NuxtLink>
              </div>
              <span v-if="t.publishedAt" :class="{ 'meta-sep': view === 'liked' && t.publisher }">
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
      <p v-else-if="!loading && profile" class="muted">
        <template v-if="view === 'liked'">
          {{ isOwnProfile ? "You haven’t liked any templates yet." : "No liked templates yet." }}
        </template>
        <template v-else>
          No published templates from this user yet.
        </template>
      </p>

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
.profile-header {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  margin-bottom: 0.75rem;
}
.profile-avatar {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  border: 1px solid var(--border);
  object-fit: cover;
}
.view-tabs {
  display: inline-flex;
  gap: 0.25rem;
  margin: 0.75rem 0 0;
  padding: 0.2rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-input);
}
.view-tabs button {
  border: none;
  background: transparent;
  color: var(--muted);
  padding: 0.35rem 0.75rem;
  border-radius: calc(var(--radius) - 2px);
  font: inherit;
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
}
.view-tabs button.active {
  background: var(--bg-elevated);
  color: var(--text);
}
.view-tabs button:hover:not(.active) {
  color: var(--text);
}
.filters {
  display: grid;
  grid-template-columns: minmax(8rem, 1.4fr) minmax(7rem, 1fr) minmax(7rem, 1fr) minmax(7rem, 1fr) auto;
  gap: 0.65rem 0.75rem;
  margin: 0.85rem 0 1.15rem;
  align-items: end;
}
@media (max-width: 700px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
.shared-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.shared-list li {
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
.shared-list li:hover {
  border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
  background: color-mix(in srgb, var(--bg-input) 80%, var(--bg-elevated));
}
.shared-cover {
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
.shared-cover img {
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
.shared-body {
  min-width: 0;
}
.shared-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.55rem;
}
.shared-name {
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
.shared-meta {
  margin-top: 0.4rem;
  font-size: 0.82rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.75rem;
}
.publisher {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.publisher a {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: inherit;
  text-decoration: none;
}
.publisher a:hover {
  color: var(--accent);
}
.meta-avatar {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  object-fit: cover;
  display: block;
}
.meta-sep::before {
  content: "·";
  margin-right: 0.75rem;
  opacity: 0.6;
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
  .shared-list li {
    grid-template-columns: auto 1fr;
    grid-template-areas:
      "cover body"
      "actions actions";
  }
  .shared-cover {
    grid-area: cover;
  }
  .shared-body {
    grid-area: body;
  }
  .row-actions {
    grid-area: actions;
    justify-self: end;
  }
}
</style>
