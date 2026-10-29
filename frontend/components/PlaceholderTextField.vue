<script setup lang="ts">
import {
  Annotation,
  Compartment,
  EditorSelection,
  EditorState,
  Prec,
  RangeSetBuilder,
  Transaction,
  type Extension,
  type TransactionSpec,
} from "@codemirror/state";
import {
  Decoration,
  EditorView,
  keymap,
  placeholder as cmPlaceholder,
  drawSelection,
  ViewPlugin,
  type ViewUpdate,
} from "@codemirror/view";
import {
  acceptCompletion,
  autocompletion,
  closeCompletion,
  completionStatus,
  CompletionContext,
  startCompletion,
  type CompletionResult,
} from "@codemirror/autocomplete";
import {
  history,
  insertNewline,
  redo,
  standardKeymap,
  undo,
} from "@codemirror/commands";
import {
  BUILTIN_PLACEHOLDERS,
  filterPlaceholderSuggestions,
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
    /**
     * Resolved event-Pokémon values from the selected event.
     * Null on a template, where no species list exists yet.
     */
    eventPokemonValues?: Record<string, string> | null;
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
    eventPokemonValues: null,
    count: null,
    max: null,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const hostRef = ref<HTMLElement | null>(null);
let editor: EditorView | null = null;
let fillObserver: ResizeObserver | null = null;
let composeActive = false;
let composeBase: string | null = null;

const skipStyle = Annotation.define<boolean>();
const editableComp = new Compartment();
const placeholderComp = new Compartment();

const typing = reactive({
  style: "plain" as LetterStyle,
  underline: false,
  strike: false,
});
/** Set while a range is selected, so the toolbar reflects that range. */
const selectionState = ref<FormatState | null>(null);
const menuOpen = ref(false);

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

const actions: { id: FormatAction; label: string; title: string }[] = [
  { id: "bold", label: "B", title: "Bold (Ctrl+B)" },
  { id: "italic", label: "I", title: "Italic (Ctrl+I)" },
  { id: "mono", label: "M", title: "Monospace (Ctrl+M)" },
  { id: "underline", label: "U", title: "Underline (Ctrl+U)" },
  { id: "strike", label: "S", title: "Strikethrough (Ctrl+K)" },
];

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

function styleTyped(text: string): string {
  if (typing.style === "plain" && !typing.underline && !typing.strike) return text;
  return applyLetterStyle(text, typing.style, typing.underline, typing.strike);
}

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

function typingFromState(state: FormatState) {
  if (state.mono) typing.style = "mono";
  else if (state.bold && state.italic) typing.style = "boldItalic";
  else if (state.bold) typing.style = "bold";
  else if (state.italic) typing.style = "italic";
  else typing.style = "plain";
  typing.underline = state.underline;
  typing.strike = state.strike;
}

function locked() {
  return props.readonly || props.disabled;
}

function placeholderCompletions(context: CompletionContext): CompletionResult | null {
  if (locked()) return null;
  const range = autocompleteRange(context.state.doc.toString(), context.pos);
  if (!range && !context.explicit) return null;
  const items = filterPlaceholderSuggestions(
    catalog.value,
    range?.query ?? "",
    props.eventPokemonValues,
  );
  if (!items.length) return null;
  const from = range ? range.start : context.state.selection.main.from;
  const to = range ? range.end : context.state.selection.main.to;
  const result: CompletionResult = {
    from,
    to,
    filter: false,
    options: items.map((item) => ({
      label: `{{${item.key}}}`,
      detail: item.label && item.label !== item.key ? item.label : undefined,
      apply(view, _completion, fromPos, toPos) {
        const insert = styleTyped(`{{${item.key}}}`);
        view.dispatch({
          changes: { from: fromPos, to: toPos, insert },
          selection: { anchor: fromPos + insert.length },
          annotations: skipStyle.of(true),
          userEvent: "input.complete",
        });
      },
    })),
  };
  const explicit = context.explicit;
  result.update = (_current, _from, _to, ctx) => {
    const next = explicit ? new CompletionContext(ctx.state, ctx.pos, true, ctx.view) : ctx;
    return placeholderCompletions(next);
  };
  return result;
}

function rewriteTransaction(tr: Transaction): Transaction | TransactionSpec {
  if (tr.annotation(skipStyle) || !tr.docChanged || composeActive) return tr;

  const styleInput =
    !locked() && (tr.isUserEvent("input.type") || tr.isUserEvent("input.paste"));
  const stripNewlines = !props.multiline;
  const plainTyping = typing.style === "plain" && !typing.underline && !typing.strike;
  if (!stripNewlines && (!styleInput || plainTyping)) return tr;

  const parts: { from: number; to: number; insert: string }[] = [];
  const spans: { fromB: number; toB: number; growth: number }[] = [];
  let changed = false;
  tr.changes.iterChanges((from, to, fromB, toB, inserted) => {
    let text = inserted.toString();
    if (stripNewlines && /[\r\n]/.test(text)) text = text.replace(/\r?\n/g, " ");
    const next = styleInput && !plainTyping ? styleTyped(text) : text;
    if (next !== inserted.toString()) changed = true;
    parts.push({ from, to, insert: next });
    spans.push({ fromB, toB, growth: next.length - inserted.length });
  });
  if (!changed) return tr;

  let newLen = tr.startState.doc.length;
  for (const part of parts) newLen += part.insert.length - (part.to - part.from);
  const mapPos = (pos: number) => {
    let shift = 0;
    for (const span of spans) {
      if (pos <= span.fromB) return pos + shift;
      if (pos < span.toB) return span.toB + shift + span.growth;
      shift += span.growth;
    }
    return pos + shift;
  };
  const clamp = (pos: number) => Math.max(0, Math.min(newLen, mapPos(pos)));

  return {
    changes: parts,
    selection: EditorSelection.create(
      tr.selection.ranges.map((range) =>
        EditorSelection.range(clamp(range.anchor), clamp(range.head)),
      ),
      tr.selection.mainIndex,
    ),
    annotations: tr.annotations.concat(skipStyle.of(true)),
    scrollIntoView: tr.scrollIntoView,
  };
}

function styleCommitted(view: EditorView, base: string) {
  if (locked()) return;
  if (typing.style === "plain" && !typing.underline && !typing.strike) return;
  const next = view.state.doc.toString();
  const change = insertedText(base, next);
  if (!change) return;
  const styled = styleTyped(change.text);
  if (styled === change.text) return;
  const from = change.start;
  const to = change.start + change.text.length;
  if (view.state.sliceDoc(from, to) !== change.text) return;
  view.dispatch({
    changes: { from, to, insert: styled },
    selection: { anchor: from + styled.length },
    annotations: skipStyle.of(true),
    userEvent: "input.type",
  });
}

function syncChrome(state: EditorState) {
  const sel = state.selection.main;
  selectionState.value = sel.empty ? null : formatState(state.sliceDoc(sel.from, sel.to));
  menuOpen.value = completionStatus(state) === "active";
}

function applyAction(action: FormatAction) {
  const view = editor;
  if (!view || locked()) return;
  const sel = view.state.selection.main;
  if (sel.empty) {
    const sample = applyLetterStyle("a", typing.style, typing.underline, typing.strike);
    typingFromState(formatState(toggleFormat(sample, action)));
    selectionState.value = null;
    view.focus();
    return;
  }
  const slice = view.state.sliceDoc(sel.from, sel.to);
  const next = toggleFormat(slice, action);
  typingFromState(formatState(next));
  view.dispatch({
    changes: { from: sel.from, to: sel.to, insert: next },
    selection: { anchor: sel.from, head: sel.from + next.length },
    annotations: skipStyle.of(true),
  });
  view.focus();
}

function bindFormat(key: string, action: FormatAction) {
  return {
    key,
    preventDefault: true,
    run() {
      if (locked()) return false;
      applyAction(action);
      return true;
    },
  };
}

function tokenDecorations(state: EditorState) {
  const builder = new RangeSetBuilder<Decoration>();
  for (const tok of findStyledTokens(state.doc.toString())) {
    if (tok.end <= tok.start) continue;
    const cls = isBuiltinPlaceholder(tok.key) ? "cm-tok-builtin" : "cm-tok-custom";
    builder.add(tok.start, tok.end, Decoration.mark({ class: cls }));
  }
  return builder.finish();
}

const tokenMarks = ViewPlugin.fromClass(
  class {
    decorations;
    constructor(view: EditorView) {
      this.decorations = tokenDecorations(view.state);
    }
    update(update: ViewUpdate) {
      if (update.docChanged) this.decorations = tokenDecorations(update.state);
    }
  },
  { decorations: (plugin) => plugin.decorations },
);

function lockExtension(): Extension {
  const off = locked();
  return [EditorState.readOnly.of(off), EditorView.editable.of(!off)];
}

function contentAttrs(): Record<string, string> {
  const attrs: Record<string, string> = {
    "aria-multiline": props.multiline ? "true" : "false",
  };
  if (props.id) attrs.id = props.id;
  if (props.required) attrs["aria-required"] = "true";
  if (props.disabled) attrs.tabindex = "-1";
  return attrs;
}

function editorTheme() {
  return EditorView.theme(
    {
      "&": {
        fontSize: "0.95rem",
        backgroundColor: "transparent",
        color: "var(--text)",
        width: "100%",
      },
      "&.cm-focused": { outline: "none" },
      ".cm-scroller": {
        fontFamily: "inherit",
        lineHeight: "1.45",
        fontVariantLigatures: "none",
        flex: props.multiline ? "1 1 auto" : "0 0 auto",
        minHeight: props.multiline ? "0" : "auto",
        overflowX: "auto",
        overflowY: props.multiline ? "auto" : "hidden",
      },
      ".cm-content": {
        fontFamily: "inherit",
        fontVariantLigatures: "none",
        fontFeatureSettings: '"liga" 0, "calt" 0',
        padding: "0.45rem 0.6rem",
        caretColor: "var(--text)",
        minHeight: props.multiline ? "100%" : "auto",
      },
      ".cm-line": { padding: "0" },
      ".cm-placeholder": {
        color: "var(--muted, #8b9aab)",
        fontFamily: "inherit",
      },
      ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--text)" },
      "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground":
        {
          backgroundColor: "color-mix(in srgb, var(--accent, #3d8bfd) 45%, transparent)",
        },
      ".cm-tok-builtin": {
        color: "color-mix(in srgb, var(--accent, #3d8bfd) 78%, var(--text))",
      },
      ".cm-tok-custom": {
        color: "color-mix(in srgb, #e2b340 82%, var(--text))",
      },
    },
    { dark: true },
  );
}

