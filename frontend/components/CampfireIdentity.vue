<script setup lang="ts">
import caStarUrl from "~/assets/images/ca-star.png";

const { me, isCA } = useCampfireSession();

const identity = computed(() => {
  const profile = me.value;
  if (!profile) return null;
  const name = (profile.displayName || profile.username || "").trim();
  const handle = (profile.username || "").trim();
  const showHandle = !!handle && handle.toLowerCase() !== name.toLowerCase();
  return {
    name: name || handle,
    handle: showHandle ? handle : "",
    initial: (name || handle || "?").slice(0, 1).toUpperCase(),
    avatarUrl: (profile.avatarUrl || "").trim(),
  };
});
</script>

<template>
  <div
    v-if="identity"
    class="campfire-who"
    :title="
      isCA
        ? `Campfire · ${identity.name} · Community Ambassador`
        : `Campfire · ${identity.name}`
    "
  >
    <span class="user-avatar">
      <img v-if="identity.avatarUrl" :src="identity.avatarUrl" alt="" />
      <span v-else class="user-avatar-fallback" aria-hidden="true">
        {{ identity.initial }}
      </span>
      <img v-if="isCA" class="ca-star" :src="caStarUrl" alt="" />
    </span>
    <span class="campfire-who-text">
      <span class="campfire-who-name">{{ identity.name }}</span>
      <span v-if="identity.handle" class="campfire-who-handle muted">@{{ identity.handle }}</span>
      <span v-else class="campfire-who-handle muted">Campfire</span>
    </span>
  </div>
</template>

<style scoped>
.campfire-who {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}
.campfire-who-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.15;
}
.campfire-who-name {
  font-size: 0.9rem;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.campfire-who-handle {
  font-size: 0.75rem;
}
</style>
