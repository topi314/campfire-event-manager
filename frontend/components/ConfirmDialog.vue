<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    message: string;
    confirmLabel?: string;
    cancelLabel?: string;
    busy?: boolean;
    danger?: boolean;
  }>(),
  {
    confirmLabel: "Confirm",
    cancelLabel: "Cancel",
    busy: false,
    danger: false,
  },
);

const emit = defineEmits<{
  confirm: [];
  cancel: [];
}>();

function onKeydown(e: KeyboardEvent) {
  if (!props.open || props.busy) return;
  if (e.key === "Escape") emit("cancel");
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    @click.self="!busy && emit('cancel')"
  >
    <div class="modal" role="dialog" aria-modal="true" :aria-labelledby="'confirm-title'">
      <h2 id="confirm-title">{{ title }}</h2>
      <p class="muted">{{ message }}</p>
      <div class="modal-actions">
        <button type="button" :disabled="busy" @click="emit('cancel')">
          {{ cancelLabel }}
        </button>
        <button
          type="button"
          :class="danger ? 'danger' : 'primary'"
          :disabled="busy"
          @click="emit('confirm')"
        >
          {{ busy ? "…" : confirmLabel }}
        </button>
      </div>
    </div>
  </div>
</template>
