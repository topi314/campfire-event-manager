<script setup lang="ts">
import { BUILTIN_PLACEHOLDERS } from "~/utils/placeholders";

withDefaults(
  defineProps<{
    /** Compact list for tighter layouts (create flow). */
    compact?: boolean;
  }>(),
  { compact: false },
);

const open = ref(false);

const coreBuiltins = computed(() =>
  BUILTIN_PLACEHOLDERS.filter((b) => b.group !== "eventPokemon"),
);
const eventPokemonBuiltins = computed(() =>
  BUILTIN_PLACEHOLDERS.filter((b) => b.group === "eventPokemon"),
);

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && open.value) {
    open.value = false;
  }
}

watch(open, (isOpen) => {
  if (!import.meta.client) return;
  document.body.style.overflow = isOpen ? "hidden" : "";
});

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});
onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
  if (import.meta.client) document.body.style.overflow = "";
});
</script>

<template>
  <div class="placeholder-help">
    <p class="lede muted">
      Use <code v-pre>{{key}}</code> tokens in the meetup title and description.
      Letter case does not matter. Built-ins fill on the server when you create or post
      (and in Preview). Any other key becomes a required create-time field unless you set a
      default.
    </p>

    <button type="button" class="open-btn" @click="open = true">
      View placeholder help
      <span class="count muted">({{ BUILTIN_PLACEHOLDERS.length }} built-ins)</span>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        class="modal-backdrop"
        @click.self="open = false"
      >
        <div
          class="modal modal-placeholders"
          role="dialog"
          aria-modal="true"
          aria-labelledby="builtin-placeholders-title"
        >
          <div class="popup-head">
            <div>
              <h2 id="builtin-placeholders-title">Placeholders</h2>
              <p class="muted sub">
                Tokens look like <code v-pre>{{liveEvent}}</code>. Indexes use
                <code v-pre>{{eventPokemon[1]}}</code> (1-based).
              </p>
            </div>
            <button type="button" class="icon-close" aria-label="Close" @click="open = false">
              ×
            </button>
          </div>

          <div class="scroll">
            <h3>How tokens work</h3>
            <p class="muted section-p">
              Put <code v-pre>{{key}}</code> in title, description, address, or cover URL.
              Matching is case-insensitive. Unknown keys become fields on the create page —
              required unless the template defines a default. Custom values override builtins
              when the same key is sent.
            </p>

            <h3>Built-ins (auto)</h3>
            <ul class="builtin-list" :class="{ compact }">
              <li v-for="b in coreBuiltins" :key="b.key">
                <div class="keys">
                  <code v-text="'{{' + b.key + '}}'" />
                </div>
                <div class="meta">
                  <strong>{{ b.label }}</strong>
                  <span class="muted">{{ b.description }}</span>
                  <span v-if="!compact" class="example muted">e.g. {{ b.example }}</span>
                </div>
              </li>
            </ul>

            <h3>Event Pokémon</h3>
            <p class="muted section-p">
              Extracted from the live event title for categories like Community Day, Raid Hour,
              Spotlight Hour, Research/Hatch Day, Max Monday/Battle Day (not GO Fest/Tour/etc.).
              Form prefixes (Mega, Shadow, Alolan, Galarian, …) are matched when present.
              <code v-pre>{{eventPokemon}}</code> is translated to the template language and is
              editable on create when used. Ceiling/Floor IV CP/WP keys are computed from that
              species and are <strong>not</strong> editable. Units: EN <code>CP</code>, DE
              <code>WP</code> (Wettkampfpunkte), FR/ES/IT <code>PC</code>.
            </p>
            <ul class="builtin-list" :class="{ compact }">
              <li v-for="b in eventPokemonBuiltins" :key="b.key">
                <div class="keys">
                  <code v-text="'{{' + b.key + '}}'" />
                </div>
                <div class="meta">
                  <strong>{{ b.label }}</strong>
                  <span class="muted">{{ b.description }}</span>
                  <span v-if="!compact" class="example muted">e.g. {{ b.example }}</span>
                </div>
              </li>
            </ul>
            <p class="muted section-p">
              Indexes: <code v-pre>{{eventPokemon[1]}}</code>,
              <code v-pre>{{eventPokemonCeilingRaid[2]}}</code> — first/second species only.
              Unindexed forms are the full comma-joined list. Out of range → empty.
            </p>

            <h3>Custom placeholders</h3>
            <p class="muted section-p">
              Any other key (e.g. <code v-pre>{{city}}</code>, <code v-pre>{{park}}</code>,
              <code v-pre>{{meetupSpot}}</code>) becomes a create-time field. Optionally set a
              label and default in the template editor. Fields without a default are
              <strong>required</strong> when posting a meetup (drafts may stay incomplete).
            </p>
          </div>

          <div class="modal-actions">
            <button type="button" class="primary" @click="open = false">Close</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.placeholder-help {
  margin: 0.35rem 0 0;
}
.lede {
  margin: 0 0 0.5rem;
  font-size: 0.85rem;
  line-height: 1.45;
}
.lede code,
.section-p code,
.keys code,
.sub code {
  font-size: 0.8em;
}
.open-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
  padding: 0.2rem 0;
  border: none;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 0.15em;
}
.open-btn:hover {
  color: var(--accent, var(--text));
}
.count {
  font-weight: 500;
  text-decoration: none;
}
.modal-placeholders {
  width: min(640px, 100%);
  max-height: min(85vh, 780px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.popup-head {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
  margin-bottom: 0.85rem;
}
.popup-head h2 {
  margin: 0;
}
.sub {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
.icon-close {
  flex: 0 0 auto;
  width: 2rem;
  height: 2rem;
  padding: 0;
  border-radius: 8px;
  font-size: 1.35rem;
  line-height: 1;
}
.scroll {
  overflow: auto;
  min-height: 0;
  flex: 1;
  padding-right: 0.15rem;
}
.scroll h3 {
  margin: 1rem 0 0.4rem;
  font-size: 0.95rem;
}
.scroll h3:first-child {
  margin-top: 0;
}
.section-p {
  margin: 0 0 0.55rem;
  font-size: 0.85rem;
  line-height: 1.45;
}
.builtin-list {
  list-style: none;
  margin: 0 0 0.5rem;
  padding: 0;
  display: grid;
  gap: 0.55rem;
}
.builtin-list li {
  display: grid;
  gap: 0.2rem 0.75rem;
  grid-template-columns: minmax(7.5rem, max-content) 1fr;
  align-items: start;
  padding: 0.45rem 0.55rem;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: color-mix(in srgb, var(--panel) 88%, transparent);
}
.keys {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  padding-top: 0.1rem;
}
.keys code {
  display: inline-block;
  padding: 0.05rem 0.3rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--text) 6%, transparent);
}
.meta {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  font-size: 0.85rem;
  line-height: 1.35;
}
.meta strong {
  font-weight: 600;
}
.example {
  font-size: 0.8rem;
}
.modal-actions {
  margin-top: 1rem;
  display: flex;
  justify-content: flex-end;
}
.compact li {
  grid-template-columns: 1fr;
  padding: 0.4rem 0.5rem;
}
@media (max-width: 640px) {
  .builtin-list li {
    grid-template-columns: 1fr;
  }
}
</style>
