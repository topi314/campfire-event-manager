<script setup lang="ts">
import type {
  Club,
  ClubDiscordSettings,
  MeetupPayload,
  TemplateOrigin,
  TemplatePlaceholder,
} from "~/types";
import type { LatLngPoint } from "~/components/LocationPicker.vue";
import {
  extractPlaceholdersFromValue,
  humanizePlaceholderKey,
  isBuiltinPlaceholder,
  normalizePlaceholderKey,
} from "~/utils/placeholders";
import {
  ensureMinMeetupEndTime,
  formatTimeOfDay,
  isMeetupTimeOfDayDurationValid,
  MIN_MEETUP_DURATION_MINUTES,
  parseTimeOfDay,
} from "~/utils/datetime";
import { DEFAULT_LOCATION_JITTER_METERS } from "~/utils/location";
import { CATEGORY_SELECT_CUSTOM } from "~/utils/eventCategory";
import {
  TEMPLATE_LANGUAGES,
  isTemplateLanguageCode,
  type TemplateLanguageCode,
} from "~/utils/languages";
import {
  snapshotFromOrigin,
  type OverrideFieldId,
} from "~/utils/templateOverrides";

const { options: TEMPLATE_CATEGORY_OPTIONS, isOption: isTemplateCategoryOption } =
  useEventCategories();

const props = withDefaults(
  defineProps<{
    modelValue?: MeetupPayload | null;
    templateName?: string;
    templateLanguage?: string | null;
    origin?: TemplateOrigin | null;
    overriddenFields?: string[] | null;
    saving?: boolean;
    submitLabel?: string;
  }>(),
  {
    modelValue: null,
    templateName: "",
    templateLanguage: null,
    origin: null,
    overriddenFields: null,
    saving: false,
    submitLabel: "Save template",
  },
);

const emit = defineEmits<{
  save: [
    data: {
      name: string;
      payload: MeetupPayload;
      language: TemplateLanguageCode;
      overriddenFields: string[];
    },
  ];
  cancel: [];
}>();

const { api } = useApi();
const { token, authHeaders } = useSessionToken();
const { preferredLanguage, preferredClubId } = usePreferences();
const clubs = ref<Club[]>([]);
/** Club whose Discord channels feed the channel picker (not required for Campfire create). */
const selectedDiscordClubId = ref("");

const linked = computed(() => !!props.origin?.id);
const overridden = ref<Set<string>>(new Set());
let suppressOverrideMark = false;

function isOverridden(field: OverrideFieldId) {
  return linked.value && overridden.value.has(field);
}

function markField(field: OverrideFieldId) {
  if (!linked.value || suppressOverrideMark) return;
  if (overridden.value.has(field)) return;
  const next = new Set(overridden.value);
  next.add(field);
  overridden.value = next;
}

function resetField(field: OverrideFieldId) {
  if (!props.origin) return;
  const snap = snapshotFromOrigin(props.origin, preferredLanguage.value);
  suppressOverrideMark = true;
  try {
    switch (field) {
      case "name":
        name.value = snap.name;
        break;
      case "language":
        language.value = isTemplateLanguageCode(String(snap.language))
          ? (snap.language as TemplateLanguageCode)
          : preferredLanguage.value;
        break;
      case "title":
        form.title = snap.title;
        break;
      case "details":
        form.details = snap.details;
        break;
      case "category":
        form.category = snap.category;
        categoryCustomMode.value =
          !!snap.category && !isTemplateCategoryOption(snap.category);
        break;
      case "coverPhotoUrl":
        form.coverPhotoUrl = snap.coverPhotoUrl;
        break;
      case "commentsPermissions":
        form.commentsPermissions = snap.commentsPermissions;
        break;
      case "allInvited":
        form.allInvited = snap.allInvited;
        break;
      case "createdByCommunityAmbassador":
        form.createdByCommunityAmbassador = snap.createdByCommunityAmbassador;
        break;
      case "placeholders":
        placeholderMeta.value = snap.placeholders.map((ph) => ({ ...ph }));
        break;
      case "location":
        form.address = snap.address;
        form.locationJitterMeters = snap.locationJitterMeters;
        location.value = snap.location ? { ...snap.location } : null;
        break;
      case "schedule":
        form.startTime = snap.startTime;
        form.endTime = snap.endTime;
        break;
      case "discord":
        form.discordChannelId = snap.discordChannelId;
        selectedDiscordClubId.value = pickDefaultDiscordClubId(
          clubs.value,
          snap.discordClubId,
        );
        break;
    }
  } finally {
    nextTick(() => {
      suppressOverrideMark = false;
    });
  }
  const next = new Set(overridden.value);
  next.delete(field);
  overridden.value = next;
}

