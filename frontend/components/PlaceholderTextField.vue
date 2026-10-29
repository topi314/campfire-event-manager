<script setup lang="ts">
import {
  BUILTIN_PLACEHOLDERS,
  isBuiltinPlaceholder,
  normalizePlaceholderKey,
} from "~/utils/placeholders";
import {
  applyLetterStyle,
  autocompleteRange,
  findStyledTokens,
  formatState,
  toggleFormat,
  type FormatAction,
  type FormatState,
  type LetterStyle,
} from "~/utils/unicodeStyle";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    id?: string;
    multiline?: boolean;
    rows?: number;
    required?: boolean;
    placeholder?: string;
    disabled?: boolean;
    readonly?: boolean;
    /** Custom {{key}} tokens offered in addition to built-ins. */
    extraPlaceholders?: { key: string; label?: string }[];
    /** Character count shown in the field footer. */
    count?: number | null;
    /** When set, the footer shows count / max and marks counts over the limit. */
    max?: number | null;
  }>(),
  {
    multiline: false,
    rows: 3,
    required: false,
    disabled: false,
    readonly: false,
    extraPlaceholders: () => [],
    count: null,
    max: null,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const inputRef = ref<HTMLInputElement | HTMLTextAreaElement | null>(null);
const mirrorRef = ref<HTMLElement | null>(null);
const wrapRef = ref<HTMLElement | null>(null);

const display = computed(() => props.modelValue ?? "");
const menuOpen = ref(false);
const browseOpen = ref(false);
const menuIndex = ref(0);
const activeQuery = ref("");
const replaceStart = ref(0);
const replaceEnd = ref(0);

const typing = reactive({
  style: "plain" as LetterStyle,
  underline: false,
  strike: false,
});
/** Set while a range is selected, so the toolbar reflects that range. */
const selectionState = ref<FormatState | null>(null);
const composing = ref(false);

const pressed = computed<FormatState>(() => {
  if (selectionState.value) return selectionState.value;
  return {
    bold: typing.style === "bold" || typing.style === "boldItalic",
    italic: typing.style === "italic" || typing.style === "boldItalic",
    mono: typing.style === "mono",
    underline: typing.underline,
    strike: typing.strike,
  };
});

const hintId = `emoji-${useId().replace(/[^a-zA-Z0-9-]/g, "")}`;
const os = ref<"windows" | "mac" | "linux" | "chromeos" | "other">("other");

const emojiShortcuts = [
  { id: "windows", name: "Windows", keys: "Win + ." },
  { id: "mac", name: "macOS", keys: "Ctrl + Cmd + Space" },
  { id: "linux", name: "Linux", keys: "Ctrl + ." },
  { id: "chromeos", name: "ChromeOS", keys: "Ctrl + Shift + Space" },
] as const;

onMounted(() => {
  const ua = navigator.userAgent || "";
  const platform = navigator.platform || "";
  if (/CrOS/.test(ua)) os.value = "chromeos";
  else if (/Win/.test(platform)) os.value = "windows";
  else if (/Mac/.test(platform)) os.value = "mac";
  else if (/Linux|X11/.test(platform)) os.value = "linux";
});

const actions: { id: FormatAction; label: string; title: string }[] = [
  { id: "bold", label: "B", title: "Bold (Ctrl+B)" },
  { id: "italic", label: "I", title: "Italic (Ctrl+I)" },
  { id: "mono", label: "M", title: "Monospace (Ctrl+M)" },
  { id: "underline", label: "U", title: "Underline (Ctrl+U)" },
  { id: "strike", label: "S", title: "Strikethrough (Ctrl+K)" },
];

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
  const tokens = findStyledTokens(raw);
  let out = "";
  let last = 0;
  for (const tok of tokens) {
    out += escapeHtml(raw.slice(last, tok.start));
    const cls = isBuiltinPlaceholder(tok.key) ? "tok-builtin" : "tok-custom";
    out += `<mark class="${cls}">${escapeHtml(raw.slice(tok.start, tok.end))}</mark>`;
    last = tok.end;
  }
  out += escapeHtml(raw.slice(last));
  if (raw.endsWith("\n")) out += "&nbsp;";
  return out || "&nbsp;";
});

