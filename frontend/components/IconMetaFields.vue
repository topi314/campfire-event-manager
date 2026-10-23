<script setup lang="ts">
const name = defineModel<string>("name", { default: "" });
const category = defineModel<string>("category", { default: "" });
const pokemon = defineModel<string[]>("pokemon", { default: () => [] });

const props = withDefaults(
  defineProps<{
    categories: string[];
    showName?: boolean;
    compact?: boolean;
  }>(),
  { showName: true, compact: false },
);

const { nameOf } = usePokemonList();

function add(slug: string) {
  if (!slug || pokemon.value.includes(slug)) return;
  pokemon.value = [...pokemon.value, slug];
}

function remove(slug: string) {
  pokemon.value = pokemon.value.filter((s) => s !== slug);
}
</script>

<template>
  <div class="meta" :class="{ compact: props.compact }">
    <div v-if="props.showName" class="field">
      <label>Name</label>
      <input v-model="name" type="text" maxlength="80" placeholder="Icon name" />
    </div>
    <div class="field">
      <label>Category</label>
      <select v-model="category">
        <option value="">No category</option>
        <option v-for="opt in categories" :key="opt" :value="opt">{{ opt }}</option>
      </select>
    </div>
    <div class="field poke-field">
      <label>Pokémon</label>
      <PokemonSelect
        placeholder="Add Pokémon"
        add
        :exclude="pokemon"
        @select="add"
      />
      <p v-if="pokemon.length" class="picks">
        <button
          v-for="slug in pokemon"
          :key="slug"
          type="button"
          class="chip"
          @click="remove(slug)"
        >
          {{ nameOf(slug) }} ×
        </button>
      </p>
    </div>
  </div>
</template>

<style scoped>
.meta {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.meta.compact {
  display: grid;
  grid-template-columns: 1.3fr 1fr;
  gap: 0.45rem 0.65rem;
  align-items: start;
}
.meta.compact .poke-field {
  grid-column: 1 / -1;
}
.field {
  margin-bottom: 0;
}
.picks {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin: 0.35rem 0 0;
}
.chip {
  padding: 0.15rem 0.5rem;
  font-size: 0.78rem;
  line-height: 1.3;
  border-radius: 999px;
}
</style>