function editorExtensions(theme: Extension): Extension {
  const formatKeys = [
    bindFormat("Mod-b", "bold"),
    bindFormat("Ctrl-b", "bold"),
    bindFormat("Mod-i", "italic"),
    bindFormat("Ctrl-i", "italic"),
    bindFormat("Mod-u", "underline"),
    bindFormat("Ctrl-u", "underline"),
    bindFormat("Mod-m", "mono"),
    bindFormat("Ctrl-m", "mono"),
    bindFormat("Mod-k", "strike"),
    bindFormat("Ctrl-k", "strike"),
    {
      key: "Enter",
      run(view: EditorView) {
        if (props.multiline) return insertNewline(view);
        const form = view.dom.closest("form");
        if (form instanceof HTMLFormElement) form.requestSubmit();
        return true;
      },
      shift(view: EditorView) {
        if (props.multiline) return insertNewline(view);
        return true;
      },
    },
    { key: "Tab", run: acceptCompletion },
  ];

  return [
    editableComp.of(lockExtension()),
    placeholderComp.of(props.placeholder ? cmPlaceholder(props.placeholder) : []),
    history(),
    drawSelection(),
    props.multiline ? EditorView.lineWrapping : [],
    tokenMarks,
    autocompletion({
      override: [placeholderCompletions],
      activateOnTyping: true,
      selectOnOpen: true,
      icons: false,
      tooltipClass: () => "ph-completion",
    }),
    keymap.of(standardKeymap),
    keymap.of([
      { key: "Mod-z", run: undo, preventDefault: true },
      { key: "Mod-y", mac: "Mod-Shift-z", run: redo, preventDefault: true },
      { key: "Mod-Shift-z", run: redo, preventDefault: true },
    ]),
    Prec.high(keymap.of(formatKeys)),
    EditorState.transactionFilter.of(rewriteTransaction),
    EditorView.domEventHandlers({
      compositionstart(_event, view) {
        composeActive = true;
        composeBase = view.state.doc.toString();
        return false;
      },
      compositionend(_event, view) {
        const base = composeBase;
        composeActive = false;
        composeBase = null;
        if (base == null) return false;
        requestAnimationFrame(() => {
          requestAnimationFrame(() => {
            if (view.dom.isConnected) styleCommitted(view, base);
          });
        });
        return false;
      },
    }),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const text = update.state.doc.toString();
        if (text !== (props.modelValue ?? "")) emit("update:modelValue", text);
      }
      if (update.docChanged || update.selectionSet || update.focusChanged) syncChrome(update.state);
    }),
    EditorView.contentAttributes.of(contentAttrs()),
    EditorView.editorAttributes.of({ class: "ph-input" }),
    theme,
  ];
}