function pickDefaultDiscordClubId(list: Club[], preferredStored?: string): string {
  const preferred = (preferredStored || preferredClubId.value || "").trim();
  if (preferred && list.some((c) => c.id === preferred)) return preferred;
  return list[0]?.id || "";
}

async function loadClubsForDiscord() {
  if (!token.value) {
    clubs.value = [];
    selectedDiscordClubId.value = "";
    return;
  }
  try {
    clubs.value = await api<Club[]>("/api/campfire/clubs", { headers: authHeaders() });
    if (
      !selectedDiscordClubId.value ||
      !clubs.value.some((c) => c.id === selectedDiscordClubId.value)
    ) {
      selectedDiscordClubId.value = pickDefaultDiscordClubId(clubs.value);
    }
  } catch {
    clubs.value = [];
    selectedDiscordClubId.value = "";
  }
}

onMounted(() => {
  void loadClubsForDiscord();
});

watch(token, () => {
  void loadClubsForDiscord();
});

const name = ref(props.templateName);
const language = ref<TemplateLanguageCode>(
  isTemplateLanguageCode(props.templateLanguage || "")
    ? (props.templateLanguage as TemplateLanguageCode)
    : preferredLanguage.value,
);
const form = reactive({
  title: "",
  details: "",
  category: "",
  address: "",
  coverPhotoUrl: "",
  commentsPermissions: "ORGANIZERS_ONLY",
  discordChannelId: "",
  startTime: "",
  endTime: "",
  allInvited: true,
  createdByCommunityAmbassador: true,
  locationJitterMeters: DEFAULT_LOCATION_JITTER_METERS,
});
const categoryCustomMode = ref(false);
const location = ref<LatLngPoint | null>(null);
const placeholderMeta = ref<TemplatePlaceholder[]>([]);
const timeError = ref("");

const selectedDiscordClub = computed(
  () => clubs.value.find((c) => c.id === selectedDiscordClubId.value) || null,
);

const selectedDiscordPreloaded = computed(
  () => selectedDiscordClub.value?.discord ?? null,
);

function onDiscordClubChange(event: Event) {
  selectedDiscordClubId.value = (event.target as HTMLSelectElement).value;
  form.discordChannelId = "";
  markField("discord");
}

function onTemplateDiscordLoaded(settings: ClubDiscordSettings) {
  const current = (form.discordChannelId || "").trim();
  if (!current) return;
  const ids = new Set((settings.channels || []).map((c) => c.id));
  if (settings.available && !ids.has(current)) {
    form.discordChannelId = "";
  }
}

const categorySelectValue = computed(() => {
  if (categoryCustomMode.value) return CATEGORY_SELECT_CUSTOM;
  const c = form.category.trim();
  if (!c) return "";
  if (isTemplateCategoryOption(c)) return c;
  return CATEGORY_SELECT_CUSTOM;
});

const showCustomCategory = computed(
  () => categorySelectValue.value === CATEGORY_SELECT_CUSTOM,
);

function onCategorySelect(event: Event) {
  const v = (event.target as HTMLSelectElement).value;
  markField("category");
  if (v === CATEGORY_SELECT_CUSTOM) {
    categoryCustomMode.value = true;
    if (!form.category.trim() || isTemplateCategoryOption(form.category.trim())) {
      form.category = "";
    }
    return;
  }
  categoryCustomMode.value = false;
  form.category = v;
}

const detectedKeys = computed(() =>
  extractPlaceholdersFromValue({
    name: form.title,
    details: form.details,
    address: form.address,
    coverPhotoUrl: form.coverPhotoUrl,
  }),
);

watch(detectedKeys, (keys) => {
  const customKeys = keys.filter((key) => !isBuiltinPlaceholder(key));
  const prev = new Map(
    placeholderMeta.value.map((p) => [normalizePlaceholderKey(p.key), p]),
  );
  placeholderMeta.value = customKeys.map((key) => {
    const existing = prev.get(normalizePlaceholderKey(key));
    return existing
      ? { key, label: existing.label || "", default: existing.default || "" }
      : { key, label: "", default: "" };
  });
});

