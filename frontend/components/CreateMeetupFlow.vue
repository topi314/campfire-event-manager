<script setup lang="ts">
import type {
  Club,
  ClubEvent,
  EditMeetupInput,
  LiveEvent,
  MeetupPayload,
  MeetupDraftItem,
  MeetupTemplate,
  DraftMeetupPayload,
} from "~/types";
import type { LatLngPoint } from "~/components/LocationPicker.vue";
import {
  addMinutesToLocal,
  calendarDateFromLiveEvent,
  combineDayAndTime,
  formatLocalDateTime,
  formatLocalDateTimeParts,
  parseCalendarDate,
  parseLocalDateTime,
  parseTimeOfDay,
  partsInTimeZone,
  toISO,
  type CalendarDate,
} from "~/utils/datetime";
import {
  applyPlaceholders,
  humanizePlaceholderKey,
  isBuiltinPlaceholder,
  normalizePlaceholderKey,
  resolveBuiltinPlaceholderValues,
  resolveCustomPlaceholderDefs,
  type PlaceholderDef,
} from "~/utils/placeholders";
import { DEFAULT_LOCATION_JITTER_METERS, parseCampfireLocation } from "~/utils/location";
import { categoryFromLiveEventName, categoryMatchOrder } from "~/utils/eventCategory";
import { buildMeetupBodyFromDraft, parseDraftPayload } from "~/utils/drafts";
import { isCampfireTokenError } from "~/composables/useSessionToken";

const props = defineProps<{
  /** When set, load this club draft into the create form. */
  draftId?: number | null;
}>();

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { token, authHeaders, openSettings, invalidateToken } = useSessionToken();
const { me: campfireMe, validateToken } = useCampfireSession();
const { timeZone, preferredClubId } = usePreferences();
const { setFlash } = useFlash();

type FlowMode = "create" | "edit";

const mode = ref<FlowMode>("create");
const templates = ref<MeetupTemplate[]>([]);
const clubs = ref<Club[]>([]);
const liveEvents = ref<LiveEvent[]>([]);
const clubEvents = ref<ClubEvent[]>([]);
const campfireName = ref("");
const error = ref("");
const success = ref("");
const creating = ref(false);
const confirmCreateOpen = ref(false);
const loadingEvents = ref(false);
const clubEventsInflight = ref<Promise<void> | null>(null);
const clubEventsLoadedFor = ref("");
const drafts = ref<MeetupDraftItem[]>([]);
const draftsBusy = ref(false);
const editingDraftId = ref<number | null>(null);

const selectedClubId = ref("");
const selectedLiveEventId = ref("");
const selectedTemplateId = ref<number | "">("");
const selectedEventId = ref("");
const originalCoverPhotoUrl = ref("");
const placeholderValues = reactive<Record<string, string>>({});

const draft = reactive({
  name: "",
  details: "",
  eventTime: "",
  eventEndTime: "",
  address: "",
  coverPhotoUrl: "",
  commentsPermissions: "ORGANIZERS_ONLY",
  allInvited: true,
  createdByCommunityAmbassador: true,
  locationJitterMeters: DEFAULT_LOCATION_JITTER_METERS,
  campfireLiveEventId: "",
});
const draftLocation = ref<LatLngPoint | null>(null);
const draftReady = ref(false);
/** True once the user changes template or meetup fields after auto-fill. */
const detailsTouched = ref(false);
/** Suppress detailsTouched while applying live-event / template selections. */
let programmaticUpdate = false;

function beginProgrammaticUpdate() {
  programmaticUpdate = true;
}

function endProgrammaticUpdate() {
  nextTick(() => {
    programmaticUpdate = false;
  });
}

function markDetailsTouched() {
  if (!programmaticUpdate) detailsTouched.value = true;
}

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  await Promise.all([loadTemplates(), loadDrafts()]);
  if (token.value) await loadCampfire();
  await applyDraftRoute();
});

watch(token, async (t) => {
  if (t) await loadCampfire();
});

watch(
  () => props.draftId,
  async (id, prev) => {
    if (!user.value) return;
    if (id == null) {
      if (editingDraftId.value != null) {
        editingDraftId.value = null;
        resetFormAfterDraft();
      }
      return;
    }
    if (id !== prev) await applyDraftRoute();
  },
);

async function loadTemplates() {
  try {
    templates.value = await api<MeetupTemplate[]>("/api/templates");
  } catch (e: any) {
    error.value = e.message || "Failed to load templates";
  }
}

async function loadDrafts() {
  if (!token.value) {
    drafts.value = [];
    return;
  }
  try {
    drafts.value = await api<MeetupDraftItem[]>("/api/drafts", {
      headers: authHeaders(),
    });
  } catch (e: any) {
    console.warn("drafts:", e.message);
    drafts.value = [];
  }
}


async function loadCampfire() {
  error.value = "";
  try {
    const ok = await validateToken({ openSettingsOnFail: false });
    if (!ok || !campfireMe.value) {
      campfireName.value = "";
      clubs.value = [];
      liveEvents.value = [];
      error.value = "Campfire token invalid";
      openSettings({ focusToken: true });
      return;
    }
    campfireName.value = campfireMe.value.displayName || campfireMe.value.username;
    clubs.value = await api<Club[]>("/api/campfire/clubs", { headers: authHeaders() });
    try {
      liveEvents.value = await api<LiveEvent[]>("/api/campfire/live-events", {
        headers: authHeaders(),
      });
    } catch (e: any) {
      liveEvents.value = [];
      console.warn("live events:", e.message);
    }
    applyDefaultSelections();
  } catch (e: any) {
    campfireName.value = "";
    clubs.value = [];
    liveEvents.value = [];
    error.value = e.message || "Campfire token invalid";
    if (isCampfireTokenError(e)) invalidateToken();
  }
}

function applyDefaultSelections() {
  if (clubs.value.length) {
    const clubOk = clubs.value.some((c) => c.id === selectedClubId.value);
    if (!clubOk) {
      const preferred = preferredClubId.value;
      const preferredOk = !!preferred && clubs.value.some((c) => c.id === preferred);
      // Watcher on selectedClubId loads club events.
      selectedClubId.value = preferredOk ? preferred : clubs.value[0].id;
    } else {
      void loadClubEvents();
    }
  } else {
    selectedClubId.value = "";
    clubEvents.value = [];
  }
  if (
    selectedLiveEventId.value &&
    !liveEvents.value.some((e) => e.id === selectedLiveEventId.value)
  ) {
    selectedLiveEventId.value = "";
  }
}

