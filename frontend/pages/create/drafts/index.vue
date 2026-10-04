<script setup lang="ts">
import type { MeetupDraftItem } from "~/types";
import {
  buildMeetupBodyFromDraft,
  formatDraftWhen,
  parseDraftPayload,
  resolveDraftDisplay,
  resolveDraftTextFields,
} from "~/utils/drafts";
import { isCampfireTokenError } from "~/composables/useSessionToken";
import { coverImageSrc } from "~/utils/covers";

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { token, authHeaders, openSettings, invalidateToken } = useSessionToken();
const { timeZone } = usePreferences();
const toast = useToast();
const { apiBase } = useRuntimeConfig().public;

const drafts = ref<MeetupDraftItem[]>([]);
const error = ref("");
const success = ref("");
const busy = ref(false);
const posting = ref(false);
const progress = ref("");
const selectedIds = ref<Set<number>>(new Set());

type ConfirmKind = "one" | "selected" | "remove" | "clear" | null;
const confirmKind = ref<ConfirmKind>(null);
const confirmDraftId = ref<number | null>(null);

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  await load();
});

async function load() {
  if (!token.value) {
    drafts.value = [];
    selectedIds.value = new Set();
    error.value = "Add your Campfire JWT in Settings to load club drafts.";
    return;
  }
  try {
    drafts.value = await api<MeetupDraftItem[]>("/api/drafts", {
      headers: authHeaders(),
    });
    pruneSelection();
    error.value = "";
  } catch (e: any) {
    error.value = e.message || "Failed to load drafts";
    drafts.value = [];
    selectedIds.value = new Set();
    if (isCampfireTokenError(e)) invalidateToken();
  }
}

function pruneSelection() {
  const valid = new Set(drafts.value.map((d) => d.id));
  const next = new Set<number>();
  for (const id of selectedIds.value) {
    if (valid.has(id)) next.add(id);
  }
  selectedIds.value = next;
}

function isSelected(id: number) {
  return selectedIds.value.has(id);
}

function toggleSelected(id: number, on?: boolean) {
  const next = new Set(selectedIds.value);
  const enable = on ?? !next.has(id);
  if (enable) next.add(id);
  else next.delete(id);
  selectedIds.value = next;
}

const allSelected = computed(
  () => drafts.value.length > 0 && drafts.value.every((d) => selectedIds.value.has(d.id)),
);

function toggleSelectAll() {
  if (allSelected.value) {
    selectedIds.value = new Set();
    return;
  }
  selectedIds.value = new Set(drafts.value.map((d) => d.id));
}

const selectedCount = computed(() => selectedIds.value.size);

const confirmTitle = computed(() => {
  switch (confirmKind.value) {
    case "one":
      return "Post meetup?";
    case "selected":
      return "Post drafts?";
    case "remove":
      return "Remove draft?";
    case "clear":
      return "Clear drafts?";
    default:
      return "";
  }
});

const confirmMessage = computed(() => {
  switch (confirmKind.value) {
    case "one": {
      const item = drafts.value.find((d) => d.id === confirmDraftId.value);
      const name = item ? resolveDraftDisplay(item, timeZone.value).name : "this draft";
      return `Create “${name}” on Campfire now? The draft will be removed after a successful post.`;
    }
    case "selected":
      return `Post ${selectedCount.value} draft${selectedCount.value === 1 ? "" : "s"} to Campfire? Successful posts remove those drafts.`;
    case "remove": {
      const item = drafts.value.find((d) => d.id === confirmDraftId.value);
      const name = item ? resolveDraftDisplay(item, timeZone.value).name : "this draft";
      return `Remove “${name}”? This cannot be undone.`;
    }
    case "clear":
      return `Remove ${selectedCount.value} draft${selectedCount.value === 1 ? "" : "s"}? This cannot be undone.`;
    default:
      return "";
  }
});

const confirmLabel = computed(() => {
  switch (confirmKind.value) {
    case "one":
      return "Post";
    case "selected":
      return `Post ${selectedCount.value}`;
    case "remove":
      return "Remove";
    case "clear":
      return `Clear ${selectedCount.value}`;
    default:
      return "Confirm";
  }
});

const confirmDanger = computed(
  () => confirmKind.value === "clear" || confirmKind.value === "remove",
);

