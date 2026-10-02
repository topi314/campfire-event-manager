<script setup lang="ts">
const props = defineProps<{
  modelValue: string;
  authHeaders: Record<string, string>;
}>();

const emit = defineEmits<{
  "update:modelValue": [string];
}>();

const { apiBase } = useRuntimeConfig().public;
const { invalidateToken, openSettings } = useSessionToken();
const fileInput = ref<HTMLInputElement | null>(null);
const dropZone = ref<HTMLElement | null>(null);
const uploading = ref(false);
const removing = ref(false);
const pasting = ref(false);
const error = ref("");
const localPreview = ref("");
const photoId = ref("");
const dragOver = ref(false);

const previewSrc = computed(() => localPreview.value || props.modelValue || "");
const busy = computed(() => uploading.value || removing.value || pasting.value);

const statusLabel = computed(() => {
  if (uploading.value && !pasting.value) return "Uploading…";
  if (pasting.value) return "Pasting…";
  if (removing.value) return "Removing…";
  return "";
});

watch(
  () => props.modelValue,
  (v) => {
    if (!v) {
      localPreview.value = "";
      photoId.value = "";
    }
  },
);

function pickFile() {
  fileInput.value?.click();
}

function fileFromClipboardEvent(ev: ClipboardEvent): File | null {
  const items = ev.clipboardData?.items;
  if (items) {
    for (const item of items) {
      if (item.kind === "file" && item.type.startsWith("image/")) {
        return item.getAsFile();
      }
    }
  }
  const files = ev.clipboardData?.files;
  if (files) {
    for (const file of files) {
      if (file.type.startsWith("image/")) return file;
    }
  }
  return null;
}

function fileFromDrop(ev: DragEvent): File | null {
  const files = ev.dataTransfer?.files;
  if (!files?.length) return null;
  for (const file of files) {
    if (file.type.startsWith("image/")) return file;
  }
  return null;
}

async function uploadImageFile(file: File) {
  error.value = "";
  if (!props.authHeaders.Authorization) {
    error.value = "Add your Campfire JWT in Settings before uploading";
    openSettings({ focusToken: true });
    return;
  }
  if (!file.type.startsWith("image/")) {
    error.value = "Please choose an image file";
    return;
  }
  if (file.size > 8 << 20) {
    error.value = "Image must be 8MB or smaller";
    return;
  }

  uploading.value = true;
  try {
    localPreview.value = URL.createObjectURL(file);

    const fd = new FormData();
    fd.append("file", file, file.name);
    const res = await fetch(`${apiBase}/api/campfire/upload-image`, {
      method: "POST",
      headers: {
        ...props.authHeaders,
        Accept: "application/json",
      },
      credentials: "include",
      body: fd,
    });
    const text = await res.text();
    let data: {
      url?: string;
      permanentUrl?: string;
      photoId?: string;
      error?: string;
    } = {};
    try {
      data = JSON.parse(text);
    } catch {
      /* ignore */
    }
    if (!res.ok) {
      throw new Error(data.error || text || res.statusText);
    }
    const next = (data.permanentUrl || data.url || "").trim();
    if (!next) throw new Error("Upload succeeded but no URL returned");
    photoId.value = (data.photoId || "").trim();
    emit("update:modelValue", next);
    localPreview.value = next;
  } catch (e: any) {
    error.value = e.message || "Upload failed";
    localPreview.value = "";
    photoId.value = "";
    if (/invalid Campfire token|missing Campfire Authorization/i.test(e.message || "")) {
      invalidateToken();
    }
  } finally {
    uploading.value = false;
  }
}

async function onFile(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  await uploadImageFile(file);
}

async function pasteFromClipboard() {
  error.value = "";
  pasting.value = true;
  try {
    if (!navigator.clipboard?.read) {
      error.value = "Clipboard paste isn’t supported here — use Upload or drop a file";
      return;
    }
    const items = await navigator.clipboard.read();
    for (const item of items) {
      const type = item.types.find((t) => t.startsWith("image/"));
      if (!type) continue;
      const blob = await item.getType(type);
      const ext =
        type === "image/png" ? "png" : type === "image/webp" ? "webp" : "jpg";
      await uploadImageFile(new File([blob], `paste.${ext}`, { type }));
      return;
    }
    error.value = "No image in the clipboard — copy a screenshot or image first";
  } catch (e: any) {
    const msg = e?.message || String(e || "");
    if (/denied|permission|not allowed/i.test(msg)) {
      error.value = "Clipboard permission denied — allow paste, or use Upload";
    } else {
      error.value = "Could not read clipboard — copy an image and try again";
    }
  } finally {
    pasting.value = false;
  }
}