function parsePayload(t: MeetupTemplate): MeetupPayload {
  return (typeof t.payload === "string" ? JSON.parse(t.payload) : t.payload) as MeetupPayload;
}

function templateCategory(t: MeetupTemplate): string {
  return (parsePayload(t).category || "").trim();
}

const selectedClub = computed(() =>
  clubs.value.find((c) => c.id === selectedClubId.value) || null,
);

const selectedLiveEvent = computed(() =>
  liveEvents.value.find((e) => e.id === selectedLiveEventId.value) || null,
);

const liveEventCategory = computed(() =>
  selectedLiveEvent.value
    ? categoryFromLiveEventName(selectedLiveEvent.value.eventName)
    : "",
);

/** Templates: prefer category match when a live event is selected; always list all. */
const filteredTemplates = computed(() => {
  if (!selectedLiveEvent.value) return templates.value;
  const cat = liveEventCategory.value;
  const order = categoryMatchOrder(cat);
  const hasExact = templates.value.some((t) => {
    const tc = templateCategory(t);
    return !!tc && tc === cat;
  });
  const preferred = new Set(
    hasExact ? [cat] : order,
  );
  const matching = templates.value.filter((t) => {
    const tc = templateCategory(t);
    return !tc || preferred.has(tc);
  });
  // Keep full list selectable; matching first for convenience.
  const matchIds = new Set(matching.map((t) => t.id));
  const rest = templates.value.filter((t) => !matchIds.has(t.id));
  return [...matching, ...rest];
});

const selectedTemplate = computed(() => {
  const id = Number(selectedTemplateId.value);
  if (!id) return null;
  return templates.value.find((t) => t.id === id) || null;
});

const rawPayload = computed(() =>
  selectedTemplate.value ? parsePayload(selectedTemplate.value) : null,
);

const placeholderDefs = computed<PlaceholderDef[]>(() => {
  if (rawPayload.value) {
    return resolveCustomPlaceholderDefs(
      rawPayload.value as Record<string, unknown>,
      rawPayload.value.placeholders,
    );
  }
  // No template selected (e.g. draft whose template was deleted): still offer
  // custom tokens found in the current meetup text / saved fills.
  const fromText = resolveCustomPlaceholderDefs({
    name: draft.name,
    details: draft.details,
    address: draft.address,
    coverPhotoUrl: draft.coverPhotoUrl,
  });
  const byNorm = new Map(
    fromText.map((d) => [normalizePlaceholderKey(d.key), d] as const),
  );
  for (const k of Object.keys(placeholderValues)) {
    const norm = normalizePlaceholderKey(k);
    if (!norm || byNorm.has(norm) || isBuiltinPlaceholder(k)) continue;
    byNorm.set(norm, { key: k });
  }
  return [...byNorm.values()];
});

const selectionReady = computed(() => placeholderDefs.value.length > 0);

/** Edit keeps {{tokens}} visible; preview shows resolved copy. */
const textViewMode = ref<"edit" | "preview">("edit");

function clockFromLocalInput(local: string): string {
  const parts = parseLocalDateTime(local);
  if (!parts) return "";
  return `${String(parts.hours).padStart(2, "0")}:${String(parts.minutes).padStart(2, "0")}`;
}

function builtinPlaceholderContext() {
  const p = rawPayload.value;
  return {
    clubName: selectedClub.value?.name || "",
    liveEventName: selectedLiveEvent.value?.eventName || "",
    category: liveEventCategory.value || p?.category || "",
    date: meetupCalendarDay(),
    startTime: clockFromLocalInput(draft.eventTime) || p?.startTime || "",
    endTime: clockFromLocalInput(draft.eventEndTime) || p?.endTime || "",
    timeZone: timeZone.value,
    title: draft.name || "",
    address: draft.address || "",
    latitude: draftLocation.value?.lat ?? null,
    longitude: draftLocation.value?.lng ?? null,
  };
}

function currentPlaceholderMap(): Record<string, string> {
  const values = resolveBuiltinPlaceholderValues(builtinPlaceholderContext());
  for (const d of placeholderDefs.value) {
    const raw = (placeholderValues[d.key] ?? "").trim();
    if (raw) values[d.key] = raw;
    else if (d.default?.trim()) values[d.key] = d.default.trim();
  }
  return values;
}

/** Resolve tokens in the editable draft text fields. */
function resolvedDraftText() {
  const map = currentPlaceholderMap();
  return {
    name: applyPlaceholders(draft.name, map),
    details: applyPlaceholders(draft.details, map),
    address: applyPlaceholders(draft.address, map),
    coverPhotoUrl: applyPlaceholders(draft.coverPhotoUrl, map),
  };
}

const previewText = computed(() => resolvedDraftText());

function timesFromPayload(p: MeetupPayload, day: CalendarDate) {
  const startTod =
    parseTimeOfDay(p.startTime) ||
    parseTimeOfDay(p.eventTime) ||
    { hours: 14, minutes: 0 };
  const endTod = parseTimeOfDay(p.endTime) || parseTimeOfDay(p.eventEndTime);

  const start = combineDayAndTime(day, startTod);
  let end = "";
  if (endTod) {
    end = combineDayAndTime(day, endTod);
    if (fromLocalOrIso(end) <= fromLocalOrIso(start)) {
      end = addMinutesToLocal(end, 24 * 60);
    }
  }
  return { start, end };
}

function fromLocalOrIso(value: string): number {
  const parts = parseLocalDateTime(value);
  if (parts) {
    return new Date(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hours,
      parts.minutes,
      parts.seconds || 0,
    ).getTime();
  }
  return new Date(value).getTime();
}

function todayCalendarDate(): CalendarDate {
  const parts = partsInTimeZone(new Date(), timeZone.value);
  if (parts) {
    return { year: parts.year, month: parts.month, day: parts.day };
  }
  const d = new Date();
  return { year: d.getFullYear(), month: d.getMonth() + 1, day: d.getDate() };
}

function meetupCalendarDay(): CalendarDate {
  const ev = selectedLiveEvent.value;
  return ev ? calendarDateFromLiveEvent(ev, timeZone.value) : todayCalendarDate();
}

