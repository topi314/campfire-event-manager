<script setup lang="ts">
import type { Club, ClubEvent } from "~/types";
import { isCampfireTokenError } from "~/composables/useSessionToken";
import { coverImageSrc } from "~/utils/covers";
import { formatInstant } from "~/utils/datetime";

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { token, authHeaders, openSettings, invalidateToken } = useSessionToken();
const { timeZone, preferredClubId } = usePreferences();
const toast = useToast();
const { apiBase } = useRuntimeConfig().public;

const clubs = ref<Club[]>([]);
const clubsLoaded = ref(false);
const selectedClubId = ref("");
const clubNameFromEvents = ref("");
const events = ref<ClubEvent[]>([]);
const eventsCursor = ref("");
const eventsHasMore = ref(false);
const error = ref("");
const loadingClubs = ref(false);
const loadingEvents = ref(false);
const loadingMore = ref(false);
const busy = ref(false);
const sharingId = ref<string | null>(null);
const confirmDeleteId = ref<string | null>(null);
const detailEventId = ref<string | null>(null);
/** Ignore watchers until first bootstrap finishes. */
const ready = ref(false);

let clubsInflight: Promise<void> | null = null;
let eventsInflight: Promise<void> | null = null;
let eventsInflightClubId = "";

function onDetailKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && detailEventId.value && !busy.value) closeDetail();
}

onMounted(async () => {
  window.addEventListener("keydown", onDetailKeydown);
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  await bootstrap();
  ready.value = true;
});

onUnmounted(() => window.removeEventListener("keydown", onDetailKeydown));

watch(token, async (t) => {
  if (!ready.value) return;
  if (!t) {
    clubs.value = [];
    clubsLoaded.value = false;
    events.value = [];
    clubNameFromEvents.value = "";
    selectedClubId.value = "";
    detailEventId.value = null;
    return;
  }
  clubsLoaded.value = false;
  await bootstrap();
});

watch(selectedClubId, (id, prev) => {
  if (!ready.value) return;
  detailEventId.value = null;
  if (!id || id === prev) return;
  void loadEvents(id, { reset: true });
});

watch(preferredClubId, (preferred) => {
  if (!ready.value) return;
  const next = (preferred || "").trim();
  if (!next || next === selectedClubId.value) return;
  selectedClubId.value = next;
});

async function bootstrap() {
  if (!token.value) {
    clubs.value = [];
    events.value = [];
    selectedClubId.value = "";
    error.value = "Add your Campfire JWT in Settings to load club events.";
    return;
  }
  error.value = "";
  const preferred = preferredClubId.value.trim();
  if (preferred) {
    selectedClubId.value = preferred;
    // Events first — don't wait on the clubs dropdown list.
    await Promise.all([loadEvents(preferred, { reset: true }), ensureClubs()]);
    return;
  }
  await ensureClubs();
  if (!selectedClubId.value && clubs.value.length) {
    selectedClubId.value = clubs.value[0].id;
  }
  if (selectedClubId.value) await loadEvents(selectedClubId.value, { reset: true });
}

/** Club list is only needed to switch clubs — load on demand / in parallel. */
async function ensureClubs() {
  if (!token.value || clubsLoaded.value) return;
  if (clubsInflight) return clubsInflight;

  loadingClubs.value = true;
  clubsInflight = (async () => {
    try {
      clubs.value = await api<Club[]>("/api/campfire/clubs", { headers: authHeaders() });
      clubsLoaded.value = true;
      if (!selectedClubId.value && clubs.value.length) {
        selectedClubId.value = clubs.value[0].id;
      }
    } catch (e: any) {
      error.value = e.message || "Failed to load clubs";
      if (isCampfireTokenError(e)) {
        invalidateToken();
        openSettings({ focusToken: true });
      }
    } finally {
      loadingClubs.value = false;
      clubsInflight = null;
    }
  })();
  return clubsInflight;
}

type EventsPage = {
  events: ClubEvent[];
  clubId?: string;
  clubName?: string;
  endCursor?: string;
  hasNextPage?: boolean;
};

function mapEvents(list: ClubEvent[], clubId: string): ClubEvent[] {
  return (list || []).map((ev) => ({
    ...ev,
    clubId: ev.clubId || clubId,
  }));
}

async function fetchEventsPage(clubId: string, after = ""): Promise<EventsPage> {
  const qs = new URLSearchParams({ first: "50", lite: "1" });
  if (after) qs.set("after", after);
  return api<EventsPage>(
    `/api/campfire/clubs/${encodeURIComponent(clubId)}/events?${qs}`,
    { headers: authHeaders() },
  );
}

