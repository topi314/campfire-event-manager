<script setup lang="ts">
import type { ClubDiscordChannel, ClubDiscordSettings } from "~/types";

const props = defineProps<{
  modelValue: string;
  clubId: string;
  /** Optional override for the muted hint under the select. */
  hint?: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [string];
  loaded: [ClubDiscordSettings];
}>();

const { api } = useApi();
const { token, authHeaders } = useSessionToken();

const loading = ref(false);
const available = ref(false);
const channels = ref<ClubDiscordChannel[]>([]);
const defaultChannelId = ref("");
const loadedForClub = ref("");

const disabled = computed(
  () => !props.clubId || !available.value || loading.value,
);

const hintText = computed(() => {
  if (!props.clubId) return "Select a club to load Discord channels.";
  if (loading.value) return "Loading Discord channels…";
  if (!available.value) {
    return "Discord is not linked for this club (no channel settings).";
  }
  return props.hint || "Posted to this channel when the meetup is created.";
});

async function loadChannels(clubId: string) {
  const id = clubId.trim();
  if (!id || !token.value) {
    available.value = false;
    channels.value = [];
    defaultChannelId.value = "";
    loadedForClub.value = "";
    emit("loaded", { available: false, channels: [] });
    return;
  }

  loading.value = true;
  loadedForClub.value = id;
  try {
    const res = await api<ClubDiscordSettings>(
      `/api/campfire/clubs/${encodeURIComponent(id)}/discord`,
      { headers: authHeaders() },
    );
    if (loadedForClub.value !== id) return;
    available.value = !!res.available;
    channels.value = res.channels || [];
    defaultChannelId.value = (res.discordChannelId || "").trim();
    emit("loaded", {
      available: available.value,
      discordGuildId: res.discordGuildId,
      discordChannelId: defaultChannelId.value,
      channels: channels.value,
    });
  } catch {
    if (loadedForClub.value !== id) return;
    available.value = false;
    channels.value = [];
    defaultChannelId.value = "";
    emit("loaded", { available: false, channels: [] });
  } finally {
    if (loadedForClub.value === id) loading.value = false;
  }
}

watch(
  () => [props.clubId, token.value] as const,
  ([clubId]) => {
    void loadChannels(clubId || "");
  },
  { immediate: true },
);

function onChange(event: Event) {
  emit("update:modelValue", (event.target as HTMLSelectElement).value);
}

/** Emoji for Discord / Campfire channel type (numeric or enum string). */
function channelTypeEmoji(raw?: string): string {
  const t = (raw || "").trim();
  if (!t) return "💬";
  const key = t.toUpperCase().replace(/^CHANNEL_TYPE_/, "");
  const byName: Record<string, string> = {
    "0": "💬",
    GUILD_TEXT: "💬",
    TEXT: "💬",
    "1": "✉️",
    DM: "✉️",
    "2": "🔊",
    GUILD_VOICE: "🔊",
    VOICE: "🔊",
    "3": "✉️",
    GROUP_DM: "✉️",
    "4": "📁",
    GUILD_CATEGORY: "📁",
    CATEGORY: "📁",
    "5": "📢",
    GUILD_ANNOUNCEMENT: "📢",
    GUILD_NEWS: "📢",
    ANNOUNCEMENT: "📢",
    NEWS: "📢",
    "10": "🧵",
    ANNOUNCEMENT_THREAD: "🧵",
    "11": "🧵",
    PUBLIC_THREAD: "🧵",
    "12": "🧵",
    PRIVATE_THREAD: "🧵",
    "13": "🎭",
    GUILD_STAGE_VOICE: "🎭",
    "14": "📂",
    GUILD_DIRECTORY: "📂",
    "15": "🗂️",
    GUILD_FORUM: "🗂️",
    FORUM: "🗂️",
    "16": "🖼️",
    GUILD_MEDIA: "🖼️",
    MEDIA: "🖼️",
  };
  return byName[key] || "💬";
}

function channelOptionLabel(ch: ClubDiscordChannel): string {
  return `${channelTypeEmoji(ch.type)} ${ch.name}`;
}
</script>

<template>
  <div class="discord-channel-picker" :class="{ disabled }">
    <select
      :value="modelValue"
      :disabled="disabled"
      aria-label="Discord channel"
      @change="onChange"
    >
      <option value="">None</option>
      <option
        v-for="ch in channels"
        :key="ch.id"
        :value="ch.id"
      >
        {{ channelOptionLabel(ch) }}
      </option>
      <option
        v-if="modelValue && !channels.some((c) => c.id === modelValue)"
        :value="modelValue"
      >
        Saved channel ({{ modelValue }})
      </option>
    </select>
    <p class="hint muted">{{ hintText }}</p>
  </div>
</template>

<style scoped>
.discord-channel-picker {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.discord-channel-picker select {
  width: 100%;
  max-width: 100%;
}
.discord-channel-picker.disabled select {
  cursor: not-allowed;
  color: var(--muted);
  border-color: var(--border);
  background: var(--bg-input);
  opacity: 1;
}
.discord-channel-picker.disabled select:focus {
  outline: none;
  border-color: var(--border);
}
.hint {
  margin: 0;
  font-size: 0.85rem;
}
</style>
