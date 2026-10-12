<script setup lang="ts">
import type {
  Club,
  ClubDiscordSettings,
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
  ensureMinMeetupEnd,
  formatLocalDateTime,
  instantToLocalInput,
  isMeetupDurationValid,
  isMeetupWithinLiveEventWindow,
  liveEventWindowBounds,
  MIN_MEETUP_DURATION_MINUTES,
  parseLocalDateTime,
} from "~/utils/datetime";
import {
  applyPlaceholders,
  extractPlaceholderKeys,
  humanizePlaceholderKey,
  isBuiltinPlaceholder,
  isEventPokemonNameKey,
  normalizePlaceholderKey,
  resolveBuiltinPlaceholderValues,
  resolveCustomPlaceholderDefs,
  type PlaceholderDef,
} from "~/utils/placeholders";
import { DEFAULT_LOCATION_JITTER_METERS, parseCampfireLocation } from "~/utils/location";
import { buildMeetupBodyFromDraft, parseDraftPayload } from "~/utils/drafts";
import { fetchMeetupSchedule } from "~/utils/meetupSchedule";
import { isCampfireTokenError } from "~/composables/useSessionToken";
import {
  TEMPLATE_LANGUAGES,
  isTemplateLanguageCode,
  type TemplateLanguageCode,
} from "~/utils/languages";

const { matchOrder: categoryMatchOrder, fixedLocalHour: isFixedLocalHourCategory } =
  useEventCategories();

const props = defineProps<{
  /** When set, load this club draft into the create form. */
  draftId?: number | null;
}>();

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { token, authHeaders, openSettings, invalidateToken } = useSessionToken();
const { me: campfireMe, validateToken } = useCampfireSession();
const { timeZone, preferredClubId, preferredLanguage } = usePreferences();
const toast = useToast();

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
/** Bumped to drop in-flight hydrate/blank work that would wipe a loaded draft. */
let hydrateGen = 0;

const selectedClubId = ref("");
const selectedLiveEventId = ref("");
const selectedTemplateId = ref<number | "">("");
const selectedEventId = ref("");
const originalCoverPhotoUrl = ref("");
const placeholderValues = reactive<Record<string, string>>({});
/** Language for eventPokemon translation; defaults from template / Settings, overridable on create. */
const meetupLanguage = ref<TemplateLanguageCode>(preferredLanguage.value);
const languageDirty = ref(false);

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
  discordChannelId: "",
  locationJitterMeters: DEFAULT_LOCATION_JITTER_METERS,
  campfireLiveEventId: "",
});
const draftLocation = ref<LatLngPoint | null>(null);
const lastDiscordSettings = ref<ClubDiscordSettings | null>(null);
const draftReady = ref(false);
/** True once the user changes template or meetup fields after auto-fill. */
const detailsTouched = ref(false);
/** Suppress detailsTouched while applying live-event / template selections. */
let programmaticDepth = 0;
let programmaticUpdate = false;

function beginProgrammaticUpdate() {
  programmaticDepth++;
  programmaticUpdate = true;
}