function liveEventDefaultTimes(ev: LiveEvent): { start: string; end: string } {
  const tz = timeZone.value;
  const day = calendarDateFromLiveEvent(ev, tz);
  const startTod =
    parseTimeOfDay(ev.localStartTime) ||
    parseTimeOfDay(ev.startTimestamp, tz) ||
    { hours: 0, minutes: 0 };
  const endTod =
    parseTimeOfDay(ev.localEndTime) || parseTimeOfDay(ev.endTimestamp, tz);
  const endDay =
    parseCalendarDate(ev.localEndTime) ||
    parseCalendarDate(ev.endTimestamp) ||
    day;

  const start = combineDayAndTime(day, startTod);
  let end = "";
  if (endTod) {
    end = combineDayAndTime(endDay, endTod);
    if (fromLocalOrIso(end) <= fromLocalOrIso(start)) {
      end = addMinutesToLocal(end, 24 * 60);
    }
  }
  return { start, end };
}

function resetPlaceholders() {
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  for (const d of placeholderDefs.value) {
    placeholderValues[d.key] = "";
  }
}

function blankDraftStandalone() {
  draft.name = "";
  draft.details = "";
  draft.address = "";
  draft.coverPhotoUrl = "";
  draft.commentsPermissions = "ORGANIZERS_ONLY";
  draft.allInvited = true;
  draft.createdByCommunityAmbassador = true;
  const day = todayCalendarDate();
  draft.eventTime = combineDayAndTime(day, { hours: 14, minutes: 0 });
  draft.eventEndTime = combineDayAndTime(day, { hours: 17, minutes: 0 });
  draft.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
  draft.campfireLiveEventId = "";
  draftLocation.value = null;
}

function blankDraftFromLiveEvent(ev: LiveEvent) {
  draft.name = "";
  draft.details = "";
  draft.address = "";
  draft.coverPhotoUrl = "";
  draft.commentsPermissions = "ORGANIZERS_ONLY";
  draft.allInvited = true;
  draft.createdByCommunityAmbassador = true;
  const times = liveEventDefaultTimes(ev);
  draft.eventTime = times.start;
  draft.eventEndTime = times.end;
  draft.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
  draft.campfireLiveEventId = ev.id;
  draftLocation.value = null;
}

function blankDraftForSelection() {
  if (selectedLiveEvent.value) blankDraftFromLiveEvent(selectedLiveEvent.value);
  else blankDraftStandalone();
}

function hydrateDraft() {
  if (!selectedClub.value) {
    draftReady.value = false;
    draftLocation.value = null;
    return;
  }

  const ev = selectedLiveEvent.value;
  const day = meetupCalendarDay();
  const raw = rawPayload.value;
  if (selectedTemplate.value && raw) {
    const times = timesFromPayload(raw, day);
    // Load raw template copy so {{placeholders}} stay visible until Preview / submit.
    draft.name = raw.name || "";
    draft.details = raw.details || "";
    draft.address = raw.address || "";
    draft.coverPhotoUrl = raw.coverPhotoUrl || "";
    draft.commentsPermissions = raw.commentsPermissions || "ORGANIZERS_ONLY";
    draft.allInvited = raw.allInvited ?? true;
    draft.createdByCommunityAmbassador = raw.createdByCommunityAmbassador ?? true;
    draft.eventTime = times.start;
    draft.eventEndTime = times.end;
    draft.campfireLiveEventId = ev?.id || "";
    draft.locationJitterMeters =
      raw.locationJitterMeters != null
        ? raw.locationJitterMeters
        : DEFAULT_LOCATION_JITTER_METERS;

    if (raw.latitude != null && raw.longitude != null) {
      draftLocation.value = { lat: raw.latitude, lng: raw.longitude };
    } else {
      draftLocation.value = null;
    }

    textViewMode.value = "edit";
    draftReady.value = true;
    return;
  }

  if (!draftReady.value) {
    blankDraftForSelection();
  } else if (ev) {
    const times = liveEventDefaultTimes(ev);
    draft.eventTime = times.start;
    draft.eventEndTime = times.end;
    draft.campfireLiveEventId = ev.id;
  } else {
    draft.campfireLiveEventId = "";
  }
  draftReady.value = true;
}

function autoSelectTemplate(force = false) {
  if (selectedTemplateId.value && !force) return;
  const cat = liveEventCategory.value;
  let next: number | "" = "";
  if (cat) {
    for (const want of categoryMatchOrder(cat)) {
      const hit = templates.value.find((t) => templateCategory(t) === want);
      if (hit) {
        next = hit.id;
        break;
      }
    }
  }
  selectedTemplateId.value = next;
}

/** Apply selected template onto the meetup being edited (keeps its calendar day). */
function applyTemplateToEdit() {
  const raw = rawPayload.value;
  if (!raw || !selectedEventId.value || !draftReady.value) return;

  const day = draft.eventTime
    ? (() => {
        const parts = parseLocalDateTime(draft.eventTime);
        return parts
          ? { year: parts.year, month: parts.month, day: parts.day }
          : meetupCalendarDay();
      })()
    : meetupCalendarDay();
  const times = timesFromPayload(raw, day);

  draft.name = raw.name || "";
  draft.details = raw.details || "";
  draft.address = raw.address || "";
  draft.coverPhotoUrl = raw.coverPhotoUrl || "";
  draft.commentsPermissions = raw.commentsPermissions || "ORGANIZERS_ONLY";
  draft.allInvited = raw.allInvited ?? true;
  draft.createdByCommunityAmbassador = raw.createdByCommunityAmbassador ?? true;
  draft.eventTime = times.start;
  draft.eventEndTime = times.end;
  if (raw.latitude != null && raw.longitude != null) {
    draftLocation.value = { lat: raw.latitude, lng: raw.longitude };
  }
  textViewMode.value = "edit";
}

watch(selectedClubId, () => {
  if (mode.value === "edit") {
    selectedEventId.value = "";
    selectedTemplateId.value = "";
    for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
    clearEditDraft();
    void loadClubEvents();
    return;
  }
  void loadClubEvents();
  if (editingDraftId.value != null) return;
  detailsTouched.value = false;
  beginProgrammaticUpdate();
  hydrateDraft();
  endProgrammaticUpdate();
});

watch(preferredClubId, (preferred) => {
  if (!preferred || !clubs.value.length) return;
  if (!clubs.value.some((c) => c.id === preferred)) return;
  if (selectedClubId.value === preferred) return;
  selectedClubId.value = preferred;
});

