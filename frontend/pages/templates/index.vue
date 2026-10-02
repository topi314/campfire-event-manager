<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate } from "~/types";
import { formatInstant } from "~/utils/datetime";
import { TEMPLATE_CATEGORY_OPTIONS } from "~/utils/eventCategory";
import {
  TEMPLATE_LANGUAGES,
  guessTemplateLanguage,
  templateLanguageLabel,
} from "~/utils/languages";
import { coverImageSrc } from "~/utils/covers";

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { timeZone } = usePreferences();
const route = useRoute();
const { apiBase } = useRuntimeConfig().public;

// Legacy: /templates?edit=123 → /templates/123/edit
const legacy = route.query.edit;
if (typeof legacy === "string" && /^\d+$/.test(legacy)) {
  await navigateTo(`/templates/${legacy}/edit`, { redirectCode: 301, replace: true });
}

const templates = ref<MeetupTemplate[]>([]);
const error = ref("");
const success = ref("");
const importing = ref(false);
const publishingId = ref<number | null>(null);
const publishTarget = ref<MeetupTemplate | null>(null);
const publishDescription = ref("");
const publishLanguage = ref(guessTemplateLanguage());
const preview = ref<MeetupTemplate | null>(null);

const filterQ = ref("");
const filterCategory = ref("");
const filterLanguage = ref("");
const filterCreator = ref("");
const filterStatus = ref<"all" | "published" | "private">("all");
const sort = ref<"updated_desc" | "created_desc" | "name_asc" | "published_desc">(
  "updated_desc",
);

onMounted(async () => {
  if (!(await ensureAuth())) return;
  await load();
});

async function load() {
  try {
    templates.value = await api<MeetupTemplate[]>("/api/templates");
  } catch (e: any) {
    error.value = e.message || "Failed to load templates";
  }
}

function parsePayload(t: MeetupTemplate): MeetupPayload {
  try {
    const raw = typeof t.payload === "string" ? JSON.parse(t.payload) : t.payload;
    return (raw && typeof raw === "object" ? raw : {}) as MeetupPayload;
  } catch {
    return {} as MeetupPayload;
  }
}

function coverOf(t: MeetupTemplate) {
  return coverImageSrc(parsePayload(t).coverPhotoUrl, apiBase as string);
}

function categoryLabel(t: MeetupTemplate) {
  return (parsePayload(t).category || "").trim();
}

function originCreatorLabel(t: MeetupTemplate) {
  const p = t.origin?.publisher;
  if (!p) return "";
  return (p.displayName || p.username || "").trim();
}

