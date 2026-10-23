<script setup lang="ts">
import type { Club, ClubEvent } from "~/types";
import caStarUrl from "~/assets/images/ca-star.png";
import { isCampfireTokenError } from "~/composables/useSessionToken";
import { coverImageSrc } from "~/utils/covers";
import { formatInstant } from "~/utils/datetime";
import { parseCampfireLocation } from "~/utils/location";

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { token, authHeaders, openSettings, invalidateToken } = useSessionToken();
const { timeZone, timeFormat, preferredClubId } = usePreferences();
const toast = useToast();
const { apiBase } = useRuntimeConfig().public;
const route = useRoute();
const router = useRouter();

type EventTab = "upcoming" | "past";

function tabFromQuery(tab: unknown): EventTab {
  return tab === "past" ? "past" : "upcoming";
}

const eventTab = ref<EventTab>(tabFromQuery(route.query.tab));
const eventsPageSize = 15;

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
const sharingBulk = ref(false);
const progress = ref("");
const selectedIds = ref<Set<string>>(new Set());
const confirmDeleteId = ref<string | null>(null);
const confirmBulk = ref(false);
const detailEventId = ref<string | null>(null);
/** Ignore watchers until first bootstrap finishes. */
const ready = ref(false);

let clubsInflight: Promise<void> | null = null;
let eventsInflight: Promise<void> | null = null;
let eventsInflightKey = "";
let eventsGeneration = 0;

function eventsRequestKey(tab: EventTab, clubId: string) {
  return `${tab}:${clubId}`;
}

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

onUnmounted(() => {
  window.removeEventListener("keydown", onDetailKeydown);
  loadMoreObserver?.disconnect();
});

watch(token, async (t) => {
  if (!ready.value) return;
  if (!t) {
    clubs.value = [];
    clubsLoaded.value = false;
    events.value = [];
    clubNameFromEvents.value = "";
    selectedClubId.value = "";
    selectedIds.value = new Set();
    detailEventId.value = null;
    return;
  }
  clubsLoaded.value = false;
  await bootstrap();
});

watch(selectedClubId, (id, prev) => {
  if (!ready.value) return;
  detailEventId.value = null;
  selectedIds.value = new Set();
  confirmDeleteId.value = null;
  confirmBulk.value = false;
  if (!id || id === prev) return;
  void loadEvents(id, { reset: true });
});

watch(eventTab, (tab, prev) => {
  if (!ready.value || tab === prev) return;
  detailEventId.value = null;
  selectedIds.value = new Set();
  confirmDeleteId.value = null;
  confirmBulk.value = false;
  if (selectedClubId.value) void loadEvents(selectedClubId.value, { reset: true });
});

watch(
  () => route.query.tab,
  (tab) => {
    const next = tabFromQuery(tab);
    if (next !== eventTab.value) eventTab.value = next;
  },
);

function setEventTab(tab: EventTab) {
  if (eventTab.value === tab) return;
  const query = { ...route.query };
  if (tab === "past") query.tab = "past";
  else delete query.tab;
  eventTab.value = tab;
  void router.replace({ path: "/events", query });
}

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

function sortEvents(list: ClubEvent[], tab: EventTab): ClubEvent[] {
  const dir = tab === "past" ? -1 : 1;
  return [...list].sort((a, b) => {
    const ta = Date.parse(a.eventTime) || 0;
    const tb = Date.parse(b.eventTime) || 0;
    return (ta - tb) * dir;
  });
}

async function fetchEventsPage(clubId: string, tab: EventTab, after = ""): Promise<EventsPage> {
  const qs = new URLSearchParams({ first: String(eventsPageSize), lite: "1" });
  if (after) qs.set("after", after);
  if (tab === "past") qs.set("feed", "past");
  return api<EventsPage>(
    `/api/campfire/clubs/${encodeURIComponent(clubId)}/events?${qs}`,
    { headers: authHeaders() },
  );
}

