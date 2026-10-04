<script setup lang="ts">
import type { MeetupPayload, TemplatePlaceholder } from "~/types";
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
import {
  CATEGORY_SELECT_CUSTOM,
  TEMPLATE_CATEGORY_OPTIONS,
  isTemplateCategoryOption,
} from "~/utils/eventCategory";

const props = withDefaults(
  defineProps<{
    modelValue?: MeetupPayload | null;
    templateName?: string;
    saving?: boolean;
    submitLabel?: string;
  }>(),
  {
    modelValue: null,
    templateName: "",
    saving: false,
    submitLabel: "Save template",
  },
);

const emit = defineEmits<{
  save: [data: { name: string; payload: MeetupPayload }];
  cancel: [];
}>();

const name = ref(props.templateName);
const form = reactive({
  title: "",
  details: "",
  category: "",
  address: "",
  coverPhotoUrl: "",
  commentsPermissions: "ORGANIZERS_ONLY",
  startTime: "14:00",
  endTime: "17:00",
  allInvited: true,
  createdByCommunityAmbassador: true,
  locationJitterMeters: DEFAULT_LOCATION_JITTER_METERS,
});
const categoryCustomMode = ref(false);
const location = ref<LatLngPoint | null>(null);
const placeholderMeta = ref<TemplatePlaceholder[]>([]);
const timeError = ref("");

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
  () => [props.modelValue, props.templateName] as const,
  () => loadFromProps(),
  { immediate: true, deep: true },
);

function loadFromProps() {
  name.value = props.templateName || "";
  const p = props.modelValue;
  if (!p) {
    form.title = "";
    form.details = "";
    form.category = "";
    categoryCustomMode.value = false;
    form.address = "";
    form.coverPhotoUrl = "";
    form.commentsPermissions = "ORGANIZERS_ONLY";
    form.startTime = "14:00";
    form.endTime = "17:00";
    form.allInvited = true;
    form.createdByCommunityAmbassador = true;
    form.locationJitterMeters = DEFAULT_LOCATION_JITTER_METERS;
    location.value = null;
    timeError.value = "";
    placeholderMeta.value = [];
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
  form.startTime = formatTimeOfDay(parseTimeOfDay(p.startTime) ?? { hours: 14, minutes: 0 });
  form.endTime = ensureMinMeetupEndTime(
    form.startTime,
    formatTimeOfDay(parseTimeOfDay(p.endTime) ?? { hours: 17, minutes: 0 }),
  );
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
}

function onPlaceSelected(place: { lat: number; lng: number; label?: string }) {
  if (place.label) form.address = place.label;
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
    startTime: form.startTime,
    endTime: form.endTime,
    allInvited: form.allInvited,
    createdByCommunityAmbassador: form.createdByCommunityAmbassador,
    ...(placeholders.length ? { placeholders } : {}),
  };
}

function onSubmit() {
  form.endTime = ensureMinMeetupEndTime(form.startTime, form.endTime);
  if (!isMeetupTimeOfDayDurationValid(form.startTime, form.endTime)) {
    timeError.value = `End time must be at least ${MIN_MEETUP_DURATION_MINUTES} minutes after start`;
    return;
  }
  timeError.value = "";
  const tplName = name.value.trim() || form.title.trim() || "Untitled template";
  emit("save", { name: tplName, payload: buildPayload() });
}
</script>