function closeConfirm() {
  if (posting.value || busy.value) return;
  confirmKind.value = null;
  confirmDraftId.value = null;
}

function askPostOne(id: number) {
  if (!token.value) {
    openSettings({ focusToken: true });
    return;
  }
  confirmDraftId.value = id;
  confirmKind.value = "one";
}

function askPostSelected() {
  if (!token.value) {
    openSettings({ focusToken: true });
    return;
  }
  if (!selectedCount.value) {
    error.value = "Select at least one draft to post";
    return;
  }
  confirmKind.value = "selected";
}

function askRemoveOne(id: number) {
  confirmDraftId.value = id;
  confirmKind.value = "remove";
}

function askClearSelected() {
  if (!selectedCount.value) {
    error.value = "Select at least one draft to clear";
    return;
  }
  confirmKind.value = "clear";
}

async function onConfirmAction() {
  const kind = confirmKind.value;
  const id = confirmDraftId.value;
  confirmKind.value = null;
  confirmDraftId.value = null;
  if (kind === "one" && id != null) {
    await postOne(id);
    return;
  }
  if (kind === "selected") {
    await postMany([...selectedIds.value]);
    return;
  }
  if (kind === "remove" && id != null) {
    await removeOne(id);
    return;
  }
  if (kind === "clear") {
    await clearMany([...selectedIds.value]);
  }
}

async function removeOne(id: number) {
  const item = drafts.value.find((d) => d.id === id);
  const name = item ? resolveDraftDisplay(item, timeZone.value).name : undefined;
  busy.value = true;
  error.value = "";
  try {
    await api(`/api/drafts/${id}`, {
      method: "DELETE",
      headers: authHeaders(),
    });
    drafts.value = drafts.value.filter((d) => d.id !== id);
    toggleSelected(id, false);
    toast.success("Draft deleted", name || undefined);
  } catch (e: any) {
    error.value = e.message || "Failed to remove draft";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    busy.value = false;
  }
}

async function clearMany(ids: number[]) {
  const pending = ids.filter((id) => drafts.value.some((d) => d.id === id));
  if (!pending.length) {
    error.value = "No drafts to clear";
    return;
  }

  busy.value = true;
  error.value = "";
  success.value = "";
  const removed: number[] = [];
  const errors: string[] = [];

  try {
    for (let i = 0; i < pending.length; i++) {
      const id = pending[i];
      const item = drafts.value.find((d) => d.id === id);
      const name = item ? resolveDraftDisplay(item, timeZone.value).name : `#${id}`;
      progress.value = `Clearing ${i + 1} of ${pending.length}: ${name}`;
      try {
        await api(`/api/drafts/${id}`, {
          method: "DELETE",
          headers: authHeaders(),
        });
        drafts.value = drafts.value.filter((d) => d.id !== id);
        toggleSelected(id, false);
        removed.push(id);
      } catch (e: any) {
        errors.push(`${name}: ${e.message || "failed"}`);
        if (isCampfireTokenError(e)) {
          invalidateToken();
          break;
        }
      }
    }
    if (removed.length && !errors.length) {
      toast.success(
        removed.length === 1 ? "Draft deleted" : `${removed.length} drafts deleted`,
      );
    } else if (removed.length && errors.length) {
      toast.success(`Deleted ${removed.length}, ${errors.length} failed`);
      error.value = errors.join("; ");
    } else {
      error.value = errors.join("; ") || "Failed to clear drafts";
    }
  } finally {
    busy.value = false;
    progress.value = "";
  }
}

function edit(id: number) {
  navigateTo(`/create/drafts/${id}`);
}