async function loadEvents(clubId: string, opts?: { reset?: boolean }) {
  const tab = eventTab.value;
  if (!token.value || !clubId) {
    events.value = [];
    clubNameFromEvents.value = "";
    eventsCursor.value = "";
    eventsHasMore.value = false;
    return;
  }
  const key = eventsRequestKey(tab, clubId);
  if (eventsInflight && eventsInflightKey === key && opts?.reset) {
    return eventsInflight;
  }

  const generation = ++eventsGeneration;
  loadingEvents.value = true;
  error.value = "";
  if (opts?.reset) {
    events.value = [];
    eventsCursor.value = "";
    eventsHasMore.value = false;
  }
  eventsInflightKey = key;
  eventsInflight = (async () => {
    try {
      const res = await fetchEventsPage(clubId, tab);
      if (generation !== eventsGeneration) return;
      if (selectedClubId.value !== clubId || eventTab.value !== tab) return;
      clubNameFromEvents.value = (res.clubName || "").trim();
      events.value = sortEvents(mapEvents(res.events || [], clubId), tab);
      eventsCursor.value = res.hasNextPage ? res.endCursor || "" : "";
      eventsHasMore.value = !!res.hasNextPage && !!res.endCursor;
      pruneSelection();
    } catch (e: any) {
      if (generation !== eventsGeneration) return;
      if (selectedClubId.value !== clubId || eventTab.value !== tab) return;
      events.value = [];
      clubNameFromEvents.value = "";
      eventsCursor.value = "";
      eventsHasMore.value = false;
      selectedIds.value = new Set();
      error.value = e.message || "Failed to load events";
      if (isCampfireTokenError(e)) {
        invalidateToken();
        openSettings({ focusToken: true });
      }
    } finally {
      if (eventsInflightKey === key) {
        loadingEvents.value = false;
        eventsInflight = null;
        eventsInflightKey = "";
      }
    }
  })();
  return eventsInflight;
}

async function loadMoreEvents() {
  const clubId = selectedClubId.value;
  const tab = eventTab.value;
  const after = eventsCursor.value;
  const generation = eventsGeneration;
  if (!token.value || !clubId || !after || loadingMore.value || loadingEvents.value) return;

  loadingMore.value = true;
  error.value = "";
  try {
    const res = await fetchEventsPage(clubId, tab, after);
    if (generation !== eventsGeneration) return;
    if (selectedClubId.value !== clubId || eventTab.value !== tab) return;
    if (res.clubName) clubNameFromEvents.value = res.clubName.trim();
    const seen = new Set(events.value.map((e) => e.id));
    const more = sortEvents(mapEvents(res.events || [], clubId), tab).filter((ev) => !seen.has(ev.id));
    events.value = [...events.value, ...more];
    eventsCursor.value = res.hasNextPage ? res.endCursor || "" : "";
    eventsHasMore.value = !!res.hasNextPage && !!res.endCursor;
    pruneSelection();
  } catch (e: any) {
    if (generation !== eventsGeneration) return;
    if (selectedClubId.value !== clubId || eventTab.value !== tab) return;
    error.value = e.message || "Failed to load more events";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    loadingMore.value = false;
  }
}

const loadMoreSentinel = ref<HTMLElement | null>(null);
let loadMoreObserver: IntersectionObserver | null = null;

function attachLoadMoreObserver() {
  loadMoreObserver?.disconnect();
  loadMoreObserver = null;
  const el = loadMoreSentinel.value;
  if (!el) return;
  loadMoreObserver = new IntersectionObserver((entries) => {
    const entry = entries[0];
    if (!entry?.isIntersecting) return;
    void loadMoreEvents();
  });
  loadMoreObserver.observe(el);
}

watch(loadMoreSentinel, () => attachLoadMoreObserver());

const selectedClub = computed(
  () => clubs.value.find((c) => c.id === selectedClubId.value) || null,
);

const selectedClubLabel = computed(() => {
  const fromList = selectedClub.value?.name;
  return (fromList || clubNameFromEvents.value || "").trim();
});