<template>
  <form class="template-editor" @submit.prevent="onSubmit">
    <p class="muted steps-hint">
      Set a category → fill meetup details → save the template for later use
    </p>

    <div class="setup-grid">
      <div class="field">
        <label for="tpl-name">Template name</label>
        <input id="tpl-name" v-model="name" type="text" required placeholder="e.g. Community Day" />
      </div>

      <div class="field">
        <label for="tpl-category">Live event category (optional)</label>
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
        />
      </div>
    </div>

    <div v-if="placeholderMeta.length" class="field">
      <label>Custom placeholders</label>
      <div class="placeholder-fill placeholder-grid">
        <p class="hint muted" style="grid-column: 1 / -1; margin: 0">
          Optional label and default shown when creating a meetup.
        </p>
        <div v-for="ph in placeholderMeta" :key="ph.key" class="placeholder-row">
          <code v-text="'{{' + ph.key + '}}'" />
          <input
            v-model="ph.label"
            type="text"
            :placeholder="humanizePlaceholderKey(ph.key)"
            aria-label="Label"
          />
          <input v-model="ph.default" type="text" placeholder="Default value" aria-label="Default" />
        </div>
      </div>
    </div>

    <div class="editor">
      <div class="editor-main">
        <div class="details-head">
          <h3>Meetup details</h3>
        </div>

        <div class="field">
          <label for="tpl-title">Title</label>
          <input
            id="tpl-title"
            v-model="form.title"
            type="text"
            required
            :placeholder="'{{liveEvent}} — {{city}}'"
          />
        </div>

        <div class="field field-details">
          <label for="tpl-details">Description</label>
          <textarea id="tpl-details" v-model="form.details" rows="3" />
          <PlaceholderHelp compact />
        </div>

        <div class="row-2">
          <div class="field">
            <label for="tpl-start">Start time</label>
            <input id="tpl-start" v-model="form.startTime" type="time" required />
          </div>
          <div class="field">
            <label for="tpl-end">End time</label>
            <input
              id="tpl-end"
              v-model="form.endTime"
              type="time"
              required
              :min="form.startTime"
            />
          </div>
        </div>
        <p v-if="timeError" class="hint" style="margin-top: -0.35rem; color: var(--danger, #b00020)">
          {{ timeError }}
        </p>
        <p class="hint muted" style="margin-top: -0.35rem">
          Clock times in your Settings timezone (end at least
          {{ MIN_MEETUP_DURATION_MINUTES }} minutes after start). When you create a meetup, the
          calendar day comes from the live event (preferring Campfire’s local start date).
        </p>

        <div class="field">
          <label>Who to invite</label>
          <InvitePicker
            :all-invited="form.allInvited"
            @update:all-invited="form.allInvited = $event"
          />
          <p class="hint muted">Everybody invites the whole club; Nobody creates an unlisted meetup.</p>
        </div>

        <div class="field">
          <label>Who can comment</label>
          <CommentsPermissionsPicker v-model="form.commentsPermissions" />
        </div>

        <div class="field">
          <label class="check">
            <input v-model="form.createdByCommunityAmbassador" type="checkbox" />
            Mark as Community Ambassador event
          </label>
          <p class="hint muted">Default for meetups created from this template.</p>
        </div>
      </div>

      <div class="editor-side">
        <div class="field">
          <label>Cover photo (optional)</label>
          <CoverPhotoField v-model="form.coverPhotoUrl" />
        </div>
        <div class="field" style="margin-bottom: 0">
          <label>Location</label>
          <ClientOnly>
            <LocationPicker
              v-model="location"
              :jitter-meters="form.locationJitterMeters"
              @place="onPlaceSelected"
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
                  />
                </div>
              </template>
            </LocationPicker>
          </ClientOnly>
          <p class="hint muted">
            Jitter randomly offsets each meetup within this many meters of the pin so markers don’t stack. Use 0 for the exact spot.
          </p>
          <div class="field" style="margin-top: 0.75rem; margin-bottom: 0">
            <label for="tpl-address">Address</label>
            <input
              id="tpl-address"
              v-model="form.address"
              type="text"
              placeholder="Optional — filled from place search"
            />
            <p class="hint muted">Shown on the meetup; place search fills this when you pick a result.</p>
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
.steps-hint {
  margin: 0 0 0.85rem;
  font-size: 0.85rem;
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
  grid-template-columns: repeat(2, minmax(0, 1fr));
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