const catalog = computed(() => {
  const seen = new Set<string>();
  const list: { key: string; label: string }[] = [];
  for (const b of BUILTIN_PLACEHOLDERS) {
    const norm = normalizePlaceholderKey(b.key);
    if (seen.has(norm)) continue;
    seen.add(norm);
    list.push({ key: b.key, label: b.label });
  }
  for (const extra of props.extraPlaceholders || []) {
    const key = (extra.key || "").trim();
    if (!key) continue;
    const norm = normalizePlaceholderKey(key);
    if (seen.has(norm)) continue;
    seen.add(norm);
    list.push({ key, label: extra.label?.trim() || key });
  }
  return list;
});

const suggestions = computed(() => {
  const q = activeQuery.value.toLowerCase();
  if (!q) return catalog.value;
  return catalog.value.filter((item) => item.key.toLowerCase().startsWith(q));
});

const menuItems = computed(() => (browseOpen.value ? catalog.value : suggestions.value));

function insertedText(prev: string, next: string): { start: number; text: string } | null {
  let start = 0;
  const max = Math.min(prev.length, next.length);
  while (start < max && prev[start] === next[start]) start++;
  let endPrev = prev.length;
  let endNext = next.length;
  while (endPrev > start && endNext > start && prev[endPrev - 1] === next[endNext - 1]) {
    endPrev--;
    endNext--;
  }
  if (endNext <= start) return null;
  return { start, text: next.slice(start, endNext) };
}

function styleTyped(text: string): string {
  if (typing.style === "plain" && !typing.underline && !typing.strike) return text;
  return applyLetterStyle(text, typing.style, typing.underline, typing.strike);
}

function onInput(e: Event) {
  const el = e.target as HTMLInputElement | HTMLTextAreaElement;
  const raw = el.value;
  if (composing.value) {
    emit("update:modelValue", raw);
    refreshCaret(el);
    return;
  }
  const change = insertedText(props.modelValue ?? "", raw);
  if (!change) {
    emit("update:modelValue", raw);
    refreshCaret(el);
    return;
  }
  const styled = styleTyped(change.text);
  if (styled === change.text) {
    emit("update:modelValue", raw);
    refreshCaret(el);
    return;
  }
  const updated = raw.slice(0, change.start) + styled + raw.slice(change.start + change.text.length);
  const caret = change.start + styled.length;
  emit("update:modelValue", updated);
  nextTick(() => {
    el.value = updated;
    el.setSelectionRange(caret, caret);
    refreshCaret(el);
  });
}

function onCompositionStart() {
  composing.value = true;
}

function onCompositionEnd(e: CompositionEvent) {
  composing.value = false;
  const el = e.target as HTMLInputElement | HTMLTextAreaElement;
  const data = e.data ?? "";
  if (!data) return;
  const styled = styleTyped(data);
  if (styled === data) return;
  const end = el.selectionEnd ?? el.value.length;
  const start = Math.max(0, end - data.length);
  const updated = el.value.slice(0, start) + styled + el.value.slice(end);
  const caret = start + styled.length;
  emit("update:modelValue", updated);
  nextTick(() => {
    el.focus();
    el.setSelectionRange(caret, caret);
    refreshCaret(el);
  });
}

function syncScroll() {
  if (!inputRef.value || !mirrorRef.value) return;
  mirrorRef.value.scrollTop = inputRef.value.scrollTop;
  mirrorRef.value.scrollLeft = inputRef.value.scrollLeft;
  if (menuOpen.value || browseOpen.value) placeMenu();
}

function typingFromState(state: FormatState) {
  if (state.mono) typing.style = "mono";
  else if (state.bold && state.italic) typing.style = "boldItalic";
  else if (state.bold) typing.style = "bold";
  else if (state.italic) typing.style = "italic";
  else typing.style = "plain";
  typing.underline = state.underline;
  typing.strike = state.strike;
}