function isOngoing(ev: ClubEvent, now = Date.now()): boolean {
  const start = Date.parse(ev.eventTime);
  if (!Number.isFinite(start) || start > now) return false;
  if (!ev.eventEndTime) return true;
  const end = Date.parse(ev.eventEndTime);
  return Number.isFinite(end) && end > now;
}

const eventRows = computed(() => {
  const now = Date.now();
  return events.value.map((ev) => {
    const name = (ev.name || "").trim() || "Untitled meetup";
    const cover = coverImageSrc(ev.coverPhotoUrl, apiBase as string);
    const when = formatInstant(ev.eventTime, timeZone.value, timeFormat.value);
    const end = ev.eventEndTime
      ? formatInstant(ev.eventEndTime, timeZone.value, timeFormat.value)
      : "";
    const creator = ev.creator?.displayName || ev.creator?.username || "";
    const ongoing = isOngoing(ev, now);
    return {
      ev,
      name,
      cover,
      initial: name.slice(0, 1).toUpperCase(),
      when,
      end,
      ongoing,
      hasLocation: parseCampfireLocation(ev.location) != null,
      hasLiveEvent: !!(ev.campfireLiveEventId || "").trim(),
      address: (ev.address || "").trim(),
      details: (ev.details || "").trim(),
      creator,
    };
  });
});

const detailEvent = computed(
  () => events.value.find((e) => e.id === detailEventId.value) || null,
);

const detailRow = computed(
  () => eventRows.value.find((r) => r.ev.id === detailEventId.value) || null,
);

const confirmDeleteEvent = computed(
  () => events.value.find((e) => e.id === confirmDeleteId.value) || null,
);

function eventAriaLabel(row: {
  name: string;
  ongoing: boolean;
  hasLocation: boolean;
  hasLiveEvent: boolean;
}) {
  const parts = [`Open ${row.name}`];
  if (row.ongoing) parts.push("ongoing");
  if (row.hasLocation) parts.push("has a map location");
  if (row.hasLiveEvent) parts.push("associated with a live event");
  return parts.join(", ");
}

function openDetail(ev: ClubEvent) {
  detailEventId.value = ev.id;
}

function closeDetail() {
  detailEventId.value = null;
}

function editEvent(ev: ClubEvent) {
  navigateTo({
    path: "/edit",
    query: {
      clubId: ev.clubId || selectedClubId.value,
      eventId: ev.id,
    },
  });
}

function pruneSelection() {
  const valid = new Set(events.value.map((ev) => ev.id));
  const next = new Set<string>();
  for (const id of selectedIds.value) {
    if (valid.has(id)) next.add(id);
  }
  selectedIds.value = next;
}

function isSelected(id: string) {
  return selectedIds.value.has(id);
}

function toggleSelected(id: string, on?: boolean) {
  const next = new Set(selectedIds.value);
  const enable = on ?? !next.has(id);
  if (enable) next.add(id);
  else next.delete(id);
  selectedIds.value = next;
}

const selectedCount = computed(() => selectedIds.value.size);

const allSelected = computed(
  () => events.value.length > 0 && events.value.every((ev) => selectedIds.value.has(ev.id)),
);

function toggleSelectAll() {
  if (allSelected.value) {
    selectedIds.value = new Set();
    return;
  }
  selectedIds.value = new Set(events.value.map((ev) => ev.id));
}

function selectedEventsInOrder(): ClubEvent[] {
  const ids = selectedIds.value;
  return events.value.filter((ev) => ids.has(ev.id));
}

function askDelete(id: string) {
  confirmBulk.value = false;
  confirmDeleteId.value = id;
}

function askDeleteSelected() {
  if (!selectedCount.value) {
    error.value = "Select at least one meetup to delete";
    return;
  }
  confirmDeleteId.value = null;
  confirmBulk.value = true;
}

function closeConfirm() {
  if (busy.value) return;
  confirmDeleteId.value = null;
  confirmBulk.value = false;
}