watch(
  () =>
    [
      props.modelValue,
      props.templateName,
      props.templateLanguage,
      props.overriddenFields,
      props.origin?.id,
    ] as const,
  () => loadFromProps(),
  { immediate: true, deep: true },
);

function loadFromProps() {
  suppressOverrideMark = true;
  overridden.value = new Set(props.overriddenFields || []);
  name.value = props.templateName || "";
  language.value = isTemplateLanguageCode(props.templateLanguage || "")
    ? (props.templateLanguage as TemplateLanguageCode)
    : preferredLanguage.value;
  const p = props.modelValue;
  if (!p) {
    form.title = "";
    form.details = "";
    form.category = "";
    categoryCustomMode.value = false;
    form.address = "";
    form.coverPhotoUrl = "";
    form.commentsPermissions = "ORGANIZERS_ONLY";
    form.discordChannelId = "";
    form.startTime = "";
    form.endTime = "";
    form.allInvited = true;
    form.createdByCommunityAmbassador = true;
    form.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
    location.value = null;
    timeError.value = "";
    placeholderMeta.value = [];
    selectedDiscordClubId.value = pickDefaultDiscordClubId(clubs.value);
    nextTick(() => {
      suppressOverrideMark = false;
    });
    return;
  }
  form.title = p.name || "";
  form.details = p.details || "";
  const cat = (p.category || "").trim();
  form.category = cat;
  categoryCustomMode.value = !!cat && !isTemplateCategoryOption(cat);
  form.address = p.address || "";
  form.coverPhotoUrl = p.coverPhotoUrl || "";
  form.commentsPermissions = p.commentsPermissions || "ORGANIZERS_ONLY";
  form.discordChannelId = (p.discordChannelId || "").trim();
  selectedDiscordClubId.value = pickDefaultDiscordClubId(clubs.value, p.discordClubId);
  const startTod = parseTimeOfDay(p.startTime);
  const endTod = parseTimeOfDay(p.endTime);
  form.startTime = startTod ? formatTimeOfDay(startTod) : "";
  form.endTime = startTod
    ? ensureMinMeetupEndTime(
        form.startTime,
        endTod ? formatTimeOfDay(endTod) : "",
      )
    : endTod
      ? formatTimeOfDay(endTod)
      : "";
  form.allInvited = p.allInvited ?? true;
  form.createdByCommunityAmbassador = p.createdByCommunityAmbassador ?? true;
  form.locationJitterMeters =
    p.locationJitterMeters != null ? p.locationJitterMeters : DEFAULT_LOCATION_JITTER_METERS;
  if (p.latitude != null && p.longitude != null) {
    location.value = { lat: p.latitude, lng: p.longitude };
  } else {
    location.value = null;
  }
  timeError.value = "";
  placeholderMeta.value = (p.placeholders || []).map((ph) => ({
    key: ph.key,
    label: ph.label || "",
    default: ph.default || "",
  }));
  nextTick(() => {
    suppressOverrideMark = false;
  });
}

function onPlaceSelected(place: { lat: number; lng: number; label?: string }) {
  if (place.label) form.address = place.label;
  markField("location");
}

watch(
  () => form.startTime,
  (start) => {
    if (!start) return;
    form.endTime = ensureMinMeetupEndTime(start, form.endTime);
    timeError.value = "";
  },
);

watch(
  () => form.endTime,
  (end) => {
    if (!form.startTime) return;
    const next = ensureMinMeetupEndTime(form.startTime, end);
    if (next !== (end || "")) form.endTime = next;
    timeError.value = "";
  },
);

function buildPayload(): MeetupPayload {
  const placeholders = placeholderMeta.value
    .map((ph) => ({
      key: normalizePlaceholderKey(ph.key),
      label: ph.label.trim(),
      default: ph.default.trim(),
    }))
    .filter((ph) => ph.key && (ph.label || ph.default));

  return {
    name: form.title,
    details: form.details || undefined,
    category: form.category.trim() || undefined,
    latitude: location.value?.lat,
    longitude: location.value?.lng,
    locationJitterMeters: form.locationJitterMeters,
    address: form.address || undefined,
    coverPhotoUrl: form.coverPhotoUrl || undefined,
    commentsPermissions: form.commentsPermissions,
    ...(form.discordChannelId.trim()
      ? { discordChannelId: form.discordChannelId.trim() }
      : {}),
    ...(selectedDiscordClubId.value.trim()
      ? { discordClubId: selectedDiscordClubId.value.trim() }
      : {}),
    ...(form.startTime.trim() ? { startTime: form.startTime } : {}),
    ...(form.endTime.trim() ? { endTime: form.endTime } : {}),
    allInvited: form.allInvited,
    createdByCommunityAmbassador: form.createdByCommunityAmbassador,
    ...(placeholders.length ? { placeholders } : {}),
  };
}

