<script setup lang="ts">
import CreateMeetupFlow from "~/components/CreateMeetupFlow.vue";

const route = useRoute();

// Legacy: /create?draft=123 → /create/drafts/123
const legacy = route.query.draft;
if (typeof legacy === "string" && /^\d+$/.test(legacy)) {
  await navigateTo(`/create/drafts/${legacy}`, { redirectCode: 301, replace: true });
}
</script>

<template>
  <CreateMeetupFlow />
</template>
