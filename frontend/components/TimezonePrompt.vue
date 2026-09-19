<script setup lang="ts">
const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();

const { setTimeZone, listTimeZones, browserTimeZone } = usePreferences();
const { openSettings } = useSessionToken();

const draftTz = ref("");
const zones = ref<string[]>([]);

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    draftTz.value = browserTimeZone();
    zones.value = listTimeZones();
    if (!zones.value.includes(draftTz.value)) {
      zones.value = [...zones.value, draftTz.value].sort((a, b) => a.localeCompare(b));
    }
  },
  { immediate: true },
);

function confirm() {
  setTimeZone(draftTz.value || browserTimeZone());
  emit("close");
}

function confirmAndOpenSettings() {
  setTimeZone(draftTz.value || browserTimeZone());
  emit("close");
  openSettings();
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.open) confirm();
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    aria-labelledby="tz-prompt-title"
    @click.self="confirm"
  >
    <div class="modal">
      <h2 id="tz-prompt-title">Choose your timezone</h2>
      <p class="muted">
        Meetup times are interpreted in this zone when you create or post events. You can change
        it later in Settings.
      </p>
      <div class="field">
        <label for="tz-prompt-select">Timezone</label>
        <select id="tz-prompt-select" v-model="draftTz">
          <option v-for="z in zones" :key="z" :value="z">{{ z }}</option>
        </select>
      </div>
      <div class="modal-actions">
        <button type="button" @click="confirmAndOpenSettings">Settings…</button>
        <button type="button" class="primary" @click="confirm">Continue</button>
      </div>
    </div>
  </div>
</template>
