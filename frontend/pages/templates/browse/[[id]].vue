<script setup lang="ts">
import TemplatesBrowseFlow from "~/components/TemplatesBrowseFlow.vue";

// /templates/browse and /templates/browse/:id share this page. A stable key
// keeps the list mounted when a template is opened or closed.
definePageMeta({
  key: () => "templates-browse",
  // Opening or closing a listing changes the path. Keep the list where it was.
  scrollToTop: (to, from) => {
    const browse = (path: string) =>
      path === "/templates/browse" || path.startsWith("/templates/browse/");
    return !(browse(from.path) && browse(to.path));
  },
});

const route = useRoute();

const id = computed(() => {
  const raw = route.params.id;
  const value = Array.isArray(raw) ? raw[0] : raw;
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? n : null;
});

watch(
  () => route.params.id,
  async (raw) => {
    const value = Array.isArray(raw) ? raw[0] : raw;
    if (value == null || value === "") return;
    if (id.value == null) {
      await navigateTo({ path: "/templates/browse", query: route.query });
    }
  },
  { immediate: true },
);
</script>

<template>
  <TemplatesBrowseFlow :highlight-id="id" />
</template>