const menuPos = ref({ top: 0, left: 0 });
const menuRef = ref<HTMLElement | null>(null);

function placeMenu() {
  const el = inputRef.value;
  const wrap = wrapRef.value;
  if (!el || !wrap) return;
  const start = el.selectionStart ?? 0;
  const cs = getComputedStyle(el);
  const mirror = document.createElement("div");
  const copy = [
    "fontFamily",
    "fontSize",
    "fontWeight",
    "fontStyle",
    "letterSpacing",
    "lineHeight",
    "paddingTop",
    "paddingRight",
    "paddingBottom",
    "paddingLeft",
    "borderTopWidth",
    "borderRightWidth",
    "borderBottomWidth",
    "borderLeftWidth",
    "boxSizing",
    "whiteSpace",
    "wordBreak",
    "overflowWrap",
  ] as const;
  mirror.style.position = "fixed";
  mirror.style.visibility = "hidden";
  mirror.style.pointerEvents = "none";
  mirror.style.whiteSpace = cs.whiteSpace;
  mirror.style.width = `${el.clientWidth}px`;
  for (const key of copy) mirror.style[key] = cs[key];
  mirror.textContent = el.value.slice(0, start);
  const mark = document.createElement("span");
  mark.textContent = "\u200b";
  mirror.appendChild(mark);
  document.body.appendChild(mirror);
  const elRect = el.getBoundingClientRect();
  const wrapRect = wrap.getBoundingClientRect();
  const top = elRect.top - wrapRect.top + mark.offsetTop - el.scrollTop + mark.offsetHeight;
  let left = elRect.left - wrapRect.left + mark.offsetLeft - el.scrollLeft;
  mirror.remove();
  const maxLeft = Math.max(8, wrap.clientWidth - 240);
  left = Math.min(Math.max(8, left), maxLeft);
  menuPos.value = { top: Math.max(0, top), left };
}

function refreshCaret(el: HTMLInputElement | HTMLTextAreaElement | null) {
  if (!el || props.readonly || props.disabled) {
    menuOpen.value = false;
    selectionState.value = null;
    return;
  }
  const start = el.selectionStart ?? 0;
  const end = el.selectionEnd ?? start;
  const value = el.value;
  selectionState.value = start === end ? null : formatState(value.slice(start, end));
  const range = autocompleteRange(value, end);
  if (!range || start !== end) {
    menuOpen.value = false;
    return;
  }
  const same =
    menuOpen.value &&
    activeQuery.value === range.query &&
    replaceStart.value === range.start &&
    replaceEnd.value === range.end;
  activeQuery.value = range.query;
  replaceStart.value = range.start;
  replaceEnd.value = range.end;
  if (browseOpen.value) return;
  menuOpen.value = true;
  if (!same) menuIndex.value = 0;
  placeMenu();
}

function applyAction(action: FormatAction) {
  const el = inputRef.value;
  if (!el || props.readonly || props.disabled) return;
  const start = el.selectionStart ?? 0;
  const end = el.selectionEnd ?? start;
  if (start === end) {
    const sample = applyLetterStyle("a", typing.style, typing.underline, typing.strike);
    typingFromState(formatState(toggleFormat(sample, action)));
    selectionState.value = null;
    return;
  }
  const value = props.modelValue ?? "";
  const next = toggleFormat(value.slice(start, end), action);
  typingFromState(formatState(next));
  const updated = value.slice(0, start) + next + value.slice(end);
  emit("update:modelValue", updated);
  const caretEnd = start + next.length;
  nextTick(() => {
    el.focus();
    el.setSelectionRange(start, caretEnd);
    refreshCaret(el);
  });
}

