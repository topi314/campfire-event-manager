<script setup lang="ts">
import { coverImageSrc } from "~/utils/covers";

defineProps<{
  name: string;
  description?: string;
  detail?: string;
  to: string;
  previews?: string[];
  badge?: string;
}>();

const { apiBase } = useRuntimeConfig().public;

function src(url: string) {
  return coverImageSrc(url, apiBase as string);
}
</script>

<template>
  <article class="lib-card">
    <NuxtLink class="lib-link" :to="to">
      <div class="thumbs" aria-hidden="true">
        <img v-for="url in (previews || []).slice(0, 5)" :key="url" :src="src(url)" alt="" />
        <span v-if="!(previews || []).length" class="empty">No icons yet</span>
      </div>
      <div class="lib-card-copy">
        <div class="name-row">
          <strong>{{ name }}</strong>
          <span v-if="badge" class="badge">{{ badge }}</span>
        </div>
        <span v-if="description" class="desc">{{ description }}</span>
        <span v-if="detail" class="detail">{{ detail }}</span>
      </div>
    </NuxtLink>
    <div v-if="$slots.action" class="actions">
      <slot name="action" />
    </div>
  </article>
</template>

<style scoped>
.lib-card {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 0.7rem;
  min-height: 8.5rem;
}
.lib-card:hover {
  border-color: var(--accent, #7aa2ff);
}
.lib-link {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-width: 0;
  flex: 1;
  text-decoration: none;
  color: inherit;
}
.thumbs {
  display: flex;
  gap: 0.35rem;
  min-width: 0;
  min-height: 2.75rem;
  align-items: center;
}
.thumbs img {
  flex: 1 1 0;
  width: 0;
  max-width: 2.75rem;
  aspect-ratio: 1;
  height: auto;
  object-fit: cover;
  border-radius: 4px;
  background: var(--bg-elevated);
}
.empty {
  color: var(--muted);
  font-size: 0.8rem;
}
.lib-card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}
.lib-card-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}
.name-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
}
.name-row strong {
  min-width: 0;
}
.lib-link strong,
.detail {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.detail {
  color: var(--muted);
  font-size: 0.75rem;
}
.desc {
  color: var(--muted);
  font-size: 0.8rem;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  white-space: normal;
}
.actions {
  display: flex;
  flex-wrap: nowrap;
  justify-content: flex-end;
  gap: 0.15rem;
}
.actions :deep(.icon-btn) {
  width: 2rem;
  height: 2rem;
  flex: none;
}
.badge {
  flex: none;
  font-size: 0.75rem;
  color: var(--muted);
}
</style>
