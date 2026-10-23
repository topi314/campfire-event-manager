<script setup lang="ts">
import CreateMeetupFlow from "~/components/CreateMeetupFlow.vue";

const route = useRoute();

// Legacy: /create?draft=123 → /create/drafts/123
const legacy = route.query.draft;
if (typeof legacy === "string" && /^\d+$/.test(legacy)) {
  await navigateTo(`/create/drafts/${legacy}`, { redirectCode: 301, replace: true });
}

// Legacy: /create?mode=edit&clubId=&eventId= → /edit
if (route.query.mode === "edit") {
  const query: Record<string, string> = {};
  if (typeof route.query.clubId === "string" && route.query.clubId.trim()) {
    query.clubId = route.query.clubId.trim();
  }
  if (typeof route.query.eventId === "string" && route.query.eventId.trim()) {
    query.eventId = route.query.eventId.trim();
  }
  await navigateTo({ path: "/edit", query }, { replace: true });
}
</script>

<template>
  <CreateMeetupFlow />
</template>