watch(selectedLiveEventId, () => {
  if (mode.value === "edit" || editingDraftId.value != null) return;

  // Until the user edits, keep template in sync with the live event category.
  if (!detailsTouched.value) {
    beginProgrammaticUpdate();
    if (selectedLiveEvent.value) {
      autoSelectTemplate(true);
    } else {
      selectedTemplateId.value = "";
    }
    // Re-apply even when the template id is unchanged (new event times / name).
    hydrateDraft();
    endProgrammaticUpdate();
    return;
  }

  beginProgrammaticUpdate();
  hydrateDraft();
  endProgrammaticUpdate();
});

watch(selectedTemplateId, (id) => {
  // Draft / live-event / club restores set the id under programmaticUpdate and
  // hydrate themselves — don't wipe the form by re-applying the template.
  if (programmaticUpdate) return;
  markDetailsTouched();
  resetPlaceholders();
  textViewMode.value = "edit";
  if (mode.value === "edit") {
    if (id && selectedEventId.value && draftReady.value) {
      applyTemplateToEdit();
    }
    return;
  }
  if (!id) {
    if (selectedClub.value) {
      blankDraftForSelection();
      draftReady.value = true;
    } else {
      draftReady.value = false;
      draftLocation.value = null;
    }
    return;
  }
  hydrateDraft();
});

watch(
  () => [
    draft.name,
    draft.details,
    draft.address,
    draft.coverPhotoUrl,
    draft.eventTime,
    draft.eventEndTime,
    draft.commentsPermissions,
    draft.allInvited,
    draft.createdByCommunityAmbassador,
    draft.locationJitterMeters,
    draftLocation.value?.lat,
    draftLocation.value?.lng,
    JSON.stringify(placeholderValues),
  ],
  () => markDetailsTouched(),
  { flush: "sync" },
);

watch(selectedEventId, (id) => {
  if (mode.value !== "edit") return;
  selectedTemplateId.value = "";
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  if (!id) {
    clearEditDraft();
    return;
  }
  const ev = clubEvents.value.find((e) => e.id === id);
  if (ev) loadDraftFromClubEvent(ev);
});

watch(mode, (next, prev) => {
  if (next === prev) return;
  error.value = "";
  success.value = "";
  editingDraftId.value = null;
  selectedEventId.value = "";
  detailsTouched.value = false;
  beginProgrammaticUpdate();
  selectedTemplateId.value = "";
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  clearEditDraft();
  void loadClubEvents();
  if (next === "create" && selectedClub.value) {
    hydrateDraft();
  }
  endProgrammaticUpdate();
});

const canCreate = computed(
  () =>
    mode.value === "create" &&
    !!token.value &&
    draftReady.value &&
    !!selectedClubId.value &&
    !!draftLocation.value &&
    !!draft.eventTime &&
    !!draft.name.trim(),
);

const canSaveDraft = computed(
  () => canCreate.value && !!selectedClub.value?.amIAdmin,
);

const canSaveEdit = computed(
  () =>
    mode.value === "edit" &&
    !!token.value &&
    draftReady.value &&
    !!selectedEventId.value &&
    !!draftLocation.value &&
    !!draft.eventTime &&
    !!draft.name.trim(),
);

function validateDraft(): string | null {
  if (!token.value) return "missing-token";
  if (!selectedClubId.value) return "Pick a club";
  if (mode.value === "edit") {
    if (!selectedEventId.value) return "Pick a meetup to edit";
  }
  if (!draftLocation.value) return "Set a location";
  if (!draft.name.trim()) return "Title is required";
  if (!draft.eventTime) return "Start time is required";
  return null;
}

function isoToLocalInput(iso: string | undefined | null): string {
  if (!iso?.trim()) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const parts = partsInTimeZone(d, timeZone.value);
  return parts ? formatLocalDateTimeParts(parts) : "";
}

function clearEditDraft() {
  draftReady.value = false;
  draftLocation.value = null;
  originalCoverPhotoUrl.value = "";
  draft.name = "";
  draft.details = "";
  draft.eventTime = "";
  draft.eventEndTime = "";
  draft.address = "";
  draft.coverPhotoUrl = "";
  draft.commentsPermissions = "ORGANIZERS_ONLY";
  draft.allInvited = true;
  draft.createdByCommunityAmbassador = true;
  draft.campfireLiveEventId = "";
  draft.locationJitterMeters = 0;
}

function onPlaceSelected(place: { lat: number; lng: number; label?: string }) {
  if (place.label) draft.address = place.label;
}

async function loadClubEvents(opts?: { force?: boolean }) {
  const clubId = selectedClubId.value;
  if (!token.value || !clubId) {
    clubEvents.value = [];
    clubEventsLoadedFor.value = "";
    return;
  }
  if (!opts?.force && clubEventsLoadedFor.value === clubId && !loadingEvents.value) {
    return;
  }
  if (clubEventsInflight.value && clubEventsLoadedFor.value === clubId) {
    return clubEventsInflight.value;
  }

  loadingEvents.value = true;
  clubEventsLoadedFor.value = clubId;
  const run = (async () => {
    try {
      const res = await api<{ events: ClubEvent[] }>(
        `/api/campfire/clubs/${encodeURIComponent(clubId)}/events?first=100`,
        { headers: authHeaders() },
      );
      if (selectedClubId.value !== clubId) return;
      clubEvents.value = res.events || [];
    } catch (e: any) {
      if (selectedClubId.value !== clubId) return;
      if (mode.value === "edit") {
        error.value = e.message || "Failed to load club meetups";
      } else {
        console.warn("club events:", e.message);
      }
      if (isCampfireTokenError(e)) invalidateToken();
      clubEvents.value = [];
      clubEventsLoadedFor.value = "";
    } finally {
      if (selectedClubId.value === clubId) loadingEvents.value = false;
      clubEventsInflight.value = null;
    }
  })();
  clubEventsInflight.value = run;
  return run;
}

/** Live event IDs that already have an upcoming meetup in the selected club. */
const liveEventIdsWithMeetup = computed(() => {
  const ids = new Set<string>();
  for (const ev of clubEvents.value) {
    const id = (ev.campfireLiveEventId || "").trim();
    if (id) ids.add(id);
  }
  return ids;
});

/** Live event IDs that already have a draft in the selected club. */
const liveEventIdsWithDraft = computed(() => {
  const clubId = selectedClubId.value;
  const ids = new Set<string>();
  if (!clubId) return ids;
  for (const item of drafts.value) {
    if (item.clubId !== clubId) continue;
    const id = (parseDraftPayload(item).liveEventId || "").trim();
    if (id) ids.add(id);
  }
  return ids;
});