function onKeydown(e: KeyboardEvent) {
  if (props.readonly || props.disabled) return;
  if ((e.ctrlKey || e.metaKey) && !e.altKey) {
    const key = e.key.toLowerCase();
    if (key === "b") {
      e.preventDefault();
      applyAction("bold");
      return;
    }
    if (key === "i") {
      e.preventDefault();
      applyAction("italic");
      return;
    }
    if (key === "u") {
      e.preventDefault();
      applyAction("underline");
      return;
    }
    if (key === "m") {
      e.preventDefault();
      applyAction("mono");
      return;
    }
    if (key === "k") {
      e.preventDefault();
      applyAction("strike");
      return;
    }
  }
  if (!menuOpen.value && !browseOpen.value) return;
  if (e.key === "ArrowDown") {
    e.preventDefault();
    const n = menuItems.value.length;
    if (n) menuIndex.value = (menuIndex.value + 1) % n;
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    const n = menuItems.value.length;
    if (n) menuIndex.value = (menuIndex.value - 1 + n) % n;
    return;
  }
  if (e.key === "Enter" || e.key === "Tab") {
    const item = menuItems.value[menuIndex.value];
    if (!item) return;
    e.preventDefault();
    if (browseOpen.value) insertPlaceholder(item.key);
    else acceptSuggestion(item.key);
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    menuOpen.value = false;
    browseOpen.value = false;
  }
}

function acceptSuggestion(key: string) {
  const el = inputRef.value;
  if (!el) return;
  const value = props.modelValue ?? "";
  const insert = styleTyped(`{{${key}}}`);
  const updated = value.slice(0, replaceStart.value) + insert + value.slice(replaceEnd.value);
  emit("update:modelValue", updated);
  menuOpen.value = false;
  const caret = replaceStart.value + insert.length;
  nextTick(() => {
    el.focus();
    el.setSelectionRange(caret, caret);
    refreshCaret(el);
  });
}

function insertPlaceholder(key: string) {
  const el = inputRef.value;
  if (!el) return;
  const value = props.modelValue ?? "";
  const start = el.selectionStart ?? 0;
  const end = el.selectionEnd ?? start;
  const insert = styleTyped(`{{${key}}}`);
  const updated = value.slice(0, start) + insert + value.slice(end);
  emit("update:modelValue", updated);
  browseOpen.value = false;
  menuOpen.value = false;
  const caret = start + insert.length;
  nextTick(() => {
    el.focus();
    el.setSelectionRange(caret, caret);
    refreshCaret(el);
  });
}

function togglePlaceholderList() {
  browseOpen.value = !browseOpen.value;
  if (browseOpen.value) {
    menuOpen.value = false;
    menuIndex.value = 0;
    placeMenu();
  }
}

function onSelect(e?: Event) {
  if (e instanceof KeyboardEvent && (e.key === "ArrowDown" || e.key === "ArrowUp")) return;
  refreshCaret(inputRef.value);
}

watch(menuIndex, () => {
  nextTick(() => {
    const menu = menuRef.value;
    const item = menu?.querySelector("li.active");
    if (!menu || !(item instanceof HTMLElement)) return;
    const top = item.offsetTop;
    const bottom = top + item.offsetHeight;
    if (top < menu.scrollTop) menu.scrollTop = top;
    else if (bottom > menu.scrollTop + menu.clientHeight) menu.scrollTop = bottom - menu.clientHeight;
  });
});

watch(suggestions, (list) => {
  if (browseOpen.value) return;
  if (!list.length) menuOpen.value = false;
  else if (menuIndex.value >= list.length) menuIndex.value = 0;
});

function closeIfOutside(e: Event) {
  if (!menuOpen.value && !browseOpen.value) return;
  const target = e.target;
  if (!(target instanceof Node)) return;
  if (wrapRef.value?.contains(target)) return;
  menuOpen.value = false;
  browseOpen.value = false;
}

onMounted(() => {
  document.addEventListener("pointerdown", closeIfOutside, true);
});
onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", closeIfOutside, true);
});
</script>

