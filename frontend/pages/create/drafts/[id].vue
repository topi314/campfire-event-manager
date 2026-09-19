<script setup lang="ts">
import CreateMeetupFlow from "~/components/CreateMeetupFlow.vue";

const route = useRoute();
const id = computed(() => {
  const raw = Number(route.params.id);
  return Number.isFinite(raw) && raw > 0 ? raw : null;
});

watch(
  id,
  async (value) => {
    if (value == null) await navigateTo("/create/drafts");
  },
  { immediate: true },
);
</script>

<template>
  <CreateMeetupFlow v-if="id != null" :draft-id="id" />
</template>