async function onPaste(ev: ClipboardEvent) {
  const file = fileFromClipboardEvent(ev);
  if (!file) return;
  ev.preventDefault();
  await uploadImageFile(file);
}

async function onDrop(ev: DragEvent) {
  ev.preventDefault();
  dragOver.value = false;
  const file = fileFromDrop(ev);
  if (!file) {
    error.value = "Drop an image file";
    return;
  }
  await uploadImageFile(file);
}

function onDragOver(ev: DragEvent) {
  ev.preventDefault();
  dragOver.value = true;
}

function onDragLeave(ev: DragEvent) {
  const next = ev.relatedTarget as Node | null;
  if (next && dropZone.value?.contains(next)) return;
  dragOver.value = false;
}

async function remove() {
  error.value = "";
  const id = photoId.value.trim();
  if (id && props.authHeaders.Authorization) {
    removing.value = true;
    try {
      const res = await fetch(
        `${apiBase}/api/campfire/upload-image?photoId=${encodeURIComponent(id)}`,
        {
          method: "DELETE",
          headers: {
            ...props.authHeaders,
            Accept: "application/json",
          },
          credentials: "include",
        },
      );
      if (!res.ok && res.status !== 204) {
        const text = await res.text();
        let data: { error?: string } = {};
        try {
          data = JSON.parse(text);
        } catch {
          /* ignore */
        }
        throw new Error(data.error || text || res.statusText);
      }
    } catch (e: any) {
      error.value = e.message || "Remove failed";
      removing.value = false;
      return;
    } finally {
      removing.value = false;
    }
  }
  emit("update:modelValue", "");
  localPreview.value = "";
  photoId.value = "";
}
</script>

<template>
  <div class="cover-photo">
    <input
      ref="fileInput"
      type="file"
      accept="image/*"
      hidden
      @change="onFile"
    />
    <div
      ref="dropZone"
      class="tile"
      :class="{
        empty: !previewSrc,
        busy,
        'drag-over': dragOver,
        'has-photo': !!previewSrc,
      }"
      tabindex="0"
      role="group"
      :aria-label="previewSrc ? 'Cover photo' : 'Add cover photo'"
      @paste="onPaste"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <img
        v-if="previewSrc"
        class="photo"
        :src="previewSrc"
        alt=""
        width="128"
        height="128"
      />

      <div v-if="!previewSrc && !statusLabel" class="empty-state" aria-hidden="true">
        <svg class="empty-icon" viewBox="0 0 24 24" fill="none">
          <rect
            x="3.5"
            y="5.5"
            width="17"
            height="13"
            rx="2.5"
            stroke="currentColor"
            stroke-width="1.5"
          />
          <circle cx="9" cy="10.5" r="1.6" fill="currentColor" />
          <path
            d="M5.5 16.5 9.2 13l2.3 2.2 3.2-3.7 3.8 5"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </div>

      <div v-if="statusLabel" class="status-layer">
        <span class="status">{{ statusLabel }}</span>
      </div>

      <div
        v-else
        class="chrome"
        :class="{ always: !previewSrc }"
      >
        <div class="toolbar">
          <button
            type="button"
            class="tool"
            :disabled="busy"
            :aria-label="previewSrc ? 'Replace photo' : 'Upload photo'"
            @click.stop="pickFile"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                fill="currentColor"
                d="M11 16V7.85l-2.6 2.6L7 9l5-5 5 5-1.4 1.45-2.6-2.6V16h-2Zm-5 4q-.825 0-1.412-.587Q4 18.825 4 18v-3h2v3h14v-3h2v3q0 .825-.587 1.413Q20.825 20 20 20H6Z"
              />
            </svg>
            <span>{{ previewSrc ? "Replace" : "Upload" }}</span>
          </button>
          <button
            type="button"
            class="tool accent"
            :disabled="busy"
            aria-label="Paste photo from clipboard"
            @click.stop="pasteFromClipboard"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                fill="currentColor"
                d="M9 18q-.825 0-1.412-.587Q7 16.825 7 16V4q0-.825.588-1.413Q8.175 2 9 2h9q.825 0 1.413.587Q20 3.175 20 4v12q0 .825-.587 1.413Q18.825 18 18 18Zm0-2h9V4H9v12Zm-4 6q-.825 0-1.412-.587Q3 20.825 3 20V6h2v14h11v2Z"
              />
            </svg>
            <span>Paste</span>
          </button>
        </div>
      </div>

      <button
        v-if="previewSrc"
        type="button"
        class="remove"
        :disabled="busy"
        title="Remove photo"
        aria-label="Remove photo"
        @click.stop="remove"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            fill="currentColor"
            d="M6.4 19 5 17.6 10.6 12 5 6.4 6.4 5 12 10.6 17.6 5 19 6.4 13.4 12 19 17.6 17.6 19 12 13.4Z"
          />
        </svg>
      </button>
    </div>

    <p class="hint muted">Optional. Drop an image onto the tile, or use Upload / Paste.</p>
    <p v-if="error" class="error" style="margin: 0">{{ error }}</p>
  </div>