<template>
  <div
    ref="wrapRef"
    class="ph-wrap"
    :class="{ framed: !readonly || count != null, 'menu-open': menuOpen || browseOpen }"
  >
    <div class="ph-field" :class="{ multiline, 'is-preview': readonly }">
      <div ref="mirrorRef" class="ph-mirror" aria-hidden="true" v-html="highlightedHtml" />
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
        :readonly="readonly"
        spellcheck="false"
        @input="onInput"
        @scroll="syncScroll"
        @keydown="onKeydown"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
        @select="onSelect"
        @keyup="onSelect"
        @click="onSelect"
        @blur="menuOpen = false; browseOpen = false"
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
        :readonly="readonly"
        spellcheck="false"
        @input="onInput"
        @scroll="syncScroll"
        @keydown="onKeydown"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
        @select="onSelect"
        @keyup="onSelect"
        @click="onSelect"
        @blur="menuOpen = false; browseOpen = false"
      />
    </div>
    <div v-if="!readonly || count != null" class="ph-toolbar">
      <div v-if="!readonly" class="ph-formats" role="toolbar" aria-label="Text formatting">
        <button
          v-for="action in actions"
          :key="action.id"
          type="button"
          class="fmt"
          :class="[action.id, { on: pressed[action.id] }]"
          :title="action.title"
          :aria-label="action.title"
          :aria-pressed="pressed[action.id]"
          :disabled="disabled"
          @mousedown.prevent
          @click="applyAction(action.id)"
        >
          {{ action.label }}
        </button>
        <button
          type="button"
          class="fmt placeholders"
          title="Insert placeholder"
          aria-label="Insert placeholder"
          :aria-expanded="browseOpen"
          :class="{ on: browseOpen }"
          :disabled="disabled"
          @mousedown.prevent
          @click="togglePlaceholderList"
        >
          {}
        </button>
        <button
          type="button"
          class="fmt emoji"
          title="Emoji shortcut"
          aria-label="Emoji shortcut"
          :popovertarget="hintId"
          :style="{ anchorName: `--${hintId}` }"
        >
          ☺
        </button>
        <div
          :id="hintId"
          popover
          class="emoji-hint"
          :style="{ positionAnchor: `--${hintId}` }"
        >
          <p>This page can’t open the emoji panel. Focus this field and use the shortcut.</p>
          <ul>
            <li v-for="item in emojiShortcuts" :key="item.id" :class="{ here: os === item.id }">
              <span>{{ item.name }}</span>
              <kbd>{{ item.keys }}</kbd>
            </li>
          </ul>
        </div>
      </div>
      <span
        v-if="count != null"
        class="ph-count"
        :class="{ over: max != null && count > max }"
      >
        {{ count }}<template v-if="max != null"> / {{ max }}</template>
      </span>
    </div>
    <ul
      v-if="(browseOpen || menuOpen) && menuItems.length"
      ref="menuRef"
      class="ph-menu"
      role="listbox"
      :style="{ top: `${menuPos.top}px`, left: `${menuPos.left}px` }"
    >
      <li
        v-for="(item, i) in menuItems"
        :key="item.key"
        role="option"
        :aria-selected="i === menuIndex"
        :class="{ active: i === menuIndex }"
        @mousedown.prevent="browseOpen ? insertPlaceholder(item.key) : acceptSuggestion(item.key)"
      >
        <code v-text="'{{' + item.key + '}}'" />
        <span class="muted">{{ item.label }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.ph-wrap {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-height: 0;
  flex: 1 1 auto;
  position: relative;
}
.ph-wrap.framed {
  gap: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 92%, transparent);
  overflow: visible;
}
.ph-wrap.menu-open {
  z-index: 20;
}
.ph-wrap.framed:focus-within {
  border-color: var(--accent, #3b82f6);
}
.ph-wrap.framed .ph-input:focus {
  outline: none;
  border-color: transparent;
}
.ph-toolbar {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.15rem 0.45rem 0.15rem 0.3rem;
  border-top: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg-elevated, var(--panel)) 55%, transparent);
}
.ph-wrap.framed .ph-toolbar {
  border-radius: 0 0 7px 7px;
}
.ph-formats {
  display: flex;
  flex-wrap: wrap;
  gap: 0.1rem;
}
.fmt {
  min-width: 1.75rem;
  height: 1.7rem;
  padding: 0 0.35rem;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--muted, #8b9aab);
  font: inherit;
  font-size: 0.85rem;
  line-height: 1;
  cursor: pointer;
}
.fmt:hover:not(:disabled) {
  border: none;
  background: color-mix(in srgb, var(--text) 8%, transparent);
  color: var(--text);
}
.fmt.bold {
  font-weight: 700;
}
.fmt.italic {
  font-style: italic;
}
.fmt.mono {
  font-family: ui-monospace, monospace;
}
.fmt.underline {
  text-decoration: underline;
}
.fmt.strike {
  text-decoration: line-through;
}
.fmt.on {
  border: none;
  color: var(--text);
  background: color-mix(in srgb, var(--accent, #3b82f6) 28%, transparent);
}
.fmt.emoji {
  font-size: 1rem;
}
.emoji-hint {
  position-area: block-start;
  margin: 0;
  inset: unset;
  width: max-content;
  max-width: 18rem;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-elevated, #1a222c);
  color: var(--text);
  box-shadow: 0 8px 24px rgb(0 0 0 / 28%);
}
.emoji-hint p {
  margin: 0 0 0.45rem;
  font-size: 0.8rem;
  line-height: 1.35;
}
.emoji-hint ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.emoji-hint li {
  display: flex;
  justify-content: space-between;
  gap: 0.8rem;
  font-size: 0.8rem;
  color: var(--muted, #8b9aab);
}
.emoji-hint li.here {
  color: var(--text);
  font-weight: 650;
}
.emoji-hint kbd {
  font-family: inherit;
  color: var(--text);
}
.fmt:disabled {
  opacity: 0.5;
  cursor: default;
}
.ph-count {
  margin-left: auto;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--muted, #8b9aab);
  font-variant-numeric: tabular-nums;
}
.ph-count.over {
  color: var(--danger, #e35d6a);
  font-weight: 700;
}
.ph-wrap.framed .ph-mirror,
.ph-wrap.framed .ph-input {
  border: none;
  border-radius: 0;
  background: transparent;
}
.ph-wrap.framed .ph-mirror {
  background: transparent;
}
.ph-field {
  position: relative;
  width: 100%;
}
.ph-field.is-preview .ph-mirror {
  background: color-mix(in srgb, var(--panel) 70%, transparent);
}
.ph-field.is-preview .ph-input {
  caret-color: transparent;
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
  font-variant-ligatures: none;
  font-feature-settings: "liga" 0, "calt" 0;
  padding: 0.45rem 0.6rem;
  border-radius: 8px;
  border: 1px solid var(--border);
  box-sizing: border-box;
  width: 100%;
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  overflow-wrap: break-word;
  scrollbar-gutter: stable;
}
.ph-mirror {
  position: absolute;
  inset: 0;
  pointer-events: none;
  color: var(--text);
  background: color-mix(in srgb, var(--panel) 92%, transparent);
  overflow: auto;
  scrollbar-width: none;
  z-index: 0;
}
.ph-mirror::-webkit-scrollbar {
  display: none;
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
  font: inherit;
  line-height: inherit;
  letter-spacing: inherit;
  padding: 0;
  margin: 0;
  border: 0;
  background: none;
}
.ph-mirror :deep(.tok-builtin) {
  color: color-mix(in srgb, var(--accent, #3d8bfd) 78%, var(--text));
}
.ph-mirror :deep(.tok-custom) {
  color: color-mix(in srgb, #e2b340 82%, var(--text));
}
.ph-menu {
  position: absolute;
  z-index: 30;
  width: max-content;
  min-width: 14rem;
  max-width: calc(100% - 1rem);
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  max-height: 12rem;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-elevated, #1a222c);
  box-shadow: 0 8px 24px rgb(0 0 0 / 35%);
}
.ph-menu li {
  display: flex;
  gap: 0.6rem;
  align-items: baseline;
  padding: 0.3rem 0.45rem;
  border-radius: 6px;
  cursor: pointer;
}
.ph-menu li.active {
  background: color-mix(in srgb, var(--accent, #3b82f6) 24%, transparent);
}
.ph-menu code {
  font-size: 0.85rem;
}
</style>
