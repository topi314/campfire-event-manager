<script setup lang="ts">
import TemplatesBrowseFlow from "~/components/TemplatesBrowseFlow.vue";

const route = useRoute();
const id = computed(() => {
  const raw = Number(route.params.id);
  return Number.isFinite(raw) && raw > 0 ? raw : null;
});

watch(
  id,
  async (value) => {
    if (value == null) {
      await navigateTo({ path: "/templates/browse", query: route.query });
    }
  },
  { immediate: true },
);
</script>

<template>
  <TemplatesBrowseFlow v-if="id != null" :highlight-id="id" />
</template>