</template>

<style scoped>
.cover-photo {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  align-items: flex-start;
}

.tile {
  position: relative;
  width: 128px;
  height: 128px;
  border-radius: 12px;
  border: 1px solid var(--border);
  overflow: hidden;
  background:
    radial-gradient(120% 90% at 20% 0%, color-mix(in srgb, var(--accent) 14%, transparent), transparent 55%),
    var(--bg-input);
  outline: none;
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease,
    background 0.15s ease;
}

.tile:focus-visible,
.tile.drag-over {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 28%, transparent);
}

.tile.drag-over {
  background:
    radial-gradient(120% 90% at 50% 50%, color-mix(in srgb, var(--accent) 22%, transparent), transparent 60%),
    var(--bg-input);
}

.tile.empty {
  border-style: dashed;
  border-color: color-mix(in srgb, var(--border) 80%, var(--muted));
}

.tile.has-photo {
  border-style: solid;
  background: #0a0e13;
}

.photo {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  display: block;
}

.empty-state {
  position: absolute;
  inset: 0 0 52px;
  display: grid;
  place-items: center;
  color: color-mix(in srgb, var(--muted) 85%, #fff);
  pointer-events: none;
}

.empty-icon {
  width: 34px;
  height: 34px;
  opacity: 0.9;
}

.status-layer {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, #0f1419 55%, transparent);
  z-index: 2;
}

.status {
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: #fff;
}

.chrome {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 1;
  padding: 0.35rem;
  background: linear-gradient(
    to top,
    color-mix(in srgb, #0b1016 88%, transparent),
    color-mix(in srgb, #0b1016 35%, transparent) 70%,
    transparent
  );
  opacity: 0;
  transform: translateY(4px);
  transition:
    opacity 0.15s ease,
    transform 0.15s ease;
  pointer-events: none;
}

.chrome.always,
.tile:hover .chrome,
.tile:focus-within .chrome,
.tile.drag-over .chrome,
.tile.busy .chrome {
  opacity: 1;
  transform: translateY(0);
  pointer-events: auto;
}

.toolbar {
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
}

.tool {
  margin: 0;
  width: 100%;
  min-width: 0;
  height: 1.75rem;
  padding: 0 0.45rem;
  border-radius: 7px;
  border: 1px solid color-mix(in srgb, #fff 12%, transparent);
  background: color-mix(in srgb, var(--bg-elevated) 82%, transparent);
  color: var(--text);
  font: inherit;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  gap: 0.35rem;
  justify-content: center;
  backdrop-filter: blur(8px);
}

.tool svg {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
}

.tool span {
  white-space: nowrap;
}

.tool.accent {
  background: color-mix(in srgb, var(--accent) 78%, #0f1419);
  border-color: color-mix(in srgb, var(--accent) 55%, transparent);
  color: #fff;
}

.tool:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--accent) 70%, transparent);
}

.tool.accent:hover:not(:disabled) {
  background: var(--accent-hover);
}

.remove {
  position: absolute;
  top: 0.35rem;
  right: 0.35rem;
  z-index: 2;
  width: 1.45rem;
  height: 1.45rem;
  padding: 0;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, #fff 14%, transparent);
  background: color-mix(in srgb, #0b1016 72%, transparent);
  color: #fff;
  backdrop-filter: blur(8px);
  opacity: 0;
  transition: opacity 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}

.remove svg {
  width: 14px;
  height: 14px;
}

.tile:hover .remove,
.tile:focus-within .remove {
  opacity: 1;
}

.remove:hover:not(:disabled) {
  border-color: var(--danger);
  color: var(--danger);
}

.hint {
  margin: 0;
  font-size: 0.8rem;
  max-width: 18rem;
}
</style>