function liveEventOptionLabel(ev: LiveEvent) {
  const hasMeetup = liveEventIdsWithMeetup.value.has(ev.id);
  const hasDraft = liveEventIdsWithDraft.value.has(ev.id);
  if (hasMeetup && hasDraft) return `${ev.eventName} · meetup + draft`;
  if (hasMeetup) return `${ev.eventName} · already has meetup`;
  if (hasDraft) return `${ev.eventName} · has draft`;
  return ev.eventName;
}

function loadDraftFromClubEvent(ev: ClubEvent) {
  draft.name = ev.name || "";
  draft.details = ev.details || "";
  draft.eventTime = isoToLocalInput(ev.eventTime);
  draft.eventEndTime = isoToLocalInput(ev.eventEndTime);
  draft.address = ev.address || "";
  draft.coverPhotoUrl = ev.coverPhotoUrl || "";
  originalCoverPhotoUrl.value = ev.coverPhotoUrl || "";
  draft.commentsPermissions = ev.commentsPermissions || "ORGANIZERS_ONLY";
  draft.allInvited = ev.allInvited ?? true;
  draft.createdByCommunityAmbassador = ev.createdByCommunityAmbassador ?? true;
  draft.campfireLiveEventId = ev.campfireLiveEventId || "";
  draft.locationJitterMeters = 0;
  draftLocation.value = parseCampfireLocation(ev.location);
  if (ev.campfireLiveEventId && ev.campfireLiveEventId !== selectedLiveEventId.value) {
    selectedLiveEventId.value = ev.campfireLiveEventId;
  }
  draftReady.value = true;
}

function clubEventOptionLabel(ev: ClubEvent) {
  const when = ev.eventTime ? formatLocalDateTime(isoToLocalInput(ev.eventTime)) : "";
  return when && when !== "—" ? `${ev.name} · ${when}` : ev.name;
}

function buildEditBody(): EditMeetupInput {
  const cover = draft.coverPhotoUrl || "";
  const photoChanged = cover !== (originalCoverPhotoUrl.value || "");
  const customValues: Record<string, string> = {};
  for (const d of placeholderDefs.value) {
    const raw = (placeholderValues[d.key] ?? "").trim();
    const fallback = (d.default || "").trim();
    if (raw || fallback) customValues[d.key] = raw || fallback;
  }
  return {
    name: draft.name.trim(),
    details: draft.details,
    eventTime: toISO(draft.eventTime, timeZone.value),
    eventEndTime: draft.eventEndTime ? toISO(draft.eventEndTime, timeZone.value) : undefined,
    latitude: draftLocation.value!.lat,
    longitude: draftLocation.value!.lng,
    address: draft.address || undefined,
    coverPhotoUrl: photoChanged ? cover || undefined : undefined,
    commentsPermissions: draft.commentsPermissions || undefined,
    campfireLiveEventId: draft.campfireLiveEventId || selectedLiveEventId.value || undefined,
    allInvited: draft.allInvited,
    createdByCommunityAmbassador: draft.createdByCommunityAmbassador,
    hasEventPhotoChanged: photoChanged,
    clubName: selectedClub.value?.name || undefined,
    liveEventName: selectedLiveEvent.value?.eventName || undefined,
    category: liveEventCategory.value || rawPayload.value?.category || undefined,
    timeZone: timeZone.value,
    ...(Object.keys(customValues).length ? { placeholderValues: customValues } : {}),
  };
}

function buildDraftPayload(): DraftMeetupPayload {
  const customValues: Record<string, string> = {};
  for (const d of placeholderDefs.value) {
    const raw = (placeholderValues[d.key] ?? "").trim();
    const fallback = (d.default || "").trim();
    if (raw || fallback) customValues[d.key] = raw || fallback;
  }
  return {
    templateName: selectedTemplate.value?.name || "Custom",
    ...(selectedTemplate.value ? { templateId: selectedTemplate.value.id } : {}),
    clubId: selectedClubId.value,
    clubName: selectedClub.value?.name || "",
    liveEventId: selectedLiveEventId.value,
    liveEventName: selectedLiveEvent.value?.eventName || "",
    category: liveEventCategory.value || rawPayload.value?.category || "",
    // Keep {{tokens}} in stored draft text; filled only when posting.
    name: draft.name.trim(),
    details: draft.details,
    eventTime: draft.eventTime,
    eventEndTime: draft.eventEndTime,
    address: draft.address,
    coverPhotoUrl: draft.coverPhotoUrl,
    commentsPermissions: draft.commentsPermissions,
    allInvited: draft.allInvited,
    createdByCommunityAmbassador: draft.createdByCommunityAmbassador,
    locationJitterMeters: draft.locationJitterMeters,
    location: { ...draftLocation.value! },
    ...(Object.keys(customValues).length ? { placeholderValues: customValues } : {}),
  };
}


function resetFormAfterDraft() {
  detailsTouched.value = false;
  beginProgrammaticUpdate();
  selectedTemplateId.value = "";
  if (selectedClub.value) {
    blankDraftForSelection();
    draftReady.value = true;
  } else {
    draftReady.value = false;
  }
  nextTick(() => {
    autoSelectTemplate();
    endProgrammaticUpdate();
  });
}

function resolveDraftTemplateId(p: DraftMeetupPayload): number | "" {
  if (p.templateId != null && Number.isFinite(p.templateId) && p.templateId > 0) {
    const byId = templates.value.find((t) => t.id === p.templateId);
    if (byId) return byId.id;
  }
  const name = (p.templateName || "").trim();
  if (name && name !== "Custom") {
    const byName = templates.value.find((t) => t.name === name);
    if (byName) return byName.id;
  }
  return "";
}

