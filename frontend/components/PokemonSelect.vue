<script setup lang="ts">
import { fuzzyRank } from "~/utils/fuzzy";

const model = defineModel<string>({ default: "" });

const props = withDefaults(
  defineProps<{
    id?: string;
    placeholder?: string;
    /** Slugs already chosen, hidden from the list. */
    exclude?: string[];
    /** Selecting adds a value and leaves the field empty. */
    add?: boolean;
  }>(),
  { placeholder: "Any", exclude: () => [], add: false },
);

const emit = defineEmits<{ select: [slug: string] }>();

const { species, ensure, nameOf } = usePokemonList();
const query = ref("");
const open = ref(false);
const active = ref(0);
const inputEl = ref<HTMLInputElement | null>(null);
const box = ref({ top: 0, left: 0, width: 0 });

function place() {
  const el = inputEl.value;
  if (!el) return;
  const rect = el.getBoundingClientRect();
  box.value = { top: rect.bottom + 4, left: rect.left, width: rect.width };
}

onMounted(() => {
  void ensure();
});

const matches = computed(() => {
  const excluded = new Set(props.exclude);
  const pool = species.value.filter((sp) => !excluded.has(sp.slug));
  const q = query.value.trim();
  const ranked = q ? fuzzyRank(q, pool, (sp) => `${sp.name} ${sp.slug}`) : pool;
  return ranked.slice(0, 40);
});

const shownValue = computed(() => {
  if (open.value) return query.value;
  return model.value ? nameOf(model.value) : "";
});

function onFocus() {
  place();
  open.value = true;
  query.value = "";
  active.value = 0;
}

function close() {
  open.value = false;
  query.value = "";
  active.value = 0;
}

function choose(slug: string) {
  emit("select", slug);
  model.value = props.add ? "" : slug;
  close();
  if (props.add) inputEl.value?.blur();
}

function onInput(event: Event) {
  query.value = (event.target as HTMLInputElement).value;
  place();
  open.value = true;
  active.value = 0;
  if (!props.add && !query.value.trim()) model.value = "";
}

function onKeydown(event: KeyboardEvent) {
  if (!open.value && (event.key === "ArrowDown" || event.key === "Enter")) {
    open.value = true;
    event.preventDefault();
    return;
  }
  if (event.key === "ArrowDown") {
    active.value = Math.min(active.value + 1, Math.max(matches.value.length - 1, 0));
    event.preventDefault();
  } else if (event.key === "ArrowUp") {
    active.value = Math.max(active.value - 1, 0);
    event.preventDefault();
  } else if (event.key === "Enter") {
    const hit = matches.value[active.value];
    if (hit) {
      choose(hit.slug);
      event.preventDefault();
    }
  } else if (event.key === "Escape") {
    close();
    inputEl.value?.blur();
  }
}

function onBlur() {
  window.setTimeout(() => {
    if (document.activeElement === inputEl.value) return;
    close();
  }, 120);
}
</script>

<template>
  <div class="poke-select">
    <input
      :id="id"
      ref="inputEl"
      type="text"
      role="combobox"
      aria-autocomplete="list"
      :aria-expanded="open"
      autocomplete="off"
      :placeholder="placeholder"
      :value="shownValue"
      @focus="onFocus"
      @input="onInput"
      @keydown="onKeydown"
      @blur="onBlur"
    />
    <Teleport to="body">
      <ul
        v-if="open"
        class="menu"
        role="listbox"
        :style="{ top: `${box.top}px`, left: `${box.left}px`, width: `${box.width}px` }"
      >
        <li v-if="!add">
          <button type="button" @mousedown.prevent="choose('')">{{ placeholder }}</button>
        </li>
        <li v-for="(sp, i) in matches" :key="sp.slug">
          <button
            type="button"
            :class="{ active: i === active }"
            @mousedown.prevent="choose(sp.slug)"
            @mouseenter="active = i"
          >
            {{ sp.name }}
          </button>
        </li>
        <li v-if="query.trim() && !matches.length" class="empty">No matches</li>
      </ul>
    </Teleport>
  </div>
</template>

<style scoped>
.poke-select {
  position: relative;
}
.menu {
  position: fixed;
  z-index: 80;
  margin: 0;
  padding: 0.25rem;
  list-style: none;
  max-height: 16rem;
  overflow: auto;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.menu button {
  width: 100%;
  text-align: left;
  border: 0;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
}
.menu button.active,
.menu button:hover {
  background: var(--bg);
}
.empty {
  padding: 0.35rem 0.5rem;
  color: var(--muted);
  font-size: 0.85rem;
}
</style>