function endProgrammaticUpdate() {
  nextTick(() => {
    programmaticDepth = Math.max(0, programmaticDepth - 1);
    if (programmaticDepth === 0) programmaticUpdate = false;
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
  // Claim draft-edit mode before club bootstrap so async hydrate can't clear the pin.
  if (props.draftId != null && Number.isFinite(props.draftId) && props.draftId > 0) {
    editingDraftId.value = props.draftId;
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

const selectedClubDiscord = computed(
  () => selectedClub.value?.discord ?? null,
);

const selectedLiveEvent = computed(() =>
  liveEvents.value.find((e) => e.id === selectedLiveEventId.value) || null,
);

const liveEventCategory = computed(
  () => selectedLiveEvent.value?.category?.trim() || "",
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

/** Used eventPokemon / eventPokemon[i] name tokens (editable) from template and draft. */
const eventPokemonNameDefs = computed(() => {
  const chunks: string[] = [];
  if (rawPayload.value) {
    chunks.push(
      rawPayload.value.name,
      rawPayload.value.details,
      rawPayload.value.address,
      rawPayload.value.coverPhotoUrl,
    );
  }
  chunks.push(draft.name, draft.details, draft.address, draft.coverPhotoUrl);
  return extractPlaceholderKeys(chunks.filter(Boolean).join("\n")).filter(
    isEventPokemonNameKey,
  );
});

const selectionReady = computed(
  () => placeholderDefs.value.length > 0 || eventPokemonNameDefs.value.length > 0,
);

const MAX_MEETUP_TITLE = 120;
const MAX_MEETUP_DETAILS = 1000;
const eventPokemonResolved = ref<Record<string, string>>({});
const eventPokemonDirty = ref<Set<string>>(new Set());
let resolveSeq = 0;
let eventPokemonResolveTimer: ReturnType<typeof setTimeout> | null = null;

/** Edit keeps {{tokens}} visible; preview shows resolved copy. */
const textViewMode = ref<"edit" | "preview">("edit");

function clockFromLocalInput(local: string): string {
  const parts = parseLocalDateTime(local);
  if (!parts) return "";
  return `${String(parts.hours).padStart(2, "0")}:${String(parts.minutes).padStart(2, "0")}`;
}

function draftCalendarDate(): string {
  const parts = parseLocalDateTime(draft.eventTime);
  if (parts) {
    return `${parts.year}-${String(parts.month).padStart(2, "0")}-${String(parts.day).padStart(2, "0")}`;
  }
  return "";
}

function builtinPlaceholderContext() {
  const p = rawPayload.value;
  const dayParts = parseLocalDateTime(draft.eventTime);
  return {
    clubName: selectedClub.value?.name || "",
    liveEventName: selectedLiveEvent.value?.eventName || "",
    category: liveEventCategory.value || p?.category || "",
    date: dayParts
      ? { year: dayParts.year, month: dayParts.month, day: dayParts.day }
      : undefined,
    startTime: clockFromLocalInput(draft.eventTime) || p?.startTime || "",
    endTime: clockFromLocalInput(draft.eventEndTime) || p?.endTime || "",
    timeZone: timeZone.value,
    title: draft.name || "",
    address: draft.address || "",
    latitude: draftLocation.value?.lat ?? null,
    longitude: draftLocation.value?.lng ?? null,
    eventPokemonValues: eventPokemonResolved.value,
  };
}

function currentPlaceholderMap(): Record<string, string> {
  const values = resolveBuiltinPlaceholderValues(builtinPlaceholderContext());
  for (const d of placeholderDefs.value) {
    const raw = (placeholderValues[d.key] ?? "").trim();
    if (raw) values[d.key] = raw;
    else if (d.default?.trim()) values[d.key] = d.default.trim();
  }
  for (const key of eventPokemonNameDefs.value) {
    const raw = (placeholderValues[key] ?? "").trim();
    if (raw) values[key] = raw;
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

const resolvedNameLen = computed(() => [...previewText.value.name].length);
const resolvedDetailsLen = computed(() => [...previewText.value.details].length);
const postLengthBlocked = computed(
  () =>
    resolvedNameLen.value > MAX_MEETUP_TITLE ||
    resolvedDetailsLen.value > MAX_MEETUP_DETAILS,
);

function collectEventPokemonNameOverrides(): Record<string, string> {
  const overrides: Record<string, string> = {};
  for (const key of eventPokemonNameDefs.value) {
    if (!eventPokemonDirty.value.has(normalizePlaceholderKey(key))) continue;
    const raw = (placeholderValues[key] ?? "").trim();
    if (raw) overrides[key] = raw;
  }
  return overrides;
}

function applyMeetupLanguage(code: string | null | undefined, opts?: { force?: boolean }) {
  if (!opts?.force && languageDirty.value) return;
  if (isTemplateLanguageCode(code || "")) {
    meetupLanguage.value = code as TemplateLanguageCode;
    return;
  }
  if (opts?.force || !meetupLanguage.value) {
    meetupLanguage.value = preferredLanguage.value;
  }
}

function onMeetupLanguageInput() {
  languageDirty.value = true;
  markDetailsTouched();
}

function onTitleInput(value: string) {
  if (textViewMode.value === "preview") return;
  draft.name = value;
}

function onDetailsInput(value: string) {
  if (textViewMode.value === "preview") return;
  draft.details = value;
}

function onAddressInput(e: Event) {
  if (textViewMode.value === "preview") return;
  draft.address = (e.target as HTMLInputElement).value;
}

/** Prefs/user load after setup — keep the select in sync until the user overrides it. */
watch(
  preferredLanguage,
  () => {
    if (languageDirty.value) return;
    applyMeetupLanguage(selectedTemplate.value?.language, { force: true });
  },
);

async function refreshEventPokemonResolve() {
  const liveName = selectedLiveEvent.value?.eventName || "";
  const category = liveEventCategory.value || rawPayload.value?.category || "";
  const language = meetupLanguage.value || "";
  const nameOverrides = collectEventPokemonNameOverrides();
  const seq = ++resolveSeq;
  if (!liveName && !category && !Object.keys(nameOverrides).length) {
    eventPokemonResolved.value = {};
    return;
  }
  try {
    const res = await api<{ values: Record<string, string> }>(
      "/api/placeholders/resolve",
      {
        method: "POST",
        body: JSON.stringify({
          liveEventName: liveName,
          category,
          language,
          ...(Object.keys(nameOverrides).length
            ? { eventPokemonValues: nameOverrides }
            : {}),
        }),
      },
    );
    if (seq !== resolveSeq) return;
    eventPokemonResolved.value = res.values || {};
    // Auto-fill must not flip detailsTouched (blocks live-event template re-select).
    beginProgrammaticUpdate();
    try {
      for (const key of eventPokemonNameDefs.value) {
        if (eventPokemonDirty.value.has(normalizePlaceholderKey(key))) continue;
        let fill = "";
        for (const [k, val] of Object.entries(res.values || {})) {
          if (normalizePlaceholderKey(k) === normalizePlaceholderKey(key)) {
            fill = val;
            break;
          }
        }
        placeholderValues[key] = fill;
      }
    } finally {
      endProgrammaticUpdate();
    }
  } catch {
    if (seq !== resolveSeq) return;
    eventPokemonResolved.value = {};
  }
}

function onEventPokemonInput(key: string) {
  eventPokemonDirty.value.add(normalizePlaceholderKey(key));
  if (eventPokemonResolveTimer != null) clearTimeout(eventPokemonResolveTimer);
  eventPokemonResolveTimer = setTimeout(() => {
    eventPokemonResolveTimer = null;
    void refreshEventPokemonResolve();
  }, 250);
}

function placeholderRequired(d: PlaceholderDef): boolean {
  return !(d.default || "").trim();
}

function collectPlaceholderOverrides(): Record<string, string> {
  const customValues: Record<string, string> = {};
  for (const d of placeholderDefs.value) {
    const raw = (placeholderValues[d.key] ?? "").trim();
    const fallback = (d.default || "").trim();
    if (raw || fallback) customValues[d.key] = raw || fallback;
  }
  for (const key of eventPokemonNameDefs.value) {
    const raw = (placeholderValues[key] ?? "").trim();
    if (raw) customValues[key] = raw;
  }
  return customValues;
}

function validatePlaceholdersForPost(): string | null {
  for (const d of placeholderDefs.value) {
    if (!placeholderRequired(d)) continue;
    if (!(placeholderValues[d.key] ?? "").trim()) {
      return `Fill required placeholder: ${d.label || humanizePlaceholderKey(d.key)}`;
    }
  }
  for (const key of eventPokemonNameDefs.value) {
    if (!(placeholderValues[key] ?? "").trim()) {
      return `Fill required placeholder: ${humanizePlaceholderKey(key)}`;
    }
  }
  if (postLengthBlocked.value) {
    return `Too long after placeholders (title ≤ ${MAX_MEETUP_TITLE}, details ≤ ${MAX_MEETUP_DETAILS})`;
  }
  return null;
}

watch(
  () =>
    [
      selectedLiveEvent.value?.eventName || "",
      liveEventCategory.value,
      meetupLanguage.value,
      selectedTemplateId.value,
    ] as const,
  () => {
    void refreshEventPokemonResolve();
  },
);

/** Prefill editable eventPokemon fields when the token is newly typed into the draft. */
watch(
  eventPokemonNameDefs,
  (keys) => {
    const resolved = eventPokemonResolved.value;
    beginProgrammaticUpdate();
    try {
      for (const key of keys) {
        if (eventPokemonDirty.value.has(normalizePlaceholderKey(key))) continue;
        if ((placeholderValues[key] ?? "").trim()) continue;
        let fill = "";
        for (const [k, val] of Object.entries(resolved)) {
          if (normalizePlaceholderKey(k) === normalizePlaceholderKey(key)) {
            fill = val;
            break;
          }
        }
        if (fill) placeholderValues[key] = fill;
      }
    } finally {
      endProgrammaticUpdate();
    }
    if (
      keys.length &&
      !Object.keys(resolved).length &&
      (selectedLiveEvent.value?.eventName || liveEventCategory.value)
    ) {
      void refreshEventPokemonResolve();
    }
  },
  { deep: true },
);

watch(selectedLiveEventId, () => {
  eventPokemonDirty.value = new Set();
});

async function resolveSchedule(opts?: {
  liveEvent?: LiveEvent | null;
  startTime?: string;
  endTime?: string;
  date?: string;
}) {
  return fetchMeetupSchedule(api, {
    timeZone: timeZone.value,
    liveEvent: opts?.liveEvent ?? selectedLiveEvent.value,
    startTime: opts?.startTime,
    endTime: opts?.endTime,
    date: opts?.date,
  });
}

function resetPlaceholders() {
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  for (const d of placeholderDefs.value) {
    placeholderValues[d.key] = "";
  }
}

async function blankDraftStandalone() {
  if (editingDraftId.value != null) return;
  const gen = hydrateGen;
  draft.name = "";
  draft.details = "";
  draft.address = "";
  draft.coverPhotoUrl = "";
  draft.commentsPermissions = "ORGANIZERS_ONLY";
  draft.allInvited = true;
  draft.createdByCommunityAmbassador = true;
  draft.discordChannelId = "";
  const times = await resolveSchedule({ liveEvent: null });
  if (gen !== hydrateGen || editingDraftId.value != null) return;
  draft.eventTime = times.eventTime;
  draft.eventEndTime = times.eventEndTime;
  draft.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
  draft.campfireLiveEventId = "";
  draftLocation.value = null;
}

async function blankDraftFromLiveEvent(ev: LiveEvent) {
  if (editingDraftId.value != null) return;
  const gen = hydrateGen;
  draft.name = "";
  draft.details = "";
  draft.address = "";
  draft.coverPhotoUrl = "";
  draft.commentsPermissions = "ORGANIZERS_ONLY";
  draft.allInvited = true;
  draft.createdByCommunityAmbassador = true;
  draft.discordChannelId = "";
  const times = await resolveSchedule({ liveEvent: ev });
  if (gen !== hydrateGen || editingDraftId.value != null) return;
  draft.eventTime = times.eventTime;
  draft.eventEndTime = times.eventEndTime;
  draft.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
  draft.campfireLiveEventId = ev.id;
  draftLocation.value = null;
}

async function blankDraftForSelection() {
  if (editingDraftId.value != null) return;
  if (selectedLiveEvent.value) await blankDraftFromLiveEvent(selectedLiveEvent.value);
  else await blankDraftStandalone();
}

async function hydrateDraft() {
  // Saved-draft edit owns the form; club/live-event bootstrap must not replace it.
  if (editingDraftId.value != null) return;

  if (!selectedClub.value) {
    draftReady.value = false;
    draftLocation.value = null;
    return;
  }

  const gen = ++hydrateGen;
  const ev = selectedLiveEvent.value;
  const raw = rawPayload.value;
  if (selectedTemplate.value && raw) {
    const times = await resolveSchedule({
      liveEvent: ev,
      startTime: raw.startTime || raw.eventTime,
      endTime: raw.endTime || raw.eventEndTime,
    });
    if (gen !== hydrateGen || editingDraftId.value != null) return;
    // Load raw template copy so {{placeholders}} stay visible until Preview / submit.
    draft.name = raw.name || "";
    draft.details = raw.details || "";
    draft.address = raw.address || "";
    draft.coverPhotoUrl = raw.coverPhotoUrl || "";
    draft.commentsPermissions = raw.commentsPermissions || "ORGANIZERS_ONLY";
    draft.allInvited = raw.allInvited ?? true;
    draft.createdByCommunityAmbassador = raw.createdByCommunityAmbassador ?? true;
    draft.discordChannelId = (raw.discordChannelId || "").trim();
    draft.eventTime = times.eventTime;
    draft.eventEndTime = times.eventEndTime;
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
    syncDiscordChannelFromSettings();
    return;
  }

  if (!draftReady.value) {
    await blankDraftForSelection();
    if (gen !== hydrateGen || editingDraftId.value != null) return;
    syncDiscordChannelFromSettings();
  } else if (ev) {
    const times = await resolveSchedule({ liveEvent: ev });
    if (gen !== hydrateGen || editingDraftId.value != null) return;
    draft.eventTime = times.eventTime;
    draft.eventEndTime = times.eventEndTime;
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
  if (!languageDirty.value) {
    const tpl = next
      ? templates.value.find((t) => t.id === next)
      : null;
    applyMeetupLanguage(tpl?.language, { force: true });
  }
}

/** Apply selected template onto the meetup being edited (keeps its calendar day). */
async function applyTemplateToEdit() {
  const raw = rawPayload.value;
  if (!raw || !selectedEventId.value || !draftReady.value) return;

  const times = await resolveSchedule({
    liveEvent: selectedLiveEvent.value,
    startTime: raw.startTime || raw.eventTime,
    endTime: raw.endTime || raw.eventEndTime,
    date: draftCalendarDate() || undefined,
  });

  draft.name = raw.name || "";
  draft.details = raw.details || "";
  draft.address = raw.address || "";
  draft.coverPhotoUrl = raw.coverPhotoUrl || "";
  draft.commentsPermissions = raw.commentsPermissions || "ORGANIZERS_ONLY";
  draft.allInvited = raw.allInvited ?? true;
  draft.createdByCommunityAmbassador = raw.createdByCommunityAmbassador ?? true;
  draft.discordChannelId = (raw.discordChannelId || "").trim();
  draft.eventTime = times.eventTime;
  draft.eventEndTime = times.eventEndTime;
  if (raw.latitude != null && raw.longitude != null) {
    draftLocation.value = { lat: raw.latitude, lng: raw.longitude };
  }
  textViewMode.value = "edit";
  syncDiscordChannelFromSettings();
}

function onDiscordChannelsLoaded(settings: ClubDiscordSettings) {
  lastDiscordSettings.value = settings;
  const current = (draft.discordChannelId || "").trim();
  const ids = new Set((settings.channels || []).map((c) => c.id));
  if (current && ids.has(current)) return;
  const clubDefault = (settings.discordChannelId || "").trim();
  if (!current && clubDefault && ids.has(clubDefault)) {
    draft.discordChannelId = clubDefault;
    return;
  }
  if (settings.available && current && !ids.has(current)) {
    draft.discordChannelId = "";
    if (clubDefault && ids.has(clubDefault)) {
      draft.discordChannelId = clubDefault;
    }
  }
}

function syncDiscordChannelFromSettings() {
  if (lastDiscordSettings.value) onDiscordChannelsLoaded(lastDiscordSettings.value);
}

/** Only send a channel id that belongs to the selected club's Discord integration. */
function resolvedDiscordChannelId(): string {
  const id = (draft.discordChannelId || "").trim();
  if (!id) return "";
  const settings = lastDiscordSettings.value;
  if (!settings?.available) return "";
  if (!(settings.channels || []).some((c) => c.id === id)) return "";
  return id;
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
  // Only bootstrap the form on first club pick — changing clubs must not wipe
  // cover photo / details the user already filled in.
  if (!draftReady.value && selectedClub.value) {
    beginProgrammaticUpdate();
    void hydrateDraft().finally(() => endProgrammaticUpdate());
  }
});

watch(preferredClubId, (preferred) => {
  if (!preferred || !clubs.value.length) return;
  if (!clubs.value.some((c) => c.id === preferred)) return;
  if (selectedClubId.value === preferred) return;
  selectedClubId.value = preferred;
});

watch(selectedLiveEventId, () => {
  // Draft/edit restores set the id under programmaticUpdate — don't overwrite them.
  if (programmaticUpdate || mode.value === "edit" || editingDraftId.value != null) return;

  // Until the user edits, keep template in sync with the live event category.
  if (!detailsTouched.value) {
    beginProgrammaticUpdate();
    languageDirty.value = false;
    if (selectedLiveEvent.value) {
      autoSelectTemplate(true);
    } else {
      selectedTemplateId.value = "";
      applyMeetupLanguage(undefined, { force: true });
    }
    // After template id updates so custom placeholder keys match the new payload.
    resetPlaceholders();
    // Re-apply even when the template id is unchanged (new event times / name).
    void hydrateDraft().finally(() => endProgrammaticUpdate());
    return;
  }

  beginProgrammaticUpdate();
  void hydrateDraft().finally(() => endProgrammaticUpdate());
});

watch(selectedTemplateId, (id) => {
  // Draft / live-event / club restores set the id under programmaticUpdate and
  // hydrate themselves — don't wipe the form by re-applying the template.
  if (programmaticUpdate) return;
  markDetailsTouched();
  resetPlaceholders();
  languageDirty.value = false;
  applyMeetupLanguage(selectedTemplate.value?.language, { force: true });
  textViewMode.value = "edit";
  if (mode.value === "edit") {
    if (id && selectedEventId.value && draftReady.value) {
      void applyTemplateToEdit();
    }
    return;
  }
  if (!id) {
    if (selectedClub.value) {
      void blankDraftForSelection().then(() => {
        draftReady.value = true;
      });
    } else {
      draftReady.value = false;
      draftLocation.value = null;
    }
    return;
  }
  void hydrateDraft();
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
    draft.discordChannelId,
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
  if (ev) void loadDraftFromClubEvent(ev);
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
    void hydrateDraft().finally(() => endProgrammaticUpdate());
    return;
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
    !!draft.eventEndTime &&
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
    !!draft.eventEndTime &&
    !!draft.name.trim(),
);

/** Earliest allowed end for `<input type="datetime-local" :min>`. */
const minEndTime = computed(() =>
  draft.eventTime ? ensureMinMeetupEnd(draft.eventTime, null) : "",
);

const selectedLiveEventWindowLabel = computed(() => {
  const ev = selectedLiveEvent.value;
  if (!ev) return "";
  const start = instantToLocalInput(ev.startTimestamp, timeZone.value);
  const end = instantToLocalInput(ev.endTimestamp, timeZone.value);
  if (!start || !end) return "";
  return `${formatLocalDateTime(start)} - ${formatLocalDateTime(end)}`;
});

const liveEventWindowError = computed(() => {
  const ev = selectedLiveEvent.value;
  if (!ev || !draft.eventTime || !draft.eventEndTime) return "";
  if (!liveEventWindowBounds(ev)) return "";
  if (
    isMeetupWithinLiveEventWindow(
      draft.eventTime,
      draft.eventEndTime,
      ev,
      timeZone.value,
    )
  ) {
    return "";
  }
  const window = selectedLiveEventWindowLabel.value;
  return window
    ? `Outside live event window (${window})`
    : "Outside live event window";
});

function validateDraft(): string | null {
  if (!token.value) return "missing-token";
  if (!selectedClubId.value) return "Pick a club";
  if (mode.value === "edit") {
    if (!selectedEventId.value) return "Pick a meetup to edit";
  }
  if (!draftLocation.value) return "Set a location";
  if (!draft.name.trim()) return "Title is required";
  if (!draft.eventTime) return "Start time is required";
  if (!draft.eventEndTime) return "End time is required";
  draft.eventEndTime = ensureMinMeetupEnd(draft.eventTime, draft.eventEndTime);
  if (!isMeetupDurationValid(draft.eventTime, draft.eventEndTime)) {
    return `End must be at least ${MIN_MEETUP_DURATION_MINUTES} minutes after start`;
  }
  if (liveEventWindowError.value) return liveEventWindowError.value;
  return null;
}

watch(
  () => draft.eventTime,
  (start) => {
    if (!start || programmaticUpdate) return;
    draft.eventEndTime = ensureMinMeetupEnd(start, draft.eventEndTime);
  },
);

watch(
  () => draft.eventEndTime,
  (end) => {
    if (!draft.eventTime || programmaticUpdate) return;
    const next = ensureMinMeetupEnd(draft.eventTime, end);
    if (next !== (end || "")) draft.eventEndTime = next;
  },
);

function isoToLocalInput(iso: string | undefined | null): string {
  return instantToLocalInput(iso, timeZone.value);
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
  draft.discordChannelId = "";
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
      const res = await api<{ events: ClubEvent[]; discord?: ClubDiscordSettings }>(
        `/api/campfire/clubs/${encodeURIComponent(clubId)}/events?first=100`,
        { headers: authHeaders() },
      );
      if (selectedClubId.value !== clubId) return;
      clubEvents.value = res.events || [];
      if (res.discord) {
        const idx = clubs.value.findIndex((c) => c.id === clubId);
        if (idx >= 0) {
          clubs.value[idx] = { ...clubs.value[idx], discord: res.discord };
        }
      }
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
  if (hasMeetup && hasDraft) return `${ev.eventName} (has meetup) (has draft)`;
  if (hasMeetup) return `${ev.eventName} (has meetup)`;
  if (hasDraft) return `${ev.eventName} (has draft)`;
  return ev.eventName;
}

async function loadDraftFromClubEvent(ev: ClubEvent) {
  draft.name = ev.name || "";
  draft.details = ev.details || "";
  draft.address = ev.address || "";
  draft.coverPhotoUrl = ev.coverPhotoUrl || "";
  originalCoverPhotoUrl.value = ev.coverPhotoUrl || "";
  draft.commentsPermissions = ev.commentsPermissions || "ORGANIZERS_ONLY";
  draft.allInvited = ev.allInvited ?? true;
  draft.createdByCommunityAmbassador = ev.createdByCommunityAmbassador ?? true;
  draft.discordChannelId = (ev.discordChannelId || "").trim();
  draft.campfireLiveEventId = ev.campfireLiveEventId || "";
  draft.locationJitterMeters = 0;
  draftLocation.value = parseCampfireLocation(ev.location);
  if (ev.campfireLiveEventId && ev.campfireLiveEventId !== selectedLiveEventId.value) {
    selectedLiveEventId.value = ev.campfireLiveEventId;
  }

  const start = isoToLocalInput(ev.eventTime);
  const end = isoToLocalInput(ev.eventEndTime);
  const live =
    liveEvents.value.find((e) => e.id === (ev.campfireLiveEventId || "")) || null;
  const fixedHour =
    live?.fixedLocalHour === true ||
    isFixedLocalHourCategory(live?.category || "");
  // Raid Hour / Spotlight / Max Monday are always 18:00–19:00 local. Campfire
  // may store the worldwide UTC window instead — keep the calendar day, fix clocks.
  if (fixedHour) {
    const times = await resolveSchedule({
      liveEvent: live,
      date: start ? start.slice(0, 10) : undefined,
      startTime: "18:00",
      endTime: "19:00",
    });
    draft.eventTime = times.eventTime;
    draft.eventEndTime = times.eventEndTime;
  } else {
    draft.eventTime = start;
    draft.eventEndTime = end;
  }
  draftReady.value = true;
  syncDiscordChannelFromSettings();
}

function clubEventOptionLabel(ev: ClubEvent) {
  let local = isoToLocalInput(ev.eventTime);
  const live = liveEvents.value.find((e) => e.id === (ev.campfireLiveEventId || ""));
  const fixedHour =
    live?.fixedLocalHour === true ||
    isFixedLocalHourCategory(live?.category || "");
  if (live && fixedHour && local) {
    // Show the real meetup hour, not Campfire's worldwide window.
    local = `${local.slice(0, 10)}T18:00`;
  }
  const when = local ? formatLocalDateTime(local) : "";
  return when && when !== "—" ? `${ev.name} · ${when}` : ev.name;
}

function buildEditBody(): EditMeetupInput {
  const cover = draft.coverPhotoUrl || "";
  const photoChanged = cover !== (originalCoverPhotoUrl.value || "");
  const customValues = collectPlaceholderOverrides();
  return {
    name: draft.name.trim(),
    details: draft.details,
    eventTime: draft.eventTime,
    eventEndTime: draft.eventEndTime || undefined,
    latitude: draftLocation.value!.lat,
    longitude: draftLocation.value!.lng,
    address: draft.address || undefined,
    coverPhotoUrl: photoChanged ? cover || undefined : undefined,
    commentsPermissions: draft.commentsPermissions || undefined,
    campfireLiveEventId: draft.campfireLiveEventId || selectedLiveEventId.value || undefined,
    allInvited: draft.allInvited,
    createdByCommunityAmbassador: draft.createdByCommunityAmbassador,
    ...(resolvedDiscordChannelId()
      ? { discordChannelId: resolvedDiscordChannelId() }
      : {}),
    hasEventPhotoChanged: photoChanged,
    clubName: selectedClub.value?.name || undefined,
    liveEventName: selectedLiveEvent.value?.eventName || undefined,
    category: liveEventCategory.value || rawPayload.value?.category || undefined,
    timeZone: timeZone.value,
    language: meetupLanguage.value || undefined,
    ...(Object.keys(customValues).length ? { placeholderValues: customValues } : {}),
  };
}

function buildDraftPayload(): DraftMeetupPayload {
  const customValues = collectPlaceholderOverrides();
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
    ...(resolvedDiscordChannelId()
      ? { discordChannelId: resolvedDiscordChannelId() }
      : {}),
    locationJitterMeters: draft.locationJitterMeters,
    location: { ...draftLocation.value! },
    language: meetupLanguage.value || undefined,
    ...(Object.keys(customValues).length ? { placeholderValues: customValues } : {}),
  };
}


function resetFormAfterDraft() {
  detailsTouched.value = false;
  languageDirty.value = false;
  beginProgrammaticUpdate();
  selectedLiveEventId.value = "";
  selectedTemplateId.value = "";
  if (selectedClub.value) {
    void blankDraftForSelection().then(() => {
      draftReady.value = true;
      nextTick(() => {
        autoSelectTemplate();
        endProgrammaticUpdate();
      });
    });
  } else {
    draftReady.value = false;
    nextTick(() => {
      autoSelectTemplate();
      endProgrammaticUpdate();
    });
  }
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
  hydrateGen++;
  draft.name = p.name;
  draft.details = p.details;
  draft.eventTime = p.eventTime;
  draft.eventEndTime = p.eventEndTime;
  draft.address = p.address;
  draft.coverPhotoUrl = p.coverPhotoUrl;
  draft.commentsPermissions = p.commentsPermissions;
  draft.allInvited = p.allInvited;
  draft.createdByCommunityAmbassador = p.createdByCommunityAmbassador ?? true;
  draft.discordChannelId = (p.discordChannelId || "").trim();
  draft.locationJitterMeters = p.locationJitterMeters;
  draft.campfireLiveEventId = p.liveEventId || "";
  const loc = p.location;
  draftLocation.value =
    loc && Number.isFinite(loc.lat) && Number.isFinite(loc.lng)
      ? { lat: loc.lat, lng: loc.lng }
      : null;
  if (p.clubId) selectedClubId.value = p.clubId;
  selectedLiveEventId.value = p.liveEventId || "";
  selectedTemplateId.value = resolveDraftTemplateId(p);
  languageDirty.value = false;
  if (isTemplateLanguageCode(p.language || "")) {
    meetupLanguage.value = p.language as TemplateLanguageCode;
    languageDirty.value = true;
  } else {
    applyMeetupLanguage(selectedTemplate.value?.language, { force: true });
  }
  for (const k of Object.keys(placeholderValues)) delete placeholderValues[k];
  eventPokemonDirty.value = new Set();
  for (const d of placeholderDefs.value) {
    placeholderValues[d.key] = "";
  }
  for (const key of eventPokemonNameDefs.value) {
    placeholderValues[key] = "";
  }
  for (const [k, v] of Object.entries(p.placeholderValues || {})) {
    if (typeof v !== "string") continue;
    const match = placeholderDefs.value.find(
      (d) => normalizePlaceholderKey(d.key) === normalizePlaceholderKey(k),
    );
    const key = match?.key || k;
    placeholderValues[key] = v;
    if (isEventPokemonNameKey(key) && v.trim()) {
      eventPokemonDirty.value.add(normalizePlaceholderKey(key));
    }
  }
  draftReady.value = true;
  syncDiscordChannelFromSettings();
  endProgrammaticUpdate();
  void refreshEventPokemonResolve();
}

async function applyDraftRoute() {
  const id = props.draftId;
  if (id == null || !Number.isFinite(id) || id <= 0) return;
  editingDraftId.value = id;
  hydrateGen++;
  let item = drafts.value.find((d) => d.id === id);
  if (!item) {
    await loadDrafts();
    item = drafts.value.find((d) => d.id === id);
  }
  if (!item) {
    error.value = "Draft not found";
    editingDraftId.value = null;
    await navigateTo("/create/drafts");
    return;
  }
  error.value = "";
  success.value = "";
  mode.value = "create";
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
    toast.success("Draft saved", payload.name || undefined);
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
  const phIssue = validatePlaceholdersForPost();
  if (phIssue) {
    error.value = phIssue;
    return;
  }
  confirmCreateOpen.value = true;
}

async function createMeetup() {
  confirmCreateOpen.value = false;
  const phIssue = validatePlaceholdersForPost();
  if (phIssue) {
    error.value = phIssue;
    return;
  }
  creating.value = true;
  error.value = "";
  success.value = "";
  try {
    const body = buildMeetupBodyFromDraft(
      buildDraftPayload(),
      timeZone.value,
      meetupLanguage.value || undefined,
    );
    const event = await api<{ id: string; name: string }>("/api/campfire/meetups", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify(body),
    });
    toast.success("Meetup posted", event.name || undefined);
    const postedDraftId = editingDraftId.value;
    if (postedDraftId != null) {
      try {
        await api(`/api/drafts/${postedDraftId}`, {
          method: "DELETE",
          headers: authHeaders(),
        });
        await loadDrafts();
      } catch {
        /* draft cleanup is best-effort */
      }
      editingDraftId.value = null;
    }
    resetFormAfterDraft();
    if (props.draftId != null) await navigateTo("/create");
    await nextTick();
    window.scrollTo({ top: 0, behavior: "smooth" });
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
  const phIssue = validatePlaceholdersForPost();
  if (phIssue) {
    error.value = phIssue;
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
    toast.success("Meetup updated", event.name || undefined);
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
    toast.success("Meetup deleted", name);
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
              {{ clubs.length ? "Select a club…" : "No clubs where you are an admin" }}
            </option>
            <option v-for="c in clubs" :key="c.id" :value="c.id">
              {{ c.name }}
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

        <div class="field setup-template-lang">
          <div class="field setup-template">
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
          </div>
          <div class="field setup-language">
            <label for="meetup-language">Language</label>
            <select
              id="meetup-language"
              v-model="meetupLanguage"
              title="Pokémon name translation; defaults from template or Settings"
              @change="onMeetupLanguageInput"
            >
              <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
                {{ l.label }}
              </option>
            </select>
          </div>
        </div>
      </div>

      <div v-if="selectionReady" class="field">
        <label>Placeholders</label>
        <div class="placeholder-fill placeholder-grid">
          <template v-if="eventPokemonNameDefs.length">
            <p class="hint muted" style="grid-column: 1 / -1; margin: 0">
              Event Pokémon
            </p>
            <div
              v-for="key in eventPokemonNameDefs"
              :key="`ep-${key}`"
              class="field ph-item"
            >
              <label :for="`ph-${key}`">
                {{ humanizePlaceholderKey(key) }}
                <span class="req">*</span>
              </label>
              <input
                :id="`ph-${key}`"
                v-model="placeholderValues[key]"
                type="text"
                required
                @input="onEventPokemonInput(key)"
              />
            </div>
          </template>
          <template v-if="placeholderDefs.length">
            <p class="hint muted" style="grid-column: 1 / -1; margin: 0">
              Custom
            </p>
            <div v-for="d in placeholderDefs" :key="d.key" class="field ph-item">
              <label :for="`ph-${d.key}`">
                {{ d.label || humanizePlaceholderKey(d.key) }}
                <span v-if="placeholderRequired(d)" class="req">*</span>
              </label>
              <input
                :id="`ph-${d.key}`"
                v-model="placeholderValues[d.key]"
                type="text"
                :required="placeholderRequired(d)"
                :placeholder="d.default || d.label || humanizePlaceholderKey(d.key)"
              />
            </div>
          </template>
        </div>
      </div>

      <div v-if="draftReady" class="editor">
        <div class="editor-main">
          <div class="details-head">
            <h3>Meetup details</h3>
            <div
              class="mode-toggle text-view-toggle"
              role="tablist"
              aria-label="Meetup text view"
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

          <div class="field">
            <label for="name">
              Title
              <span
                class="char-count muted"
                :class="{ over: resolvedNameLen > MAX_MEETUP_TITLE }"
              >
                {{ resolvedNameLen }} / {{ MAX_MEETUP_TITLE }}
              </span>
            </label>
            <PlaceholderTextField
              id="name"
              :model-value="textViewMode === 'edit' ? draft.name : previewText.name"
              :required="textViewMode === 'edit'"
              :readonly="textViewMode === 'preview'"
              @update:model-value="onTitleInput"
            />
          </div>

          <div class="field field-details">
            <label for="details">
              Description
              <span
                class="char-count muted"
                :class="{ over: resolvedDetailsLen > MAX_MEETUP_DETAILS }"
              >
                {{ resolvedDetailsLen }} / {{ MAX_MEETUP_DETAILS }}
              </span>
            </label>
            <PlaceholderTextField
              id="details"
              :model-value="textViewMode === 'edit' ? draft.details : previewText.details"
              multiline
              :rows="10"
              :readonly="textViewMode === 'preview'"
              @update:model-value="onDetailsInput"
            />
            <PlaceholderHelp compact />
            <p v-if="postLengthBlocked" class="error" style="margin-top: 0.35rem">
              Too long after placeholders (title ≤ {{ MAX_MEETUP_TITLE }}, details ≤
              {{ MAX_MEETUP_DETAILS }})
            </p>
          </div>

          <div class="row-2">
            <div class="field">
              <label for="start">Start</label>
              <input id="start" v-model="draft.eventTime" type="datetime-local" required />
            </div>
            <div class="field">
              <label for="end">End</label>
              <input
                id="end"
                v-model="draft.eventEndTime"
                type="datetime-local"
                required
                :min="minEndTime"
              />
            </div>
          </div>
          <p class="hint muted" style="margin-top: -0.35rem">
            {{ timeZone }}; min {{ MIN_MEETUP_DURATION_MINUTES }} min
            <template v-if="selectedLiveEvent && selectedLiveEventWindowLabel">
              · must be within {{ selectedLiveEventWindowLabel }}
            </template>
          </p>
          <p v-if="liveEventWindowError" class="error" style="margin-top: 0.35rem">
            {{ liveEventWindowError }}
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
            <label>Discord channel</label>
            <DiscordChannelPicker
              v-model="draft.discordChannelId"
              :club-id="selectedClubId"
              :preloaded="selectedClubDiscord"
              @loaded="onDiscordChannelsLoaded"
            />
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
            <CoverPhotoField v-model="draft.coverPhotoUrl" />
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
              Jitter: random offset so pins don’t stack (0 = exact)
            </p>
            <div class="field" style="margin-top: 0.75rem; margin-bottom: 0">
              <label for="address">Address</label>
              <input
                id="address"
                :value="textViewMode === 'edit' ? draft.address : previewText.address"
                type="text"
                :readonly="textViewMode === 'preview'"
                :class="{ 'preview-field': textViewMode === 'preview' }"
                placeholder="Optional — from place search"
                @input="onAddressInput"
              />
            </div>
          </div>
        </div>
      </div>

      <div v-if="draftReady" style="margin-top: 1.25rem; display: flex; flex-wrap: wrap; gap: 0.5rem">
        <template v-if="mode === 'create'">
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
            :disabled="creating || !canCreate || postLengthBlocked"
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
            :disabled="creating || !canSaveEdit || postLengthBlocked"
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
.setup-template-lang {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(7.5rem, 0.85fr);
  gap: 0.75rem 0.65rem;
  margin-bottom: 0 !important;
}
.setup-template-lang > .field {
  margin-bottom: 0 !important;
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
  min-height: 14rem;
  margin-bottom: 0.7rem !important;
}
.field-details :deep(.ph-field.multiline) {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 12rem;
}
.field-details :deep(.ph-field.multiline .ph-input),
.field-details :deep(.ph-field.multiline .ph-mirror),
.field-details textarea {
  flex: 1 1 auto;
  min-height: 12rem;
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
.req {
  color: var(--danger, #b91c1c);
  font-weight: 700;
}
.char-count {
  float: right;
  font-weight: 500;
  font-size: 0.8rem;
}
.char-count.over {
  color: var(--danger, #b91c1c);
  font-weight: 700;
}
@media (max-width: 900px) {
  .setup-grid {
    grid-template-columns: 1fr;
  }
  .setup-template-lang {
    grid-template-columns: 1fr;
  }
  .editor {
    grid-template-columns: 1fr;
  }
  .placeholder-grid {
    grid-template-columns: 1fr;
  }
  .field-details {
    min-height: 16rem;
  }
  .field-details :deep(.ph-field.multiline) {
    min-height: 14rem;
  }
  .field-details :deep(.ph-field.multiline .ph-input),
  .field-details :deep(.ph-field.multiline .ph-mirror),
  .field-details textarea {
    min-height: 14rem;
  }
  .editor-side :deep(.location-map) {
    height: 220px;
  }
}
</style>