async function postItem(item: MeetupDraftItem) {
  const payload = parseDraftPayload(item);
  const body = buildMeetupBodyFromDraft(payload, timeZone.value);
  const event = await api<{ id: string; name: string }>("/api/campfire/meetups", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(body),
  });
  await api(`/api/drafts/${item.id}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  drafts.value = drafts.value.filter((d) => d.id !== item.id);
  toggleSelected(item.id, false);
  return event;
}

async function postOne(id: number) {
  const item = drafts.value.find((d) => d.id === id);
  if (!item) return;

  posting.value = true;
  error.value = "";
  success.value = "";
  const name = resolveDraftDisplay(item, timeZone.value).name;
  progress.value = `Posting “${name}”…`;
  try {
    const event = await postItem(item);
    toast.success("Meetup posted", event.name || undefined);
  } catch (e: any) {
    error.value = e.message || "Failed to post meetup";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    posting.value = false;
    progress.value = "";
  }
}

async function postMany(ids: number[]) {
  const pending = ids
    .map((id) => drafts.value.find((d) => d.id === id))
    .filter((d): d is MeetupDraftItem => !!d);
  if (!pending.length) {
    error.value = "No drafts to post";
    return;
  }

  posting.value = true;
  error.value = "";
  success.value = "";
  const created: string[] = [];
  const errors: string[] = [];

  try {
    for (let i = 0; i < pending.length; i++) {
      const item = pending[i];
      const name = resolveDraftDisplay(item, timeZone.value).name;
      progress.value = `Posting ${i + 1} of ${pending.length}: ${name}`;
      try {
        const event = await postItem(item);
        created.push(event.name || name);
      } catch (e: any) {
        errors.push(`${name}: ${e.message || "failed"}`);
        if (isCampfireTokenError(e)) {
          invalidateToken();
          break;
        }
      }
    }
    if (created.length && !errors.length) {
      toast.success(
        created.length === 1 ? "Meetup posted" : `${created.length} meetups posted`,
        created.length === 1 ? created[0] : undefined,
      );
    } else if (created.length && errors.length) {
      toast.success(`Posted ${created.length}, ${errors.length} failed`);
      error.value = errors.join("; ");
    } else {
      error.value = errors.join("; ") || "All posts failed";
    }
  } finally {
    posting.value = false;
    progress.value = "";
  }
}

function creatorLabel(item: MeetupDraftItem) {
  const c = item.creator;
  if (!c) return "";
  return c.displayName || c.username || "";
}

const draftRows = computed(() =>
  drafts.value.map((item) => {
    const raw = parseDraftPayload(item);
    const text = resolveDraftTextFields(raw, timeZone.value);
    const name = text.name || "?";
    const cover = coverImageSrc(text.coverPhotoUrl, apiBase as string);
    return {
      item,
      name,
      cover,
      initial: name.slice(0, 1).toUpperCase(),
      clubName: raw.clubName || "Club",
      liveEventName: raw.liveEventName || "No live event",
      templateName: raw.templateName || "Custom",
      when: formatDraftWhen(raw.eventTime),
      creator: creatorLabel(item),
    };
  }),
);
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel">
      <div class="page-head">
        <div>
          <h2>Drafts{{ drafts.length ? ` (${drafts.length})` : "" }}</h2>
          <p class="muted">
            Club-shared meetups ready to post — visible to every admin of that club.
            Build new ones on <NuxtLink to="/create">Create</NuxtLink>.
          </p>
        </div>
        <div class="page-actions">
          <button
            type="button"
            class="primary"
            :disabled="posting || busy || !selectedCount || !token"
            @click="askPostSelected"
          >
            <Icon name="post" />
            {{ posting ? "Posting…" : selectedCount ? `Post (${selectedCount})` : "Post" }}
          </button>
          <button
            type="button"
            :disabled="posting || busy || !selectedCount"
            @click="askClearSelected"
          >
            <Icon name="clear" />
            {{ selectedCount ? `Clear (${selectedCount})` : "Clear" }}
          </button>
          <NuxtLink to="/create" class="btn primary create-action">
            <Icon name="plus" />
            Create
          </NuxtLink>
        </div>
      </div>

      <p v-if="progress" class="muted">{{ progress }}</p>
      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="success" class="success">{{ success }}</p>

      <div v-if="draftRows.length" class="selection-bar">
        <label class="select-all">
          <input
            type="checkbox"
            :checked="allSelected"
            :disabled="posting || busy"
            :indeterminate.prop="selectedCount > 0 && !allSelected"
            @change="toggleSelectAll"
          />
          <span>{{ allSelected ? "Deselect all" : "Select all" }}</span>
        </label>
        <span v-if="selectedCount" class="muted">{{ selectedCount }} selected</span>
      </div>

      <ul v-if="draftRows.length" class="draft-list">
        <li
          v-for="row in draftRows"
          :key="row.item.id"
          class="draft-row"
          :class="{ selected: isSelected(row.item.id) }"
          role="button"
          tabindex="0"
          :aria-label="`Edit ${row.name}`"
          @click="edit(row.item.id)"
          @keydown.enter.prevent="edit(row.item.id)"
        >
          <label class="draft-check" @click.stop>
            <input
              type="checkbox"
              :checked="isSelected(row.item.id)"
              :disabled="posting || busy"
              :aria-label="`Select ${row.name}`"
              @change="toggleSelected(row.item.id)"
            />
          </label>
          <div class="draft-cover" aria-hidden="true">
            <img v-if="row.cover" :src="row.cover" alt="" />
            <span v-else>{{ row.initial }}</span>
          </div>
          <div class="draft-body">
            <strong class="draft-name">{{ row.name }}</strong>
            <div class="draft-meta muted">
              <span>{{ row.clubName }}</span>
              <span class="meta-sep">{{ row.liveEventName }}</span>
              <span class="meta-sep">{{ row.templateName }}</span>
              <span class="meta-sep">{{ row.when }}</span>
              <span v-if="row.creator" class="meta-sep">by {{ row.creator }}</span>
            </div>
          </div>
          <div class="row-actions" @click.stop>
            <button
              type="button"
              class="icon-btn icon-btn-accent"
              title="Post"
              aria-label="Post"
              :disabled="posting || busy || !token"
              @click="askPostOne(row.item.id)"
            >
              <Icon name="post" />
            </button>
            <button
              type="button"
              class="icon-btn"
              title="Edit"
              aria-label="Edit"
              :disabled="posting || busy"
              @click="edit(row.item.id)"
            >
              <Icon name="edit" />
            </button>
            <button
              type="button"
              class="icon-btn icon-btn-danger"
              title="Remove"
              aria-label="Remove"
              :disabled="posting || busy"
              @click="askRemoveOne(row.item.id)"
            >
              <Icon name="trash" />
            </button>
          </div>
        </li>
      </ul>
      <p v-else class="muted">
        No drafts yet. Save a meetup from
        <NuxtLink to="/create">Create</NuxtLink>
        to post it later.
      </p>
    </div>

    <ConfirmDialog
      :open="confirmKind != null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-label="confirmLabel"
      :danger="confirmDanger"
      :busy="posting || busy"
      @cancel="closeConfirm"
      @confirm="onConfirmAction"
    />
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
  margin-left: auto;
  align-items: center;
}
.page-actions .create-action {
  margin-left: 0.25rem;
}
.selection-bar {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  margin-bottom: 0.75rem;
  flex-wrap: wrap;
}
.select-all {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  cursor: pointer;
  font-size: 0.9rem;
}
.draft-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.draft-list li {
  display: grid;
  grid-template-columns: auto auto 1fr auto;
  gap: 0.85rem;
  align-items: center;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: calc(var(--radius) + 2px);
  background: color-mix(in srgb, var(--bg-input) 55%, var(--bg-elevated));
  cursor: pointer;
}
.draft-list li:hover {
  border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
}
.draft-list li.selected {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  background: color-mix(in srgb, var(--accent) 8%, var(--bg-elevated));
}
.draft-list li:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.draft-check {
  display: grid;
  place-items: center;
  cursor: pointer;
}
.draft-check input {
  width: 1.05rem;
  height: 1.05rem;
  accent-color: var(--accent);
}
.draft-cover {
  width: 56px;
  height: 56px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  overflow: hidden;
  flex-shrink: 0;
  background: var(--bg);
  display: grid;
  place-items: center;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--muted);
  line-height: 1;
}
.draft-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.draft-body {
  min-width: 0;
}
.draft-name {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.25;
}
.draft-meta {
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
@media (max-width: 560px) {
  .draft-list li {
    grid-template-columns: auto auto 1fr;
    grid-template-areas:
      "check cover body"
      "actions actions actions";
  }
  .draft-check {
    grid-area: check;
  }
  .draft-cover {
    grid-area: cover;
  }
  .draft-body {
    grid-area: body;
  }
  .row-actions {
    grid-area: actions;
    justify-self: end;
  }
}
</style>