onMounted(() => {
  const ua = navigator.userAgent || "";
  const platform = navigator.platform || "";
  if (/CrOS/.test(ua)) os.value = "chromeos";
  else if (/Win/.test(platform)) os.value = "windows";
  else if (/Mac/.test(platform)) os.value = "mac";
  else if (/Linux|X11/.test(platform)) os.value = "linux";

  const parent = hostRef.value;
  if (!parent) return;
  editor = new EditorView({
    parent,
    state: EditorState.create({
      doc: props.modelValue ?? "",
      extensions: editorExtensions(editorTheme()),
    }),
  });
  syncChrome(editor.state);
  if (props.multiline) {
    const fill = () => {
      if (!editor) return;
      const next = `${editor.scrollDOM.clientHeight}px`;
      if (editor.contentDOM.style.minHeight !== next) editor.contentDOM.style.minHeight = next;
    };
    fillObserver = new ResizeObserver(fill);
    fillObserver.observe(editor.scrollDOM);
    fill();
  }
});

onBeforeUnmount(() => {
  fillObserver?.disconnect();
  fillObserver = null;
  editor?.destroy();
  editor = null;
});

watch(
  () => props.modelValue,
  (value) => {
    if (!editor) return;
    const next = value ?? "";
    if (next === editor.state.doc.toString()) return;
    const anchor = Math.min(editor.state.selection.main.head, next.length);
    editor.dispatch({
      changes: { from: 0, to: editor.state.doc.length, insert: next },
      selection: { anchor },
      annotations: [skipStyle.of(true), Transaction.addToHistory.of(false)],
    });
  },
);