function loadDraftFromPayload(p: DraftMeetupPayload) {
  if (mode.value !== "create") mode.value = "create";
  detailsTouched.value = true;
  beginProgrammaticUpdate();
  draft.name = p.name;
  draft.details = p.details;
  draft.eventTime = p.eventTime;
  draft.eventEndTime = p.eventEndTime;
  draft.address = p.address;
  draft.coverPhotoUrl = p.coverPhotoUrl;
  draft.commentsPermissions = p.commentsPermissions;
  draft.allInvited = p.allInvited;
  draft.createdByCommunityAmbassador = p.createdByCommunityAmbassador ?? true;
  draft.locationJitterMeters = p.locationJitterMeters;
  draft.campfireLiveEventId = p.liveEventId || "";
  draftLocation.value = { ...p.location };
  if (p.clubId) selectedClubId.value = p.clubId;
  selectedLiveEventId.value = p.liveEventId || "";
  selectedTemplateId.value = resolveDraftTemplateId(p);
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  for (const d of placeholderDefs.value) {
    placeholderValues[d.key] = "";
  }
  for (const [k, v] of Object.entries(p.placeholderValues || {})) {
    if (typeof v !== "string") continue;
    const match = placeholderDefs.value.find(
      (d) => normalizePlaceholderKey(d.key) === normalizePlaceholderKey(k),
    );
    placeholderValues[match?.key || k] = v;
  }
  draftReady.value = true;
  endProgrammaticUpdate();
}

async function applyDraftRoute() {
  const id = props.draftId;
  if (id == null || !Number.isFinite(id) || id <= 0) return;
  let item = drafts.value.find((d) => d.id === id);
  if (!item) {
    await loadDrafts();
    item = drafts.value.find((d) => d.id === id);
  }
  if (!item) {
    error.value = "Draft not found";
    await navigateTo("/create/drafts");
    return;
  }
  error.value = "";
  success.value = "";
  mode.value = "create";
  editingDraftId.value = id;
  loadDraftFromPayload(parseDraftPayload(item));
}

function cancelEditDraftItem() {
  editingDraftId.value = null;
  resetFormAfterDraft();
  if (props.draftId != null) navigateTo("/create");
}

async function saveDraft() {
  const issue = validateDraft();
  if (issue === "missing-token") {
    openSettings({ focusToken: true });
    return;
  }
  if (issue) {
    error.value = issue;
    return;
  }
  if (!selectedClub.value?.amIAdmin) {
    error.value = "Only club admins can save shared club drafts";
    return;
  }

  draftsBusy.value = true;
  error.value = "";
  success.value = "";
  const payload = buildDraftPayload();

  try {
    if (editingDraftId.value != null) {
      await api(`/api/drafts/${editingDraftId.value}`, {
        method: "PUT",
        headers: authHeaders(),
        body: JSON.stringify({ payload }),
      });
      editingDraftId.value = null;
    } else {
      await api("/api/drafts", {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ payload }),
      });
    }
    resetFormAfterDraft();
    await loadDrafts();
    setFlash(`Saved draft “${payload.name}”`);
    await navigateTo("/create/drafts");
  } catch (e: any) {
    error.value = e.message || "Failed to save draft";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    draftsBusy.value = false;
  }
}

async function askCreateMeetup() {
  const issue = validateDraft();
  if (issue === "missing-token") {
    openSettings({ focusToken: true });
    return;
  }
  if (issue) {
    error.value = issue;
    return;
  }
  confirmCreateOpen.value = true;
}

async function createMeetup() {
  confirmCreateOpen.value = false;
  creating.value = true;
  error.value = "";
  success.value = "";
  try {
    const body = buildMeetupBodyFromDraft(buildDraftPayload(), timeZone.value);
    const event = await api<{ id: string; name: string }>("/api/campfire/meetups", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(body),
    });
    success.value = `Created meetup “${event.name}” (${event.id})`;
  } catch (e: any) {
    error.value = e.message || "Failed to create meetup";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    creating.value = false;
  }
}

async function saveEditedMeetup() {
  const issue = validateDraft();
  if (issue === "missing-token") {
    openSettings({ focusToken: true });
    return;
  }
  if (issue) {
    error.value = issue;
    return;
  }

  creating.value = true;
  error.value = "";
  success.value = "";
  try {
    const body = buildEditBody();
    const event = await api<{ id: string; name: string }>(
      `/api/campfire/meetups/${encodeURIComponent(selectedEventId.value)}`,
      {
        method: "PUT",
        headers: authHeaders(),
        body: JSON.stringify(body),
      },
    );
    success.value = `Updated meetup “${event.name}” (${event.id})`;
    originalCoverPhotoUrl.value = draft.coverPhotoUrl || "";
    await loadClubEvents();
  } catch (e: any) {
    error.value = e.message || "Failed to update meetup";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    creating.value = false;
  }
}

async function deleteEditedMeetup() {
  if (!token.value) {
    openSettings({ focusToken: true });
    return;
  }
  const id = selectedEventId.value;
  if (!id) {
    error.value = "Pick a meetup to delete";
    return;
  }
  const name = draft.name.trim() || "this meetup";
  if (!confirm(`Delete “${name}” from Campfire? This cannot be undone.`)) return;

  creating.value = true;
  error.value = "";
  success.value = "";
  try {
    await api(`/api/campfire/meetups/${encodeURIComponent(id)}`, {
      method: "DELETE",
      headers: authHeaders(),
    });
    success.value = `Deleted meetup “${name}”`;
    selectedEventId.value = "";
    clearEditDraft();
    await loadClubEvents();
  } catch (e: any) {
    error.value = e.message || "Failed to delete meetup";
    if (isCampfireTokenError(e)) invalidateToken();
  } finally {
    creating.value = false;
  }
}