async function loadEvents(clubId: string, opts?: { reset?: boolean }) {
  if (!token.value || !clubId) {
    events.value = [];
    clubNameFromEvents.value = "";
    eventsCursor.value = "";
    eventsHasMore.value = false;
    return;
  }
  if (eventsInflight && eventsInflightClubId === clubId && opts?.reset) {
    return eventsInflight;
  }

  loadingEvents.value = true;
  error.value = "";
  eventsInflightClubId = clubId;
  eventsInflight = (async () => {
    try {
      const res = await fetchEventsPage(clubId);
      if (selectedClubId.value !== clubId) return;
      clubNameFromEvents.value = (res.clubName || "").trim();
      events.value = mapEvents(res.events || [], clubId).sort((a, b) => {
        const ta = Date.parse(a.eventTime) || 0;
        const tb = Date.parse(b.eventTime) || 0;
        return ta - tb;
      });
      eventsCursor.value = res.hasNextPage ? res.endCursor || "" : "";
      eventsHasMore.value = !!res.hasNextPage && !!res.endCursor;
    } catch (e: any) {
      if (selectedClubId.value !== clubId) return;
      events.value = [];
      clubNameFromEvents.value = "";
      eventsCursor.value = "";
      eventsHasMore.value = false;
      error.value = e.message || "Failed to load events";
      if (isCampfireTokenError(e)) {
        invalidateToken();
        openSettings({ focusToken: true });
      }
    } finally {
      if (eventsInflightClubId === clubId) {
        loadingEvents.value = false;
        eventsInflight = null;
        eventsInflightClubId = "";
      }
    }
  })();
  return eventsInflight;
}

async function loadMoreEvents() {
  const clubId = selectedClubId.value;
  const after = eventsCursor.value;
  if (!token.value || !clubId || !after || loadingMore.value || loadingEvents.value) return;

  loadingMore.value = true;
  error.value = "";
  try {
    const res = await fetchEventsPage(clubId, after);
    if (selectedClubId.value !== clubId) return;
    if (res.clubName) clubNameFromEvents.value = res.clubName.trim();
    const more = mapEvents(res.events || [], clubId);
    const seen = new Set(events.value.map((e) => e.id));
    const merged = [...events.value];
    for (const ev of more) {
      if (!seen.has(ev.id)) merged.push(ev);
    }
    events.value = merged.sort((a, b) => {
      const ta = Date.parse(a.eventTime) || 0;
      const tb = Date.parse(b.eventTime) || 0;
      return ta - tb;
    });
    eventsCursor.value = res.hasNextPage ? res.endCursor || "" : "";
    eventsHasMore.value = !!res.hasNextPage && !!res.endCursor;
  } catch (e: any) {
    if (selectedClubId.value !== clubId) return;
    error.value = e.message || "Failed to load more events";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    loadingMore.value = false;
  }
}

const selectedClub = computed(
  () => clubs.value.find((c) => c.id === selectedClubId.value) || null,
);

const selectedClubLabel = computed(() => {
  const fromList = selectedClub.value?.name;
  return (fromList || clubNameFromEvents.value || "").trim();
});

const eventRows = computed(() =>
  events.value.map((ev) => {
    const name = (ev.name || "").trim() || "Untitled meetup";
    const cover = coverImageSrc(ev.coverPhotoUrl, apiBase as string);
    const when = formatInstant(ev.eventTime, timeZone.value);
    const end = ev.eventEndTime
      ? formatInstant(ev.eventEndTime, timeZone.value)
      : "";
    const creator = ev.creator?.displayName || ev.creator?.username || "";
    return {
      ev,
      name,
      cover,
      initial: name.slice(0, 1).toUpperCase(),
      when,
      end,
      address: (ev.address || "").trim(),
      details: (ev.details || "").trim(),
      creator,
    };
  }),
);

const detailEvent = computed(
  () => events.value.find((e) => e.id === detailEventId.value) || null,
);

const detailRow = computed(
  () => eventRows.value.find((r) => r.ev.id === detailEventId.value) || null,
);

const confirmDeleteEvent = computed(
  () => events.value.find((e) => e.id === confirmDeleteId.value) || null,
);

function openDetail(ev: ClubEvent) {
  detailEventId.value = ev.id;
}

function closeDetail() {
  detailEventId.value = null;
}

function editEvent(ev: ClubEvent) {
  navigateTo({
    path: "/create",
    query: {
      mode: "edit",
      clubId: ev.clubId || selectedClubId.value,
      eventId: ev.id,
    },
  });
}

function askDelete(id: string) {
  confirmDeleteId.value = id;
}

function closeConfirm() {
  confirmDeleteId.value = null;
}