watch(
  () => [props.readonly, props.disabled] as const,
  () => {
    editor?.dispatch({ effects: editableComp.reconfigure(lockExtension()) });
  },
);

watch(
  () => props.placeholder,
  (value) => {
    editor?.dispatch({
      effects: placeholderComp.reconfigure(value ? cmPlaceholder(value) : []),
    });
  },
);

function togglePlaceholderList() {
  const view = editor;
  if (!view || locked()) return;
  view.focus();
  if (completionStatus(view.state) === "active") closeCompletion(view);
  else startCompletion(view);
}

function onInvalid() {
  editor?.focus();
}
</script>

<template>
  <div class="ph-wrap" :class="{ framed: !readonly || count != null, 'menu-open': menuOpen }">
    <div class="ph-field" :class="{ multiline, 'is-preview': readonly, 'is-disabled': disabled }">
      <div ref="hostRef" class="ph-host" />
      <input
        v-if="required"
        class="ph-validity"
        type="text"
        :value="modelValue ?? ''"
        required
        tabindex="-1"
        autocomplete="off"
        aria-hidden="true"
        @invalid="onInvalid"
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
          :aria-expanded="menuOpen"
          :class="{ on: menuOpen }"
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
        <div :id="hintId" popover class="emoji-hint" :style="{ positionAnchor: `--${hintId}` }">
          <p>This page can’t open the emoji panel. Focus this field and use the shortcut.</p>
          <ul>
            <li v-for="item in emojiShortcuts" :key="item.id" :class="{ here: os === item.id }">
              <span>{{ item.name }}</span>
              <kbd>{{ item.keys }}</kbd>
            </li>
          </ul>
        </div>
      </div>
      <span v-if="count != null" class="ph-count" :class="{ over: max != null && count > max }">
        {{ count }}<template v-if="max != null"> / {{ max }}</template>
      </span>
    </div>
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
.ph-field {
  position: relative;
  width: 100%;
  min-width: 0;
}
.ph-field.multiline {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
}
.ph-host {
  min-width: 0;
  width: 100%;
}
.ph-field.multiline .ph-host {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 12rem;
}
.ph-field :deep(.ph-input) {
  border: none;
  border-radius: 0;
  background: transparent;
}
.ph-field.multiline :deep(.ph-input) {
  flex: 1 1 auto;
  min-height: 12rem;
  height: 100%;
  overflow: hidden;
  resize: vertical;
}
.ph-field.multiline :deep(.cm-scroller) {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
  overflow: auto;
}
.ph-field.is-preview :deep(.cm-cursor) {
  display: none !important;
}
.ph-validity {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  border: 0;
  opacity: 0;
  pointer-events: none;
  resize: none;
}
</style>

<style>
.cm-tooltip.ph-completion {
  z-index: 80;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-elevated, #1a222c);
  color: var(--text);
  box-shadow: 0 8px 24px rgb(0 0 0 / 35%);
  overflow: hidden;
}
.cm-tooltip.ph-completion > ul {
  font-family: inherit;
  max-height: 12rem !important;
}
.cm-tooltip.ph-completion > ul > li {
  padding: 0.3rem 0.45rem;
  line-height: 1.35;
}
.cm-tooltip.ph-completion > ul > li[aria-selected] {
  background: color-mix(in srgb, var(--accent, #3b82f6) 24%, transparent);
  color: var(--text);
}
.cm-tooltip.ph-completion .cm-completionLabel {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.85rem;
}
.cm-tooltip.ph-completion .cm-completionDetail {
  margin-left: 0.6rem;
  color: var(--muted, #8b9aab);
  font-style: normal;
  font-size: 0.8rem;
}
</style>
