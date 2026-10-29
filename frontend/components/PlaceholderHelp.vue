<script setup lang="ts">
import { layoutPieces } from "~/utils/datetime";
import { POKEMON_NAME_LANGUAGES, templateLanguageLabel } from "~/utils/languages";
import { BUILTIN_PLACEHOLDERS } from "~/utils/placeholders";
import { TIME_FORMATS } from "~/utils/timeFormats";

/** Reference instant used in the layout table: 18 Oct 2026, 15:45. */
const layoutSample = { year: 2026, month: 10, day: 18, hours: 15, minutes: 45 };

const timeLayouts = TIME_FORMATS.map((preset) => {
  const pieces = layoutPieces(preset.id, layoutSample);
  return { id: preset.id, date: pieces.date, time: pieces.time };
});

const pokemonNameLanguages = POKEMON_NAME_LANGUAGES.map((code) => ({
  code,
  label: templateLanguageLabel(code),
}));

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
      <template v-if="compact">
        <code v-pre>{{key}}</code> tokens · filled in Preview / on post
      </template>
      <template v-else>
        Use <code v-pre>{{key}}</code> tokens in the meetup title and description.
        Letter case does not matter. Built-ins fill on the server when you create or post
        (and in Preview). Any other key becomes a required create-time field unless you set a
        default.
      </template>
    </p>

    <button type="button" class="open-btn" @click="open = true">
      Placeholder help
      <span class="count muted">({{ BUILTIN_PLACEHOLDERS.length }})</span>
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
              <svg class="icon-close-mark" viewBox="0 0 16 16" aria-hidden="true">
                <path d="M4 4 L12 12 M12 4 L4 12" />
              </svg>
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
            <h3>Time layouts</h3>
            <p class="muted section-p">
              <code v-pre>{{date}}</code>, <code v-pre>{{startTime}}</code>, and
              <code v-pre>{{endTime}}</code> accept a time-format id after a colon, the same
              ids as Settings. <code v-pre>{{date:id}}</code> is the date half.
              <code v-pre>{{startTime:id}}</code> and <code v-pre>{{endTime:id}}</code> are the
              clock half (same clock for a given id). Examples are for 18 Oct 2026 at 15:45.
              Without an id, <code v-pre>{{date}}</code> is <code>Oct 18, 2026</code>,
              <code v-pre>{{dateShort}}</code> is <code>2026-10-18</code>,
              <code v-pre>{{year}}</code> is <code>2026</code>,
              <code v-pre>{{month}}</code> is <code>10</code>,
              <code v-pre>{{day}}</code> is <code>18</code>, and
              <code v-pre>{{startTime}}</code> / <code v-pre>{{endTime}}</code> are
              <code>15:45</code>. An unknown id falls back to that default.
            </p>
            <table class="help-table">
              <thead>
                <tr>
                  <th scope="col">Id</th>
                  <th scope="col"><code v-pre>{{date:id}}</code></th>
                  <th scope="col"><code v-pre>{{startTime:id}}</code></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in timeLayouts" :key="row.id">
                  <th scope="row"><code>{{ row.id }}</code></th>
                  <td>{{ row.date }}</td>
                  <td>{{ row.time }}</td>
                </tr>
              </tbody>
            </table>
            <p class="muted section-p">
              <code>hm-dmy</code> uses the same pieces as <code>dmy-24</code>, and
              <code>hm-dmy-slash</code> uses the same pieces as <code>dmy-slash-24</code>.
              Settings shows those two with the clock before the date when both appear together.
            </p>

            <h3>Event Pokémon</h3>
            <p class="muted section-p">
              Extracted from the live event title for categories like Community Day, Raid Hour,
              Raid Day, Super Mega Raid Day, Shadow Raid Day, Spotlight Hour, Research Day,
              Catch Mastery, Hatch Day, Max Monday/Battle Day (not GO Fest/Tour/etc.).
              Form prefixes (Mega, Shadow, Alolan, Galarian, …) are matched when present.
              <code v-pre>{{eventPokemon}}</code> is translated to the meetup language
              (defaults from the template; overridable on create) and is editable when used.
              Ceiling/Floor IV keys are computed from that species and are
              <strong>not</strong> editable (bare numbers, no CP/WP unit).
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
              Pin a name with <code v-pre>{{eventPokemon:de}}</code> or
              <code v-pre>{{eventPokemon[2]:fr}}</code>. Unscoped
              <code v-pre>{{eventPokemon}}</code> stays on the meetup language.
              Ceiling and floor keys stay numbers.
            </p>
            <p class="muted section-p">
              These codes each have a Pokémon name list of their own. Any other code
              falls back to English. A regional code such as <code>zh-CN</code> uses
              <code>zh</code>.
            </p>
            <table class="help-table">
              <thead>
                <tr>
                  <th scope="col">Code</th>
                  <th scope="col">Language</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="lang in pokemonNameLanguages" :key="lang.code">
                  <th scope="row"><code>{{ lang.code }}</code></th>
                  <td>{{ lang.label }}</td>
                </tr>
              </tbody>
            </table>

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
  width: min(720px, 100%);
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
.help-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0 0 0.55rem;
  font-size: 0.8rem;
}
.help-table th,
.help-table td {
  text-align: left;
  padding: 0.28rem 0.45rem;
  border-bottom: 1px solid var(--border);
  vertical-align: baseline;
  font-weight: 400;
}
.help-table thead th {
  font-weight: 600;
  font-size: 0.75rem;
}
.help-table code {
  font-size: 0.85em;
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
