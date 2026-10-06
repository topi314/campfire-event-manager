<script setup lang="ts">
import { isBuiltinPlaceholder, PLACEHOLDER_RE } from "~/utils/placeholders";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    id?: string;
    multiline?: boolean;
    rows?: number;
    required?: boolean;
    placeholder?: string;
    disabled?: boolean;
  }>(),
  {
    multiline: false,
    rows: 3,
    required: false,
    disabled: false,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const inputRef = ref<HTMLInputElement | HTMLTextAreaElement | null>(null);
const mirrorRef = ref<HTMLElement | null>(null);

const display = computed(() => props.modelValue ?? "");

function escapeHtml(s: string) {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

const highlightedHtml = computed(() => {
  const raw = display.value;
  if (!raw) return "&nbsp;";
  PLACEHOLDER_RE.lastIndex = 0;
  let out = "";
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = PLACEHOLDER_RE.exec(raw)) !== null) {
    out += escapeHtml(raw.slice(last, m.index));
    const full = m[0];
    const key = m[1];
    const cls = isBuiltinPlaceholder(key) ? "tok-builtin" : "tok-custom";
    out += `<mark class="${cls}">${escapeHtml(full)}</mark>`;
    last = m.index + full.length;
  }
  out += escapeHtml(raw.slice(last));
  // Preserve trailing newline for textarea mirror height
  if (raw.endsWith("\n")) out += "&nbsp;";
  return out || "&nbsp;";
});

function onInput(e: Event) {
  const el = e.target as HTMLInputElement | HTMLTextAreaElement;
  emit("update:modelValue", el.value);
}

function syncScroll() {
  if (!inputRef.value || !mirrorRef.value) return;
  mirrorRef.value.scrollTop = inputRef.value.scrollTop;
  mirrorRef.value.scrollLeft = inputRef.value.scrollLeft;
}
</script>

<template>
  <div class="ph-field" :class="{ multiline }">
    <div
      ref="mirrorRef"
      class="ph-mirror"
      aria-hidden="true"
      v-html="highlightedHtml"
    />
    <textarea
      v-if="multiline"
      :id="id"
      ref="inputRef"
      class="ph-input"
      :value="modelValue"
      :rows="rows"
      :required="required"
      :placeholder="placeholder"
      :disabled="disabled"
      spellcheck="false"
      @input="onInput"
      @scroll="syncScroll"
    />
    <input
      v-else
      :id="id"
      ref="inputRef"
      class="ph-input"
      type="text"
      :value="modelValue"
      :required="required"
      :placeholder="placeholder"
      :disabled="disabled"
      spellcheck="false"
      @input="onInput"
      @scroll="syncScroll"
    />
  </div>
</template>

<style scoped>
.ph-field {
  position: relative;
  width: 100%;
}
.ph-field.multiline {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
}
.ph-mirror,
.ph-input {
  font: inherit;
  font-size: 0.95rem;
  line-height: 1.45;
  letter-spacing: inherit;
  padding: 0.45rem 0.6rem;
  border-radius: 8px;
  border: 1px solid var(--border);
  box-sizing: border-box;
  width: 100%;
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-wrap: break-word;
}
.ph-mirror {
  position: absolute;
  inset: 0;
  pointer-events: none;
  color: var(--text);
  background: color-mix(in srgb, var(--panel) 92%, transparent);
  overflow: hidden;
  z-index: 0;
}
.ph-input {
  position: relative;
  z-index: 1;
  color: transparent;
  caret-color: var(--text);
  background: transparent;
  overflow: auto;
  resize: vertical;
}
.ph-field.multiline .ph-input {
  flex: 1 1 auto;
  min-height: inherit;
  height: 100%;
}
.ph-field:not(.multiline) .ph-input {
  resize: none;
  white-space: nowrap;
  overflow-x: auto;
}
.ph-field:not(.multiline) .ph-mirror {
  white-space: pre;
  overflow: hidden;
}
.ph-mirror :deep(mark) {
  color: inherit;
  border-radius: 3px;
  padding: 0 0.05em;
}
.ph-mirror :deep(.tok-builtin) {
  background: color-mix(in srgb, var(--accent, #3b82f6) 28%, transparent);
}
.ph-mirror :deep(.tok-custom) {
  background: color-mix(in srgb, #ca8a04 32%, transparent);
}
</style>
