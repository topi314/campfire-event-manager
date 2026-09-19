<script setup lang="ts">
import TemplatesBrowseFlow from "~/components/TemplatesBrowseFlow.vue";

const route = useRoute();

// Legacy: /templates/browse?highlight=123 → /templates/browse/123
const legacy = route.query.highlight;
if (typeof legacy === "string" && /^\d+$/.test(legacy)) {
  await navigateTo(`/templates/browse/${legacy}`, { redirectCode: 301, replace: true });
}
</script>

<template>
  <TemplatesBrowseFlow />
</template>
