<script setup lang="ts">
import TemplateFormFlow from "~/components/TemplateFormFlow.vue";

const route = useRoute();
const id = computed(() => {
  const raw = Number(route.params.id);
  return Number.isFinite(raw) && raw > 0 ? raw : null;
});

watch(
  id,
  async (value) => {
    if (value == null) await navigateTo("/templates");
  },
  { immediate: true },
);
</script>

<template>
  <TemplateFormFlow v-if="id != null" :template-id="id" />
</template>
