<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate } from "~/types";
import {
  BUILTIN_PLACEHOLDERS,
  extractPlaceholdersFromValue,
  isBuiltinPlaceholder,
} from "~/utils/placeholders";
import { templateLanguageLabel } from "~/utils/languages";

const props = withDefaults(
  defineProps<{
    template: MeetupTemplate;
    busy?: boolean;
    likeBusy?: boolean;
    /** shared: Add · mine: Edit */
    variant?: "shared" | "mine";
  }>(),
  {
    busy: false,
    likeBusy: false,
    variant: "shared",
  },
);

const emit = defineEmits<{
  close: [];
  add: [];
  edit: [];
  toggleLike: [];
}>();

const payload = computed((): MeetupPayload => {
  const p = props.template.payload;
  return (typeof p === "string" ? JSON.parse(p) : p) as MeetupPayload;
});

const cover = computed(() => (payload.value.coverPhotoUrl || "").trim());

const languageLabel = computed(() => templateLanguageLabel(props.template.language));

const usedKeys = computed(() =>
  extractPlaceholdersFromValue({
    name: payload.value.name,
    details: payload.value.details,
    address: payload.value.address,
    coverPhotoUrl: payload.value.coverPhotoUrl,
  }),
);

const usedBuiltins = computed(() => {
  const norms = new Set(
    usedKeys.value.filter(isBuiltinPlaceholder).map((k) => k.toLowerCase()),
  );
  return BUILTIN_PLACEHOLDERS.filter((b) => norms.has(b.key.toLowerCase()));
});

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") emit("close");
}

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div
      class="modal modal-preview"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="`preview-title-${template.id}`"
    >
      <div class="preview-head">
        <div>
          <h2 :id="`preview-title-${template.id}`">{{ template.name }}</h2>
          <p v-if="template.publisher" class="muted publisher">
            <span>by</span>
            <NuxtLink
              :to="`/profiles/${template.publisher.id}`"
              @click="emit('close')"
            >
              <img
                v-if="template.publisher.avatarUrl"
                :src="template.publisher.avatarUrl"
                alt=""
                class="avatar"
              />
              <span>{{ template.publisher.displayName || template.publisher.username }}</span>
            </NuxtLink>
          </p>
          <p v-else-if="variant === 'mine' && template.origin" class="muted published-tag">
            <template v-if="template.synced">Synced with origin — edits will unsync</template>
            <template v-else>Unsynced copy of origin</template>
          </p>
          <p v-else-if="template.publishedAt" class="muted published-tag">
            Published to Browse
          </p>
          <div
            v-if="variant === 'shared'"
            class="preview-likes"
          >
            <TemplateLikes
              :template="template"
              :creator-id="template.publisher?.id || template.discordUserId"
              :busy="likeBusy"
              @toggle="emit('toggleLike')"
            />
          </div>
        </div>
        <button type="button" class="icon-close" aria-label="Close" @click="emit('close')">
          ×
        </button>
      </div>

      <div class="preview-body">
        <img v-if="cover" class="cover" :src="cover" alt="" />

        <dl class="meta">
          <template v-if="template.publishDescription">
            <dt>Listing</dt>
            <dd class="pre">{{ template.publishDescription }}</dd>
          </template>
          <template v-if="payload.category">
            <dt>Category</dt>
            <dd>{{ payload.category }}</dd>
          </template>
          <template v-if="template.language">
            <dt>Language</dt>
            <dd>{{ languageLabel }}</dd>
          </template>
          <dt>Meetup title</dt>
          <dd>{{ payload.name || "—" }}</dd>
          <dt>Description</dt>
          <dd class="pre">{{ payload.details || "—" }}</dd>
          <dt>Time</dt>
          <dd>
            {{ payload.startTime || "—" }}
            <template v-if="payload.endTime"> – {{ payload.endTime }}</template>
          </dd>
        </dl>

        <div v-if="usedBuiltins.length || payload.placeholders?.length" class="placeholders">
          <h3>Placeholders</h3>
          <ul v-if="usedBuiltins.length">
            <li v-for="b in usedBuiltins" :key="b.key">
              <code v-text="'{{' + b.key + '}}'" />
              <span class="muted">{{ b.label }} · auto</span>
            </li>
          </ul>
          <ul v-if="payload.placeholders?.length">
            <li v-for="ph in payload.placeholders" :key="ph.key">
              <code v-text="'{{' + ph.key + '}}'" />
              <span v-if="ph.label" class="muted">{{ ph.label }}</span>
              <span v-if="ph.default" class="default">default: {{ ph.default }}</span>
            </li>
          </ul>
        </div>
      </div>

      <div class="modal-actions">
        <button type="button" @click="emit('close')">Close</button>
        <button
          v-if="variant === 'shared'"
          type="button"
          class="primary"
          :disabled="busy"
          @click="emit('add')"
        >
          <Icon name="add" />
          {{ busy ? "…" : "Add" }}
        </button>
        <button
          v-else
          type="button"
          class="primary"
          :disabled="busy"
          @click="emit('edit')"
        >
          <Icon name="edit" />
          Edit
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.preview-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 0.85rem;
}
.preview-head h2 {
  margin: 0;
  font-size: 1.15rem;
}
.published-tag {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
.preview-likes {
  margin-top: 0.55rem;
}
.publisher {
  margin: 0.35rem 0 0;
  font-size: 0.9rem;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
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
.icon-close {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  padding: 0;
  font-size: 1.35rem;
  line-height: 1;
}
.preview-body {
  max-height: min(60vh, 28rem);
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}
.cover {
  width: 100%;
  max-height: 10rem;
  object-fit: cover;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--bg-input);
}
.meta {
  display: grid;
  grid-template-columns: 7.5rem 1fr;
  gap: 0.4rem 0.75rem;
  margin: 0;
  font-size: 0.92rem;
}
.meta dt {
  margin: 0;
  color: var(--muted);
  font-weight: 500;
}
.meta dd {
  margin: 0;
  min-width: 0;
  word-break: break-word;
}
.meta dd.pre {
  white-space: pre-wrap;
}
.placeholders h3 {
  margin: 0 0 0.4rem;
  font-size: 0.95rem;
}
.placeholders ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
.placeholders li {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem 0.65rem;
  align-items: baseline;
  padding: 0.35rem 0;
  border-bottom: 1px solid var(--border);
  font-size: 0.88rem;
}
.placeholders li:last-child {
  border-bottom: none;
}
.placeholders .default {
  color: var(--muted);
}
@media (max-width: 520px) {
  .meta {
    grid-template-columns: 1fr;
    gap: 0.15rem;
  }
  .meta dt {
    margin-top: 0.35rem;
  }
}
</style>