async function shareEvent(ev: ClubEvent) {
  if (!token.value || sharingId.value) return;
  sharingId.value = ev.id;
  error.value = "";
  try {
    const res = await api<{ url: string; shortId: string }>(
      `/api/campfire/meetups/${encodeURIComponent(ev.id)}/share-link`,
      { method: "POST", headers: authHeaders() },
    );
    const url = (res.url || "").trim();
    if (!url) throw new Error("No share link returned");
    await navigator.clipboard.writeText(url);
    toast.success("Share link copied", url);
  } catch (e: any) {
    error.value = e.message || "Failed to copy share link";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    sharingId.value = null;
  }
}

async function onConfirmDelete() {
  const id = confirmDeleteId.value;
  if (!id || !token.value) return;
  busy.value = true;
  error.value = "";
  try {
    await api(`/api/campfire/meetups/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: authHeaders(),
    });
    const name = confirmDeleteEvent.value?.name || "Meetup";
    toast.success("Meetup deleted", name);
    events.value = events.value.filter((e) => e.id !== id);
    if (detailEventId.value === id) detailEventId.value = null;
    closeConfirm();
  } catch (e: any) {
    error.value = e.message || "Failed to delete meetup";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel">
      <div class="page-head">
        <div>
          <h2>Upcoming events{{ eventRows.length ? ` (${eventRows.length})` : "" }}</h2>
          <p class="muted">Upcoming and ongoing meetups for clubs you admin.</p>
        </div>
        <div class="page-actions">
          <NuxtLink to="/create" class="btn primary create-action">
            <Icon name="plus" />
            Create
          </NuxtLink>
        </div>
      </div>

      <div class="filters">
        <div class="field" style="margin-bottom: 0">
          <label for="events-club">Club</label>
          <select
            id="events-club"
            v-model="selectedClubId"
            :disabled="!token || loadingClubs"
            @focus="ensureClubs"
            @mousedown="ensureClubs"
          >
            <option v-if="!selectedClubId" value="" disabled>
              {{
                loadingClubs
                  ? "Loading clubs…"
                  : clubsLoaded
                    ? "No clubs where you are an admin"
                    : "Select a club…"
              }}
            </option>
            <option
              v-if="selectedClubId && !clubs.some((c) => c.id === selectedClubId)"
              :value="selectedClubId"
            >
              {{ selectedClubLabel || "Selected club" }}
            </option>
            <option v-for="c in clubs" :key="c.id" :value="c.id">
              {{ c.name }}
            </option>
          </select>
        </div>
      </div>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-else-if="loadingEvents && !eventRows.length" class="muted">Loading meetups…</p>

      <ul v-if="eventRows.length" class="event-list">
        <li
          v-for="row in eventRows"
          :key="row.ev.id"
          class="event-row"
          role="button"
          tabindex="0"
          :aria-label="`Open ${row.name}`"
          @click="openDetail(row.ev)"
          @keydown.enter.prevent="openDetail(row.ev)"
        >
          <div class="event-cover" aria-hidden="true">
            <img v-if="row.cover" :src="row.cover" alt="" />
            <span v-else>{{ row.initial }}</span>
          </div>
          <div class="event-body">
            <strong class="event-name">{{ row.name }}</strong>
            <div class="event-meta muted">
              <span v-if="selectedClubLabel">{{ selectedClubLabel }}</span>
              <span class="meta-sep">{{ row.when }}</span>
              <span v-if="row.end" class="meta-sep">→ {{ row.end }}</span>
              <span v-if="row.address" class="meta-sep">{{ row.address }}</span>
              <span v-if="row.creator" class="meta-sep">by {{ row.creator }}</span>
            </div>
          </div>
          <div class="row-actions" @click.stop>
            <button
              type="button"
              class="icon-btn"
              title="Copy share link"
              aria-label="Copy share link"
              :disabled="busy || !token || sharingId === row.ev.id"
              @click="shareEvent(row.ev)"
            >
              <Icon name="share" />
            </button>
            <button
              type="button"
              class="icon-btn"
              title="Edit"
              aria-label="Edit"
              :disabled="busy"
              @click="editEvent(row.ev)"
            >
              <Icon name="edit" />
            </button>
            <button
              type="button"
              class="icon-btn icon-btn-danger"
              title="Delete"
              aria-label="Delete"
              :disabled="busy || !token"
              @click="askDelete(row.ev.id)"
            >
              <Icon name="trash" />
            </button>
          </div>
        </li>
      </ul>
      <div v-if="eventsHasMore" class="load-more">
        <button
          type="button"
          :disabled="loadingMore || loadingEvents || !token"
          @click="loadMoreEvents"
        >
          {{ loadingMore ? "Loading…" : "Load more" }}
        </button>
      </div>
      <p v-else-if="selectedClubId && !loadingEvents && !eventRows.length" class="muted">
        No upcoming meetups in this club.
      </p>
      <p v-else-if="!token" class="muted">
        Add your Campfire JWT in Settings to load events.
      </p>
    </div>

    <div
      v-if="detailEvent && detailRow"
      class="modal-backdrop"
      @click.self="!busy && closeDetail()"
    >
      <div
        class="modal modal-preview event-detail-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="event-detail-title"
      >
        <div class="detail-head">
          <div>
            <h2 id="event-detail-title">{{ detailRow.name }}</h2>
            <p v-if="selectedClubLabel || detailRow.creator" class="muted detail-sub">
              <span v-if="selectedClubLabel">{{ selectedClubLabel }}</span>
              <span v-if="detailRow.creator" class="meta-sep">by {{ detailRow.creator }}</span>
            </p>
          </div>
          <button type="button" class="icon-close" aria-label="Close" @click="closeDetail">
            ×
          </button>
        </div>

        <div class="detail-body">
          <img v-if="detailRow.cover" class="detail-cover" :src="detailRow.cover" alt="" />
          <dl class="detail-meta">
            <dt>When</dt>
            <dd>
              {{ detailRow.when }}
              <template v-if="detailRow.end"> – {{ detailRow.end }}</template>
            </dd>
            <template v-if="detailRow.address">
              <dt>Address</dt>
              <dd>{{ detailRow.address }}</dd>
            </template>
            <template v-if="detailRow.details">
              <dt>Description</dt>
              <dd class="pre">{{ detailRow.details }}</dd>
            </template>
          </dl>
        </div>

        <div class="modal-actions">
          <button
            type="button"
            :disabled="busy || !token || sharingId === detailEvent.id"
            @click="shareEvent(detailEvent)"
          >
            <Icon name="share" />
            {{ sharingId === detailEvent.id ? "Copying…" : "Share" }}
          </button>
          <button type="button" :disabled="busy" @click="editEvent(detailEvent)">
            <Icon name="edit" />
            Edit
          </button>
          <button
            type="button"
            class="danger"
            :disabled="busy || !token"
            @click="askDelete(detailEvent.id)"
          >
            <Icon name="trash" />
            Delete
          </button>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :open="confirmDeleteId != null"
      title="Delete meetup?"
      :message="
        confirmDeleteEvent
          ? `Delete “${confirmDeleteEvent.name || 'this meetup'}” from Campfire? This cannot be undone.`
          : 'Delete this meetup from Campfire?'
      "
      confirm-label="Delete"
      :danger="true"
      :busy="busy"
      @cancel="closeConfirm"
      @confirm="onConfirmDelete"
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
.filters {
  display: grid;
  grid-template-columns: minmax(12rem, 20rem);
  gap: 0.75rem;
  margin-bottom: 1rem;
}
.load-more {
  margin-top: 0.85rem;
  display: flex;
  justify-content: center;
}
.event-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.event-row {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 0.85rem;
  align-items: center;
  padding: 0.85rem 1rem;
  border: 1px solid var(--border);
  border-radius: calc(var(--radius) + 2px);
  background: color-mix(in srgb, var(--bg-input) 55%, var(--bg-elevated));
  cursor: pointer;
}
.event-row:hover {
  border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
}
.event-row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.event-cover {
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
.event-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.event-body {
  min-width: 0;
}
.event-name {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.25;
}
.event-meta {
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
.detail-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}
.detail-head h2 {
  margin: 0 0 0.25rem;
}
.detail-sub {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem 0.75rem;
  font-size: 0.85rem;
}
.icon-close {
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 1.5rem;
  line-height: 1;
  padding: 0.15rem 0.4rem;
  cursor: pointer;
}
.icon-close:hover {
  color: var(--text);
}
.detail-body {
  margin-top: 1rem;
}
.detail-cover {
  width: 100%;
  max-height: 200px;
  object-fit: cover;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  margin-bottom: 1rem;
  display: block;
}
.detail-meta {
  margin: 0;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.45rem 0.85rem;
  font-size: 0.92rem;
}
.detail-meta dt {
  margin: 0;
  color: var(--muted);
  font-weight: 500;
}
.detail-meta dd {
  margin: 0;
}
.detail-meta dd.pre {
  white-space: pre-wrap;
  word-break: break-word;
}
.event-detail-modal .modal-actions {
  flex-wrap: wrap;
}
@media (max-width: 560px) {
  .filters {
    grid-template-columns: 1fr;
  }
  .event-row {
    grid-template-columns: auto 1fr;
    grid-template-areas:
      "cover body"
      "actions actions";
  }
  .event-cover {
    grid-area: cover;
  }
  .event-body {
    grid-area: body;
  }
  .row-actions {
    grid-area: actions;
    justify-self: end;
  }
}
</style>