function templateOptionLabel(t: MeetupTemplate) {
  const cat = templateCategory(t);
  return cat ? `${t.name} (${cat})` : t.name;
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel create-flow">
      <div style="display: flex; justify-content: space-between; gap: 1rem; align-items: baseline; flex-wrap: wrap">
        <h2 style="margin: 0; font-size: 1.1rem">
          {{ mode === "edit" ? "Edit meetup" : "Create meetup" }}
        </h2>
        <span v-if="campfireName" class="muted">Campfire: {{ campfireName }}</span>
      </div>

      <div class="mode-toggle" role="tablist" aria-label="Meetup mode">
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'create'"
          :class="{ active: mode === 'create' }"
          @click="mode = 'create'"
        >
          Create
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="mode === 'edit'"
          :class="{ active: mode === 'edit' }"
          @click="mode = 'edit'"
        >
          Edit existing
        </button>
      </div>

      <p class="muted steps-hint">
        <template v-if="mode === 'create'">
          Pick a club (live event optional) → fill details → create now or
          <NuxtLink to="/create/drafts">save a draft</NuxtLink>
          to post later
        </template>
        <template v-else>
          Pick a club and meetup → optionally apply a template → save changes
        </template>
      </p>

      <p v-if="error" class="error">{{ error }}</p>
      <p v-if="success" class="success">{{ success }}</p>
      <div class="setup-grid">
        <div class="field">
          <label for="club">Club</label>
          <select id="club" v-model="selectedClubId">
            <option value="" disabled>
              {{ clubs.length ? "Select a club…" : "No clubs where you can create meetups" }}
            </option>
            <option v-for="c in clubs" :key="c.id" :value="c.id">
              {{ c.name }}{{ c.amIAdmin ? " (admin)" : "" }}
            </option>
          </select>
        </div>

        <div v-if="mode === 'create'" class="field">
          <label for="live">Live event</label>
          <select id="live" v-model="selectedLiveEventId">
            <option value="">None (no linked live event)</option>
            <option v-for="ev in liveEvents" :key="ev.id" :value="ev.id">
              {{ liveEventOptionLabel(ev) }}
            </option>
          </select>
        </div>

        <div v-else class="field">
          <label for="meetup">Meetup</label>
          <select id="meetup" v-model="selectedEventId" :disabled="!selectedClubId || loadingEvents">
            <option value="" disabled>
              {{
                loadingEvents
                  ? "Loading meetups…"
                  : !selectedClubId
                    ? "Select a club first…"
                    : clubEvents.length
                      ? "Select a meetup…"
                      : "No upcoming meetups in this club"
              }}
            </option>
            <option v-for="ev in clubEvents" :key="ev.id" :value="ev.id">
              {{ clubEventOptionLabel(ev) }}
            </option>
          </select>
        </div>

        <div class="field">
          <label for="tpl">Template</label>
          <select
            id="tpl"
            v-model="selectedTemplateId"
            :disabled="mode === 'edit' && !selectedEventId"
          >
            <option value="">{{ mode === "edit" ? "Keep current details" : "No template" }}</option>
            <option v-for="t in filteredTemplates" :key="t.id" :value="t.id">
              {{ templateOptionLabel(t) }}
            </option>
          </select>
          <p v-if="!templates.length" class="field-hint muted">
            No templates —
            <NuxtLink to="/templates">create one</NuxtLink>
          </p>
        </div>
      </div>

      <div v-if="selectionReady" class="field">
        <label>Placeholders</label>
        <div class="placeholder-fill placeholder-grid">
          <p class="hint muted" style="grid-column: 1 / -1; margin: 0">
            Custom fields for this template:
          </p>
          <div v-for="d in placeholderDefs" :key="d.key" class="field ph-item">
            <label :for="`ph-${d.key}`">{{ d.label || humanizePlaceholderKey(d.key) }}</label>
            <input
              :id="`ph-${d.key}`"
              v-model="placeholderValues[d.key]"
              type="text"
              :placeholder="d.default || d.label || humanizePlaceholderKey(d.key)"
            />
          </div>
        </div>
      </div>

      <div v-if="draftReady" class="editor">
        <div class="editor-main">
          <div class="details-head">
            <h3>Meetup details</h3>
            <div
              v-if="selectedTemplate"
              class="mode-toggle text-view-toggle"
              role="tablist"
              aria-label="Template text view"
            >
              <button
                type="button"
                role="tab"
                :aria-selected="textViewMode === 'edit'"
                :class="{ active: textViewMode === 'edit' }"
                @click="textViewMode = 'edit'"
              >
                Edit
              </button>
              <button
                type="button"
                role="tab"
                :aria-selected="textViewMode === 'preview'"
                :class="{ active: textViewMode === 'preview' }"
                @click="textViewMode = 'preview'"
              >
                Preview
              </button>
            </div>
          </div>
          <p v-if="selectedTemplate && textViewMode === 'edit'" class="hint muted">
            Placeholders like <code v-pre>{{liveEvent}}</code> stay visible here and in saved
            drafts. Preview shows filled values; the server resolves them when posting to Campfire.
          </p>

          <div class="field">
            <label for="name">Title</label>
            <input
              v-if="textViewMode === 'edit' || !selectedTemplate"
              id="name"
              v-model="draft.name"
              type="text"
              required
            />
            <input
              v-else
              id="name-preview"
              type="text"
              :value="previewText.name"
              readonly
              class="preview-field"
            />
          </div>

          <div class="field field-details">
            <label for="details">Description</label>
            <textarea
              v-if="textViewMode === 'edit' || !selectedTemplate"
              id="details"
              v-model="draft.details"
              rows="3"
            />
            <textarea
              v-else
              id="details-preview"
              :value="previewText.details"
              rows="3"
              readonly
              class="preview-field"
            />
            <PlaceholderHelp compact />
          </div>

          <div class="row-2">
            <div class="field">
              <label for="start">Start</label>
              <input id="start" v-model="draft.eventTime" type="datetime-local" required />
            </div>
            <div class="field">
              <label for="end">End</label>
              <input id="end" v-model="draft.eventEndTime" type="datetime-local" />
            </div>
          </div>
          <p class="hint muted" style="margin-top: -0.35rem">
            Times are wall-clock values in your Settings timezone ({{ timeZone }}) and sent to
            Campfire as UTC.
            <template v-if="selectedLiveEvent">
              Template clocks use the live event’s local calendar day.
            </template>
            <template v-else>
              With no live event linked, template clocks use today.
            </template>
          </p>

          <div class="field">
            <label>Who to invite</label>
            <InvitePicker
              :all-invited="draft.allInvited"
              @update:all-invited="draft.allInvited = $event"
            />
          </div>

          <div class="field">
            <label>Who can comment</label>
            <CommentsPermissionsPicker v-model="draft.commentsPermissions" />
          </div>

          <div class="field">
            <label class="check">
              <input v-model="draft.createdByCommunityAmbassador" type="checkbox" />
              Mark as Community Ambassador event
            </label>
            <p class="hint muted">Shows the CA badge on the meetup in Campfire.</p>
          </div>
        </div>

        <div class="editor-side">
          <div class="field">
            <label>Cover photo (optional)</label>
            <CoverPhotoField
              v-model="draft.coverPhotoUrl"
              :auth-headers="authHeaders()"
            />
          </div>
          <div class="field" style="margin-bottom: 0">
            <label>Location</label>
            <ClientOnly>
              <LocationPicker
                v-model="draftLocation"
                :initial-center="draftLocation || undefined"
                :jitter-meters="mode === 'create' ? draft.locationJitterMeters : 0"
                @place="onPlaceSelected"
              >
                <template v-if="mode === 'create'" #meta>
                  <div class="jitter-inline">
                    <label for="jitter" title="Random offset so repeated meetup pins don’t stack">
                      Jitter (m)
                    </label>
                    <input
                      id="jitter"
                      v-model.number="draft.locationJitterMeters"
                      type="number"
                      min="0"
                      max="500"
                      step="5"
                      title="Random radius in meters around the pin. 0 keeps the exact coordinates."
                    />
                  </div>
                </template>
              </LocationPicker>
            </ClientOnly>
            <p v-if="mode === 'create'" class="hint muted">
              Jitter randomly offsets each meetup within this many meters of the pin so markers don’t stack. Use 0 for the exact spot.
            </p>
            <div class="field" style="margin-top: 0.75rem; margin-bottom: 0">
              <label for="address">Address</label>
              <input
                v-if="textViewMode === 'edit' || !selectedTemplate"
                id="address"
                v-model="draft.address"
                type="text"
                placeholder="Optional — filled from place search"
              />
              <input
                v-else
                id="address-preview"
                type="text"
                :value="previewText.address"
                readonly
                class="preview-field"
                placeholder="Optional"
              />
              <p class="hint muted">Shown on the meetup; place search fills this when you pick a result.</p>
            </div>
          </div>
        </div>
      </div>

      <div v-if="draftReady" style="margin-top: 1.25rem; display: flex; flex-wrap: wrap; gap: 0.5rem">
        <template v-if="mode === 'create'">
          <p
            v-if="selectedClub && !selectedClub.amIAdmin"
            class="hint muted"
            style="flex-basis: 100%; margin: 0"
          >
            Drafts are shared with club admins — only admins can save them.
          </p>
          <button
            v-if="editingDraftId != null"
            type="button"
            :disabled="creating || draftsBusy"
            @click="cancelEditDraftItem"
          >
            Cancel edit
          </button>
          <button
            type="button"
            :class="{ primary: editingDraftId != null }"
            :disabled="creating || draftsBusy || !canSaveDraft"
            @click="saveDraft"
          >
            <Icon name="save" />
            {{ editingDraftId != null ? "Save changes" : "Save draft" }}
          </button>
          <button
            v-if="editingDraftId == null"
            type="button"
            class="primary"
            :disabled="creating || !canCreate"
            @click="askCreateMeetup"
          >
            <Icon name="post" />
            {{ creating ? "Creating…" : "Create now" }}
          </button>
        </template>
        <template v-else>
          <button
            type="button"
            class="primary"
            :disabled="creating || !canSaveEdit"
            @click="saveEditedMeetup"
          >
            <Icon name="save" />
            {{ creating ? "Saving…" : "Save changes" }}
          </button>
          <button
            type="button"
            class="danger"
            :disabled="creating || !selectedEventId"
            @click="deleteEditedMeetup"
          >
            <Icon name="trash" />
            Delete meetup
          </button>
        </template>
      </div>

    </div>

    <ConfirmDialog
      :open="confirmCreateOpen"
      title="Create meetup?"
      :message="`Post “${previewText.name || draft.name || 'this meetup'}” to Campfire now?`"
      confirm-label="Create"
      :busy="creating"
      @cancel="confirmCreateOpen = false"
      @confirm="createMeetup"
    />
  </AppShell>
</template>

<style scoped>
.hint {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
.check {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: fit-content;
  max-width: 100%;
  margin: 0;
  color: var(--text);
  font-size: 0.9rem;
  cursor: pointer;
}
.check input {
  flex: 0 0 auto;
}
.create-flow {
  max-width: none;
}
.create-flow :deep(.field) {
  margin-bottom: 0.7rem;
}
.create-flow :deep(label) {
  margin-bottom: 0.2rem;
}
.create-flow :deep(input:not([type="radio"]):not([type="checkbox"])),
.create-flow :deep(textarea),
.create-flow :deep(select) {
  padding: 0.45rem 0.6rem;
}
.steps-hint {
  margin: 0.25rem 0 0.85rem;
  font-size: 0.85rem;
}
.mode-toggle {
  display: inline-flex;
  margin: 0.75rem 0 0.15rem;
  padding: 0.2rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-input);
  gap: 0.15rem;
}
.mode-toggle button {
  margin: 0;
  padding: 0.35rem 0.85rem;
  border: none;
  border-radius: calc(var(--radius) - 2px);
  background: transparent;
  color: var(--muted);
  font-size: 0.85rem;
  cursor: pointer;
}
.mode-toggle button.active {
  background: var(--bg);
  color: var(--text);
  box-shadow: 0 0 0 1px var(--border);
}
.details-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.35rem;
}
.details-head h3 {
  margin: 0;
}
.text-view-toggle {
  flex: 0 0 auto;
}
.preview-field {
  opacity: 0.92;
  cursor: default;
  background: color-mix(in srgb, var(--panel) 70%, transparent);
}
.create-flow h3 {
  margin: 0 0 0.65rem;
  font-size: 0.95rem;
}
.setup-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem 1rem;
  margin-bottom: 0.35rem;
}
.field-hint {
  margin: 0.25rem 0 0;
  font-size: 0.8rem;
}
.editor {
  margin-top: 0.75rem;
  padding: 0.9rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-input);
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(280px, 0.9fr);
  gap: 1rem 1.25rem;
  align-items: stretch;
}
.editor-main,
.editor-side {
  min-width: 0;
}
.editor-main {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.field-details {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 8rem;
  margin-bottom: 0.7rem !important;
}
.field-details textarea {
  flex: 1 1 auto;
  min-height: 6rem;
  height: 100%;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  resize: vertical;
}
.editor-side :deep(.cover-photo .tile) {
  width: 128px;
  height: 128px;
}
.editor-side :deep(.location-map) {
  height: 260px;
}
.editor-side :deep(.location-meta) {
  margin-top: 0.35rem;
}
.jitter-inline {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-left: auto;
}
.jitter-inline label {
  margin: 0;
  white-space: nowrap;
}
.jitter-inline input,
.editor-side #jitter {
  width: 5.5rem;
  max-width: 5.5rem;
}
.placeholder-fill {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.65rem;
  background: var(--bg-input);
}
.placeholder-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem 0.75rem;
}
.ph-item {
  margin-bottom: 0 !important;
}
@media (max-width: 900px) {
  .setup-grid {
    grid-template-columns: 1fr;
  }
  .editor {
    grid-template-columns: 1fr;
  }
  .placeholder-grid {
    grid-template-columns: 1fr;
  }
  .editor-side :deep(.location-map) {
    height: 220px;
  }
}
</style>