function onSubmit() {
  const hasStart = !!form.startTime.trim();
  const hasEnd = !!form.endTime.trim();
  if (hasStart || hasEnd) {
    if (!hasStart || !hasEnd) {
      timeError.value = "Set both start and end, or leave both empty";
      return;
    }
    form.endTime = ensureMinMeetupEndTime(form.startTime, form.endTime);
    if (!isMeetupTimeOfDayDurationValid(form.startTime, form.endTime)) {
      timeError.value = `End time must be at least ${MIN_MEETUP_DURATION_MINUTES} minutes after start`;
      return;
    }
  }
  timeError.value = "";
  const tplName = name.value.trim() || form.title.trim() || "Untitled template";
  emit("save", {
    name: tplName,
    payload: buildPayload(),
    language: language.value,
    overriddenFields: linked.value ? [...overridden.value].sort() : [],
  });
}
</script>

<template>
  <form class="template-editor" @submit.prevent="onSubmit">
    <div class="setup-grid">
      <div class="field">
        <div class="field-label-row">
          <label for="tpl-name">Template name</label>
          <span v-if="isOverridden('name')" class="override-tools">
            <span class="override-badge">Custom</span>
            <button type="button" class="linkish" @click="resetField('name')">
              Reset to original
            </button>
          </span>
        </div>
        <input
          id="tpl-name"
          v-model="name"
          type="text"
          required
          placeholder="e.g. Community Day"
          @input="markField('name')"
        />
      </div>

      <div class="field">
        <div class="field-label-row">
          <label for="tpl-category">Live event category</label>
          <span v-if="isOverridden('category')" class="override-tools">
            <span class="override-badge">Custom</span>
            <button type="button" class="linkish" @click="resetField('category')">
              Reset to original
            </button>
          </span>
        </div>
        <select
          id="tpl-category"
          :value="categorySelectValue"
          @change="onCategorySelect"
        >
          <option value="">Any — show for all live events</option>
          <option v-for="cat in TEMPLATE_CATEGORY_OPTIONS" :key="cat" :value="cat">
            {{ cat }}
          </option>
          <option :value="CATEGORY_SELECT_CUSTOM">Custom…</option>
        </select>
        <input
          v-if="showCustomCategory"
          id="tpl-category-custom"
          v-model="form.category"
          type="text"
          required
          placeholder="Custom category name"
          style="margin-top: 0.45rem"
          aria-label="Custom live event category"
          @input="markField('category')"
        />
      </div>

      <div class="field">
        <div class="field-label-row">
          <label for="tpl-language">Language</label>
          <span v-if="isOverridden('language')" class="override-tools">
            <span class="override-badge">Custom</span>
            <button type="button" class="linkish" @click="resetField('language')">
              Reset to original
            </button>
          </span>
        </div>
        <select id="tpl-language" v-model="language" required @change="markField('language')">
          <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
            {{ l.label }}
          </option>
        </select>
      </div>
    </div>

    <div v-if="placeholderMeta.length" class="field">
      <div class="field-label-row">
        <label>Custom placeholders</label>
        <span v-if="isOverridden('placeholders')" class="override-tools">
          <span class="override-badge">Custom</span>
          <button type="button" class="linkish" @click="resetField('placeholders')">
            Reset to original
          </button>
        </span>
      </div>
      <div class="placeholder-fill placeholder-grid">
        <p class="hint muted" style="grid-column: 1 / -1; margin: 0">
          Label + default for create
        </p>
        <div v-for="ph in placeholderMeta" :key="ph.key" class="placeholder-row">
          <code v-text="'{{' + ph.key + '}}'" />
          <input
            v-model="ph.label"
            type="text"
            :placeholder="humanizePlaceholderKey(ph.key)"
            aria-label="Label"
            @input="markField('placeholders')"
          />
          <input
            v-model="ph.default"
            type="text"
            placeholder="Default value"
            aria-label="Default"
            @input="markField('placeholders')"
          />
        </div>
      </div>
    </div>

    <div class="editor">
      <div class="editor-main">
        <div class="details-head">
          <h3>Meetup details</h3>
        </div>

        <div class="field">
          <div class="field-label-row">
            <label for="tpl-title">Title</label>
            <span v-if="isOverridden('title')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('title')">
                Reset to original
              </button>
            </span>
          </div>
          <PlaceholderTextField
            id="tpl-title"
            v-model="form.title"
            required
            :placeholder="'{{liveEvent}} — {{city}}'"
            @update:model-value="markField('title')"
          />
        </div>

        <div class="field field-details">
          <div class="field-label-row">
            <label for="tpl-details">Description</label>
            <span v-if="isOverridden('details')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('details')">
                Reset to original
              </button>
            </span>
          </div>
          <PlaceholderTextField
            id="tpl-details"
            v-model="form.details"
            multiline
            :rows="10"
            @update:model-value="markField('details')"
          />
          <PlaceholderHelp compact />
        </div>

        <div class="field">
          <div class="field-label-row">
            <span class="label-text">Schedule</span>
            <span v-if="isOverridden('schedule')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('schedule')">
                Reset to original
              </button>
            </span>
          </div>
          <div class="row-2">
            <div class="field" style="margin-bottom: 0">
              <label for="tpl-start">Start time</label>
              <input
                id="tpl-start"
                v-model="form.startTime"
                type="time"
                @change="markField('schedule')"
              />
            </div>
            <div class="field" style="margin-bottom: 0">
              <label for="tpl-end">End time</label>
              <input
                id="tpl-end"
                v-model="form.endTime"
                type="time"
                :min="form.startTime || undefined"
                @change="markField('schedule')"
              />
            </div>
          </div>
        </div>
        <p v-if="timeError" class="hint" style="margin-top: -0.35rem; color: var(--danger, #b00020)">
          {{ timeError }}
        </p>
        <p class="hint muted" style="margin-top: -0.35rem">
          Optional · empty = category default (11-17 / 14-17 / 18-19) · day from live event
        </p>

        <div class="field">
          <div class="field-label-row">
            <label>Who to invite</label>
            <span v-if="isOverridden('allInvited')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('allInvited')">
                Reset to original
              </button>
            </span>
          </div>
          <InvitePicker
            :all-invited="form.allInvited"
            @update:all-invited="
              form.allInvited = $event;
              markField('allInvited');
            "
          />
          <p class="hint muted">Everybody = whole club; Nobody = unlisted</p>
        </div>

        <div class="field">
          <div class="field-label-row">
            <label>Who can comment</label>
            <span v-if="isOverridden('commentsPermissions')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('commentsPermissions')">
                Reset to original
              </button>
            </span>
          </div>
          <CommentsPermissionsPicker
            v-model="form.commentsPermissions"
            @update:model-value="markField('commentsPermissions')"
          />
        </div>

        <div class="field">
          <div class="field-label-row">
            <span class="label-text">Discord</span>
            <span v-if="isOverridden('discord')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('discord')">
                Reset to original
              </button>
            </span>
          </div>
          <div class="row-2">
            <div class="field" style="margin-bottom: 0">
              <label for="tpl-discord-club">Discord club</label>
              <select
                id="tpl-discord-club"
                :value="selectedDiscordClubId"
                :disabled="!clubs.length"
                @change="onDiscordClubChange"
              >
                <option v-if="!clubs.length" value="">
                  {{ token ? "No admin clubs" : "Campfire token required" }}
                </option>
                <option v-for="c in clubs" :key="c.id" :value="c.id">
                  {{ c.name }}
                </option>
              </select>
            </div>
            <div class="field" style="margin-bottom: 0">
              <label>Discord channel</label>
              <DiscordChannelPicker
                v-model="form.discordChannelId"
                :club-id="selectedDiscordClubId"
                :preloaded="selectedDiscordPreloaded"
                hint="Optional · from the club’s Discord integration"
                @loaded="onTemplateDiscordLoaded"
                @update:model-value="markField('discord')"
              />
            </div>
          </div>
        </div>

        <div class="field">
          <div class="field-label-row">
            <label class="check">
              <input
                v-model="form.createdByCommunityAmbassador"
                type="checkbox"
                @change="markField('createdByCommunityAmbassador')"
              />
              Mark as Community Ambassador event
            </label>
            <span v-if="isOverridden('createdByCommunityAmbassador')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button
                type="button"
                class="linkish"
                @click="resetField('createdByCommunityAmbassador')"
              >
                Reset to original
              </button>
            </span>
          </div>
        </div>
      </div>

      <div class="editor-side">
        <div class="field">
          <div class="field-label-row">
            <label>Cover photo (optional)</label>
            <span v-if="isOverridden('coverPhotoUrl')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('coverPhotoUrl')">
                Reset to original
              </button>
            </span>
          </div>
          <CoverPhotoField
            v-model="form.coverPhotoUrl"
            @update:model-value="markField('coverPhotoUrl')"
          />
        </div>
        <div class="field" style="margin-bottom: 0">
          <div class="field-label-row">
            <label>Location</label>
            <span v-if="isOverridden('location')" class="override-tools">
              <span class="override-badge">Custom</span>
              <button type="button" class="linkish" @click="resetField('location')">
                Reset to original
              </button>
            </span>
          </div>
          <ClientOnly>
            <LocationPicker
              v-model="location"
              :jitter-meters="form.locationJitterMeters"
              @place="onPlaceSelected"
              @update:model-value="markField('location')"
            >
              <template #meta>
                <div class="jitter-inline">
                  <label for="tpl-jitter" title="Random offset so repeated meetup pins don’t stack">
                    Jitter (m)
                  </label>
                  <input
                    id="tpl-jitter"
                    v-model.number="form.locationJitterMeters"
                    type="number"
                    min="0"
                    max="500"
                    step="5"
                    title="Random radius in meters around the pin. 0 keeps the exact coordinates."
                    @change="markField('location')"
                  />
                </div>
              </template>
            </LocationPicker>
          </ClientOnly>
          <p class="hint muted">
            Jitter: random offset so pins don’t stack (0 = exact)
          </p>
          <div class="field" style="margin-top: 0.75rem; margin-bottom: 0">
            <label for="tpl-address">Address</label>
            <input
              id="tpl-address"
              v-model="form.address"
              type="text"
              placeholder="Optional — from place search"
              @input="markField('location')"
            />
          </div>
        </div>
      </div>
    </div>

    <div class="actions">
      <button type="button" :disabled="saving" @click="emit('cancel')">Cancel</button>
      <button type="submit" class="primary" :disabled="saving">
        <Icon name="save" />
        {{ saving ? "Saving…" : submitLabel }}
      </button>
    </div>
  </form>
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
.template-editor {
  max-width: none;
}
.template-editor :deep(.field) {
  margin-bottom: 0.7rem;
}
.template-editor :deep(label) {
  margin-bottom: 0.2rem;
}
.template-editor :deep(input:not([type="radio"]):not([type="checkbox"])),
.template-editor :deep(textarea),
.template-editor :deep(select) {
  padding: 0.45rem 0.6rem;
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
  font-size: 0.95rem;
}
.setup-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.1fr) minmax(7.5rem, 0.75fr);
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
.editor-side #tpl-jitter {
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
  grid-template-columns: 1fr;
  gap: 0.5rem 0.75rem;
}
.placeholder-row {
  display: grid;
  grid-template-columns: minmax(5rem, auto) 1fr 1fr;
  gap: 0.5rem;
  align-items: center;
}
.placeholder-row code {
  font-size: 0.85rem;
  color: var(--muted);
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 1.25rem;
}
.field-label-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.35rem 0.75rem;
  margin-bottom: 0.2rem;
}
.field-label-row label,
.field-label-row .label-text {
  margin-bottom: 0;
  font-size: 0.85rem;
  font-weight: 600;
}
.override-tools {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.8rem;
}
.override-badge {
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent, #3d7eff) 18%, transparent);
  color: var(--text);
  font-size: 0.72rem;
  font-weight: 600;
}
.linkish {
  border: 0;
  background: none;
  padding: 0;
  color: var(--accent, #3d7eff);
  cursor: pointer;
  font: inherit;
  font-size: inherit;
  text-decoration: underline;
}
.linkish:hover {
  filter: brightness(1.08);
}
@media (max-width: 900px) {
  .setup-grid {
    grid-template-columns: 1fr;
  }
  .editor {
    grid-template-columns: 1fr;
  }
  .editor-side :deep(.location-map) {
    height: 220px;
  }
}
@media (max-width: 640px) {
  .placeholder-row {
    grid-template-columns: 1fr;
  }
}
</style>