const creatorOptions = computed(() => {
  const byId = new Map<string, string>();
  for (const t of templates.value) {
    const p = t.origin?.publisher;
    if (!p?.id) continue;
    const label = (p.displayName || p.username || p.id).trim();
    if (!label) continue;
    if (!byId.has(p.id)) byId.set(p.id, label);
  }
  return [...byId.entries()]
    .map(([id, label]) => ({ id, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
});

const filteredTemplates = computed(() => {
  const q = filterQ.value.trim().toLowerCase();
  const list = templates.value.filter((t) => {
    if (filterStatus.value === "published" && !t.publishedAt) return false;
    if (filterStatus.value === "private" && t.publishedAt) return false;
    if (filterCategory.value && categoryLabel(t) !== filterCategory.value) return false;
    if (
      filterLanguage.value &&
      (t.language || "").toLowerCase() !== filterLanguage.value
    ) {
      return false;
    }
    if (filterCreator.value) {
      if ((t.origin?.publisher?.id || "") !== filterCreator.value) return false;
    }
    if (q) {
      const hay = [
        t.name,
        t.publishDescription || "",
        categoryLabel(t),
        templateLanguageLabel(t.language),
        originCreatorLabel(t),
      ]
        .join(" ")
        .toLowerCase();
      if (!hay.includes(q)) return false;
    }
    return true;
  });

  return [...list].sort((a, b) => {
    switch (sort.value) {
      case "name_asc":
        return a.name.localeCompare(b.name) || b.id - a.id;
      case "created_desc":
        return (b.createdAt || "").localeCompare(a.createdAt || "") || b.id - a.id;
      case "published_desc": {
        const ap = a.publishedAt || "";
        const bp = b.publishedAt || "";
        if (ap && bp) return bp.localeCompare(ap) || b.id - a.id;
        if (ap) return -1;
        if (bp) return 1;
        return (b.updatedAt || "").localeCompare(a.updatedAt || "") || b.id - a.id;
      }
      case "updated_desc":
      default:
        return (b.updatedAt || "").localeCompare(a.updatedAt || "") || b.id - a.id;
    }
  });
});

function startCreate() {
  navigateTo("/templates/new");
}

function openPreview(t: MeetupTemplate) {
  preview.value = t;
}

function closePreview() {
  preview.value = null;
}

function onPreviewEdit() {
  if (preview.value) void goEdit(preview.value);
}

async function goEdit(t: MeetupTemplate) {
  closePreview();
  await navigateTo(`/templates/${t.id}/edit`);
}

async function remove(id: number) {
  if (!confirm("Delete this template?")) return;
  error.value = "";
  try {
    await api(`/api/templates/${id}`, { method: "DELETE" });
    if (preview.value?.id === id) closePreview();
    await load();
    success.value = "Deleted";
  } catch (e: any) {
    error.value = e.message || "Failed to delete";
  }
}

function openPublishModal(t: MeetupTemplate) {
  publishTarget.value = t;
  publishDescription.value = (t.publishDescription || "").trim();
  const existing = (t.language || "").trim().toLowerCase();
  publishLanguage.value = (
    TEMPLATE_LANGUAGES.some((l) => l.code === existing)
      ? existing
      : guessTemplateLanguage()
  ) as typeof publishLanguage.value;
  error.value = "";
  success.value = "";
}

function closePublishModal() {
  if (publishingId.value) return;
  publishTarget.value = null;
  publishDescription.value = "";
}

async function confirmPublish() {
  const t = publishTarget.value;
  if (!t) return;
  if (!publishLanguage.value) {
    error.value = "Pick a language for the listing";
    return;
  }
  publishingId.value = t.id;
  error.value = "";
  try {
    await api(`/api/templates/${t.id}/publish`, {
      method: "POST",
      body: JSON.stringify({
        description: publishDescription.value.trim(),
        language: publishLanguage.value,
      }),
    });
    await load();
    if (preview.value?.id === t.id) {
      preview.value = templates.value.find((x) => x.id === t.id) || preview.value;
    }
    success.value = t.publishedAt
      ? "Listing updated"
      : "Published — others can find it under Browse";
    publishTarget.value = null;
    publishDescription.value = "";
  } catch (e: any) {
    error.value = e.message || "Failed to publish";
  } finally {
    publishingId.value = null;
  }
}

async function unpublish(t: MeetupTemplate) {
  publishingId.value = t.id;
  error.value = "";
  try {
    await api(`/api/templates/${t.id}/unpublish`, { method: "POST" });
    await load();
    if (preview.value?.id === t.id) {
      preview.value = templates.value.find((x) => x.id === t.id) || preview.value;
    }
    success.value = "Unpublished";
  } catch (e: any) {
    error.value = e.message || "Failed to unpublish";
  } finally {
    publishingId.value = null;
  }
}

async function onImport(ev: Event) {
  const input = ev.target as HTMLInputElement;
  if (!input.files?.length) return;
  importing.value = true;
  error.value = "";
  try {
    const fd = new FormData();
    for (const f of Array.from(input.files)) {
      fd.append("files", f);
    }
    await api("/api/templates/import", { method: "POST", body: fd });
    await load();
    success.value = "Imported";
    input.value = "";
  } catch (e: any) {
    error.value = e.message || "Import failed";
  } finally {
    importing.value = false;
  }
}

function download(t: MeetupTemplate) {
  const blob = new Blob(
    [JSON.stringify({ name: t.name, payload: t.payload }, null, 2)],
    { type: "application/json" },
  );
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${t.name.replace(/[^\w.-]+/g, "_")}.json`;
  a.click();
  URL.revokeObjectURL(url);
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
            <h2>My templates</h2>
            <p class="muted">
              Your private library. Use them on
              <NuxtLink to="/create">Create</NuxtLink>,
              post later from
              <NuxtLink to="/create/drafts">Drafts</NuxtLink>,
              or publish to
              <NuxtLink to="/templates/browse">Browse templates</NuxtLink>.
            </p>
          </div>
          <div class="page-actions">
            <label class="btn">
              {{ importing ? "Importing…" : "Upload JSON" }}
              <input
                type="file"
                accept="application/json,.json"
                multiple
                hidden
                @change="onImport"
              />
            </label>
            <button type="button" class="primary" @click="startCreate">
              <Icon name="plus" />
              New template
            </button>
          </div>
        </div>

        <p v-if="error" class="error">{{ error }}</p>
        <p v-if="success" class="success">{{ success }}</p>

        <div v-if="templates.length" class="filters">
          <div class="field" style="margin-bottom: 0">
            <label for="mine-q">Search</label>
            <input
              id="mine-q"
              v-model="filterQ"
              type="search"
              placeholder="Name or listing…"
            />
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="mine-cat">Category</label>
            <select id="mine-cat" v-model="filterCategory">
              <option value="">Any</option>
              <option v-for="c in TEMPLATE_CATEGORY_OPTIONS" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="mine-lang">Language</label>
            <select id="mine-lang" v-model="filterLanguage">
              <option value="">Any</option>
              <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
                {{ l.label }}
              </option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="mine-creator">Creator</label>
            <select id="mine-creator" v-model="filterCreator">
              <option value="">Any</option>
              <option v-for="c in creatorOptions" :key="c.id" :value="c.id">
                {{ c.label }}
              </option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="mine-status">Status</label>
            <select id="mine-status" v-model="filterStatus">
              <option value="all">All</option>
              <option value="published">Published</option>
              <option value="private">Private</option>
            </select>
          </div>
          <div class="field" style="margin-bottom: 0">
            <label for="mine-sort">Sort</label>
            <select id="mine-sort" v-model="sort">
              <option value="updated_desc">Recently updated</option>
              <option value="created_desc">Newest created</option>
              <option value="published_desc">Newest published</option>
              <option value="name_asc">Name A–Z</option>
            </select>
          </div>
        </div>

        <ul v-if="filteredTemplates.length" class="mine-list">
          <li
            v-for="t in filteredTemplates"
            :key="t.id"
            role="button"
            tabindex="0"
            @click="openPreview(t)"
            @keydown.enter.prevent="openPreview(t)"
          >
            <div class="mine-cover" aria-hidden="true">
              <img v-if="coverOf(t)" :src="coverOf(t)" alt="" />
              <span v-else class="cover-fallback">{{ (t.name || "?").slice(0, 1).toUpperCase() }}</span>
            </div>
            <div class="mine-body">
              <div class="mine-title-row">
                <strong class="mine-name">{{ t.name }}</strong>
                <span v-if="t.publishedAt" class="badge">Published</span>
                <span v-if="t.origin && t.synced" class="badge synced">Synced</span>
                <span v-else-if="t.origin && !t.synced" class="badge unsynced">Unsynced</span>
                <span v-if="categoryLabel(t)" class="cat-badge">{{ categoryLabel(t) }}</span>
                <span v-if="t.language" class="cat-badge">{{ templateLanguageLabel(t.language) }}</span>
              </div>
              <p v-if="t.publishDescription" class="listing-desc muted">
                {{ t.publishDescription }}
              </p>
              <div class="mine-meta muted">
                <span>Updated {{ formatInstant(t.updatedAt, timeZone) }}</span>
                <template v-if="t.origin">
                  <span class="meta-sep">
                    {{ t.synced ? "Synced with" : "Based on" }}
                    <NuxtLink :to="`/templates/browse/${t.origin.id}`" @click.stop>
                      {{ t.origin.name }}
                    </NuxtLink>
                    <template v-if="t.origin.publisher">
                      by
                      <NuxtLink
                        :to="`/profiles/${t.origin.publisher.id}`"
                        @click.stop
                      >
                        {{ t.origin.publisher.displayName || t.origin.publisher.username }}
                      </NuxtLink>
                    </template>
                  </span>
                </template>
              </div>
            </div>
            <div class="row-actions" @click.stop>
              <button
                type="button"
                class="icon-btn icon-btn-accent"
                title="Edit"
                aria-label="Edit"
                @click="goEdit(t)"
              >
                <Icon name="edit" />
              </button>
              <button
                v-if="!t.publishedAt"
                type="button"
                class="icon-btn"
                title="Publish"
                aria-label="Publish"
                :disabled="publishingId === t.id"
                @click="openPublishModal(t)"
              >
                <Icon name="publish" />
              </button>
              <template v-else>
                <button
                  type="button"
                  class="icon-btn"
                  title="Edit listing"
                  aria-label="Edit listing"
                  :disabled="publishingId === t.id"
                  @click="openPublishModal(t)"
                >
                  <Icon name="publish" />
                </button>
                <button
                  type="button"
                  class="icon-btn"
                  title="Unpublish"
                  aria-label="Unpublish"
                  :disabled="publishingId === t.id"
                  @click="unpublish(t)"
                >
                  <Icon name="unpublish" />
                </button>
              </template>
              <button
                type="button"
                class="icon-btn"
                title="Export"
                aria-label="Export"
                @click="download(t)"
              >
                <Icon name="export" />
              </button>
              <button
                type="button"
                class="icon-btn icon-btn-danger"
                title="Delete"
                aria-label="Delete"
                @click="remove(t.id)"
              >
                <Icon name="trash" />
              </button>
            </div>
          </li>
        </ul>
        <p v-else-if="templates.length" class="muted">No templates match these filters.</p>
        <p v-else class="muted">No templates yet. Create one to get started.</p>

        <TemplatePreview
          v-if="preview"
          variant="mine"
          :template="preview"
          @close="closePreview"
          @edit="onPreviewEdit"
        />

        <div
          v-if="publishTarget"
          class="modal-backdrop"
          @click.self="closePublishModal"
        >
          <div class="modal" role="dialog" aria-modal="true" aria-labelledby="publish-modal-title">
            <h2 id="publish-modal-title">
              {{ publishTarget.publishedAt ? "Edit listing" : "Publish to Browse" }}
            </h2>
            <p class="muted">
              Share “{{ publishTarget.name }}” with other Community Ambassadors.
              Add an optional short description for the listing.
            </p>
            <div class="field">
              <label for="publish-lang">Language</label>
              <select id="publish-lang" v-model="publishLanguage" required>
                <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
                  {{ l.label }}
                </option>
              </select>
            </div>
            <div class="field">
              <label for="publish-desc">Listing description (optional)</label>
              <textarea
                id="publish-desc"
                v-model="publishDescription"
                rows="3"
                maxlength="500"
                placeholder="What is this template for? When do you use it?"
              />
              <p class="hint muted">{{ publishDescription.length }}/500</p>
            </div>
            <div class="modal-actions">
              <button type="button" :disabled="!!publishingId" @click="closePublishModal">
                Cancel
              </button>
              <button
                type="button"
                class="primary"
                :disabled="!!publishingId"
                @click="confirmPublish"
              >
                {{
                  publishingId
                    ? "…"
                    : publishTarget.publishedAt
                      ? "Save listing"
                      : "Publish"
                }}
              </button>
            </div>
          </div>
        </div>
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
.page-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.filters {
  display: grid;
  grid-template-columns: minmax(7rem, 1.2fr) repeat(5, minmax(6rem, 1fr));
  gap: 0.65rem 0.75rem;
  margin: 0 0 1.15rem;
  align-items: end;
}
@media (max-width: 1100px) {
  .filters {
    grid-template-columns: 1fr 1fr 1fr;
  }
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
}
.mine-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.mine-list li {
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
.mine-list li:hover {
  border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
  background: color-mix(in srgb, var(--bg-input) 80%, var(--bg-elevated));
}
.mine-cover {
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
.mine-cover img {
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
.mine-body {
  min-width: 0;
}
.mine-title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.55rem;
}
.mine-name {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.25;
}
.badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--accent);
  color: var(--accent);
}
.badge.synced {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  color: var(--muted);
  background: color-mix(in srgb, var(--accent) 12%, var(--bg));
}
.badge.unsynced {
  border-color: var(--border);
  color: var(--muted);
  background: var(--bg);
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
.mine-meta {
  margin-top: 0.4rem;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.35rem 0.75rem;
  font-size: 0.82rem;
}
.meta-sep::before {
  content: "·";
  margin-right: 0.75rem;
  opacity: 0.6;
}
.row-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
}
.origin {
  margin: 0 0 0.75rem;
  font-size: 0.85rem;
}
.hint {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
@media (max-width: 560px) {
  .mine-list li {
    grid-template-columns: auto 1fr;
    grid-template-areas:
      "cover body"
      "actions actions";
  }
  .mine-cover {
    grid-area: cover;
  }
  .mine-body {
    grid-area: body;
  }
  .row-actions {
    grid-area: actions;
    justify-self: end;
  }
}
</style>