async function shareEvent(ev: ClubEvent) {
  if (!token.value || sharingId.value || sharingBulk.value) return;
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

async function fetchShareUrl(id: string): Promise<string> {
  const res = await api<{ url: string; shortId: string }>(
    `/api/campfire/meetups/${encodeURIComponent(id)}/share-link`,
    { method: "POST", headers: authHeaders() },
  );
  const url = (res.url || "").trim();
  if (!url) throw new Error("No share link returned");
  return url;
}

async function shareSelected() {
  const list = selectedEventsInOrder();
  if (!token.value || !list.length || sharingBulk.value || busy.value) return;
  sharingBulk.value = true;
  error.value = "";
  progress.value = "";
  const lines: string[] = [];
  const failed: string[] = [];
  try {
    for (let i = 0; i < list.length; i++) {
      const ev = list[i];
      const name = (ev.name || "").trim() || "Untitled meetup";
      progress.value = `Copying share links… ${i + 1}/${list.length}`;
      try {
        const url = await fetchShareUrl(ev.id);
        lines.push(`${name}: ${url}`);
      } catch (e: any) {
        failed.push(name);
        if (isCampfireTokenError(e)) {
          invalidateToken();
          break;
        }
      }
    }
    if (lines.length) {
      await navigator.clipboard.writeText(lines.join("\n"));
      toast.success(
        lines.length === 1 ? "Share link copied" : `${lines.length} share links copied`,
      );
    }
    if (failed.length) {
      error.value = lines.length
        ? `Copied ${lines.length}. Failed: ${failed.join(", ")}`
        : "Failed to copy share links";
    }
  } catch (e: any) {
    error.value = e.message || "Failed to copy share links";
  } finally {
    progress.value = "";
    sharingBulk.value = false;
  }
}

async function deleteSelected() {
  const list = selectedEventsInOrder();
  if (!token.value || !list.length) return;
  busy.value = true;
  error.value = "";
  const failed: string[] = [];
  let deleted = 0;
  try {
    for (let i = 0; i < list.length; i++) {
      const ev = list[i];
      const name = (ev.name || "").trim() || "Untitled meetup";
      progress.value = `Deleting meetups… ${i + 1}/${list.length}`;
      try {
        await api(`/api/campfire/meetups/${encodeURIComponent(ev.id)}`, {
          method: "DELETE",
          headers: authHeaders(),
        });
        events.value = events.value.filter((e) => e.id !== ev.id);
        const next = new Set(selectedIds.value);
        next.delete(ev.id);
        selectedIds.value = next;
        if (detailEventId.value === ev.id) detailEventId.value = null;
        deleted++;
      } catch (e: any) {
        failed.push(name);
        if (isCampfireTokenError(e)) {
          invalidateToken();
          break;
        }
      }
    }
    if (deleted) {
      toast.success(deleted === 1 ? "Meetup deleted" : `${deleted} meetups deleted`);
    }
    if (failed.length) {
      error.value = `Failed to delete: ${failed.join(", ")}`;
    }
  } finally {
    progress.value = "";
    busy.value = false;
  }
}

async function onConfirmDelete() {
  const bulk = confirmBulk.value;
  const id = confirmDeleteId.value;
  const name = confirmDeleteEvent.value?.name || "Meetup";
  confirmBulk.value = false;
  confirmDeleteId.value = null;
  if (bulk) {
    await deleteSelected();
    return;
  }
  if (!id || !token.value) return;
  busy.value = true;
  error.value = "";
  try {
    await api(`/api/campfire/meetups/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: authHeaders(),
    });
    toast.success("Meetup deleted", name);
    events.value = events.value.filter((e) => e.id !== id);
    const next = new Set(selectedIds.value);
    next.delete(id);
    selectedIds.value = next;
    if (detailEventId.value === id) detailEventId.value = null;
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
          <h2>Events{{ eventRows.length ? ` (${eventRows.length})` : "" }}</h2>
          <p class="muted">
            {{
              eventTab === "past"
                ? "Past meetups for clubs you admin."
                : "Upcoming and ongoing meetups for clubs you admin."
            }}
          </p>
        </div>
        <div class="page-head-end">
          <div class="page-actions">
            <button
              type="button"
              :disabled="busy || sharingBulk || !selectedCount || !token"
              @click="shareSelected"
            >
              <Icon name="share" />
              {{ sharingBulk ? "Copying…" : selectedCount ? `Share (${selectedCount})` : "Share" }}
            </button>
            <button
              v-if="eventTab === 'upcoming'"
              type="button"
              class="danger"
              :disabled="busy || sharingBulk || !selectedCount || !token"
              @click="askDeleteSelected"
            >
              <Icon name="trash" />
              {{ selectedCount ? `Delete (${selectedCount})` : "Delete" }}
            </button>
            <NuxtLink to="/create" class="btn primary create-action">
              <Icon name="plus" />
              Create
            </NuxtLink>
          </div>
          <CampfireIdentity />
        </div>
      </div>

      <div class="feed-bar">
        <div class="view-tabs" role="tablist" aria-label="Meetup feed">
          <button
            type="button"
            role="tab"
            :aria-selected="eventTab === 'upcoming'"
            :class="{ active: eventTab === 'upcoming' }"
            @click="setEventTab('upcoming')"
          >
            Upcoming
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="eventTab === 'past'"
            :class="{ active: eventTab === 'past' }"
            @click="setEventTab('past')"
          >
            Past
          </button>
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
      </div>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="progress" class="muted">{{ progress }}</p>
      <p v-else-if="loadingEvents && !eventRows.length" class="muted">Loading meetups…</p>

      <div v-if="eventRows.length" class="selection-bar">
        <label class="select-all">
          <input
            type="checkbox"
            :checked="allSelected"
            :disabled="busy || sharingBulk"
            :indeterminate.prop="selectedCount > 0 && !allSelected"
            @change="toggleSelectAll"
          />
          <span>{{ allSelected ? "Deselect all" : "Select all" }}</span>
        </label>
        <span v-if="selectedCount" class="muted">{{ selectedCount }} selected</span>
      </div>

      <ul v-if="eventRows.length" class="event-list">
        <li
          v-for="row in eventRows"
          :key="row.ev.id"
          class="event-row"
          :class="{ selected: isSelected(row.ev.id) }"
          role="button"
          tabindex="0"
          :aria-label="eventAriaLabel(row)"
          @click="openDetail(row.ev)"
          @keydown.enter.prevent="openDetail(row.ev)"
        >
          <label class="event-check" @click.stop @keydown.stop>
            <input
              type="checkbox"
              :checked="isSelected(row.ev.id)"
              :disabled="busy || sharingBulk"
              :aria-label="`Select ${row.name}`"
              @change="toggleSelected(row.ev.id)"
            />
          </label>
          <div class="event-cover" aria-hidden="true">
            <img v-if="row.cover" :src="row.cover" alt="" />
            <span v-else>{{ row.initial }}</span>
          </div>
          <div class="event-body">
            <strong class="event-name">
              {{ row.name }}
              <span
                v-if="row.hasLocation"
                class="event-flag"
                title="Map location"
              >
                <svg class="event-flag-pin" viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    fill="currentColor"
                    d="M12 2.2a7.2 7.2 0 0 0-7.2 7.2c0 5.4 7.2 12.4 7.2 12.4s7.2-7 7.2-12.4A7.2 7.2 0 0 0 12 2.2Zm0 9.6a2.4 2.4 0 1 1 0-4.8 2.4 2.4 0 0 1 0 4.8Z"
                  />
                </svg>
              </span>
              <img
                v-if="row.hasLiveEvent"
                class="event-flag-star"
                :src="caStarUrl"
                alt=""
                title="Associated with a live event"
              />
              <span v-if="row.ongoing" class="ongoing-badge">Ongoing</span>
            </strong>
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
              :disabled="busy || sharingBulk || !token || sharingId === row.ev.id"
              @click="shareEvent(row.ev)"
            >
              <Icon name="share" />
            </button>
            <button
              v-if="eventTab === 'upcoming'"
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
              v-if="eventTab === 'upcoming'"
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
      <div v-if="eventsHasMore" ref="loadMoreSentinel" class="load-more-sentinel" aria-hidden="true" />
      <p v-if="loadingMore" class="muted load-more-status">Loading more…</p>
      <p v-if="!eventsHasMore && selectedClubId && !loadingEvents && !eventRows.length" class="muted">
        {{ eventTab === "past" ? "No past meetups in this club." : "No upcoming meetups in this club." }}
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
            <h2 id="event-detail-title" class="detail-title">
              {{ detailRow.name }}
              <span
                v-if="detailRow.hasLocation"
                class="event-flag"
                title="Map location"
                role="img"
                aria-label="Map location"
              >
                <svg class="event-flag-pin" viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    fill="currentColor"
                    d="M12 2.2a7.2 7.2 0 0 0-7.2 7.2c0 5.4 7.2 12.4 7.2 12.4s7.2-7 7.2-12.4A7.2 7.2 0 0 0 12 2.2Zm0 9.6a2.4 2.4 0 1 1 0-4.8 2.4 2.4 0 0 1 0 4.8Z"
                  />
                </svg>
              </span>
              <img
                v-if="detailRow.hasLiveEvent"
                class="event-flag-star"
                :src="caStarUrl"
                alt="Associated with a live event"
                title="Associated with a live event"
              />
            </h2>
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
          <button
            v-if="eventTab === 'upcoming'"
            type="button"
            :disabled="busy"
            @click="editEvent(detailEvent)"
          >
            <Icon name="edit" />
            Edit
          </button>
          <button
            v-if="eventTab === 'upcoming'"
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
      :open="confirmDeleteId != null || confirmBulk"
      :title="confirmBulk ? 'Delete meetups?' : 'Delete meetup?'"
      :message="
        confirmBulk
          ? `Delete ${selectedCount} meetup${selectedCount === 1 ? '' : 's'} from Campfire? This cannot be undone.`
          : confirmDeleteEvent
            ? `Delete “${confirmDeleteEvent.name || 'this meetup'}” from Campfire? This cannot be undone.`
            : 'Delete this meetup from Campfire?'
      "
      :confirm-label="confirmBulk ? `Delete ${selectedCount}` : 'Delete'"
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
.page-head-end {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  margin-left: auto;
  flex-wrap: wrap;
}
.page-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
  align-items: center;
}
.page-actions .create-action {
  margin-left: 0.25rem;
}
.feed-bar {
  display: flex;
  align-items: flex-end;
  gap: 0.85rem;
  margin-bottom: 1rem;
}
.view-tabs {
  display: inline-flex;
  flex-shrink: 0;
  gap: 0.25rem;
  margin: 0;
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
  grid-template-columns: minmax(0, 20rem);
  gap: 0.75rem;
  margin: 0;
  flex: 1;
  min-width: 0;
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
.event-check {
  display: grid;
  place-items: center;
  cursor: pointer;
}
.event-check input {
  width: 1.05rem;
  height: 1.05rem;
  accent-color: var(--accent);
}
.load-more-sentinel {
  height: 1px;
}
.load-more-status {
  margin: 0.75rem 0 0;
  text-align: center;
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
  grid-template-columns: auto auto 1fr auto;
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
.event-row.selected {
  border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
  background: color-mix(in srgb, var(--accent) 8%, var(--bg-elevated));
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
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
}
.event-flag {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  line-height: 0;
}
.event-flag-pin {
  width: 1rem;
  height: 1rem;
  display: block;
  color: var(--text);
}
.event-flag-star {
  width: 1rem;
  height: 1rem;
  display: block;
  object-fit: contain;
  flex-shrink: 0;
}
.ongoing-badge {
  font-size: 0.7rem;
  font-weight: 600;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  border: 1px solid var(--accent);
  color: var(--accent);
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
.detail-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
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
  .feed-bar {
    flex-wrap: wrap;
  }
  .filters {
    flex-basis: 12rem;
    grid-template-columns: 1fr;
  }
  .event-row {
    grid-template-columns: auto auto 1fr;
    grid-template-areas:
      "check cover body"
      "actions actions actions";
  }
  .event-check {
    grid-area: check;
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
