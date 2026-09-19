<script setup lang="ts">
import type { MeetupTemplate, TemplatePublisher } from "~/types";

const props = withDefaults(
  defineProps<{
    template: MeetupTemplate;
    /** When set, likers matching this id are marked as the creator. */
    creatorId?: string | null;
    busy?: boolean;
    compact?: boolean;
    /** Show avatar strip of people who liked. */
    showLikers?: boolean;
  }>(),
  {
    creatorId: null,
    busy: false,
    compact: false,
    showLikers: true,
  },
);

const emit = defineEmits<{
  toggle: [];
}>();

const likeCount = computed(() => Math.max(0, Number(props.template.likeCount) || 0));
const likedByMe = computed(() => !!props.template.likedByMe);
const likers = computed(() => props.template.likers || []);

const creatorLiked = computed(() => {
  const id = props.creatorId || props.template.publisher?.id || props.template.discordUserId;
  if (!id) return false;
  return likers.value.some((l) => l.id === id);
});

function labelOf(u: TemplatePublisher) {
  return u.displayName || u.username || "User";
}

function isCreator(u: TemplatePublisher) {
  const id = props.creatorId || props.template.publisher?.id || props.template.discordUserId;
  return !!id && u.id === id;
}
</script>

<template>
  <div class="template-likes" :class="{ compact }">
    <button
      type="button"
      class="like-btn"
      :class="{ liked: likedByMe }"
      :disabled="busy"
      :aria-pressed="likedByMe"
      :title="likedByMe ? 'Unlike' : 'Like'"
      @click.stop="emit('toggle')"
    >
      <span class="heart" aria-hidden="true">{{ likedByMe ? "♥" : "♡" }}</span>
      <span class="count">{{ likeCount }}</span>
      <span class="sr-only">{{ likedByMe ? "Unlike" : "Like" }}, {{ likeCount }} likes</span>
    </button>

    <div
      v-if="showLikers && likers.length"
      class="likers"
      :title="likers.map(labelOf).join(', ')"
    >
      <NuxtLink
        v-for="u in likers.slice(0, compact ? 4 : 8)"
        :key="u.id"
        :to="`/profiles/${u.id}`"
        class="liker"
        :class="{ creator: isCreator(u) }"
        :title="isCreator(u) ? `${labelOf(u)} (creator)` : labelOf(u)"
        @click.stop
      >
        <img v-if="u.avatarUrl" :src="u.avatarUrl" alt="" class="avatar" />
        <span v-else class="avatar avatar-fallback">{{ labelOf(u).slice(0, 1).toUpperCase() }}</span>
      </NuxtLink>
      <span v-if="likers.length > (compact ? 4 : 8)" class="more muted">
        +{{ likers.length - (compact ? 4 : 8) }}
      </span>
    </div>
    <p v-if="showLikers && !compact && creatorLiked" class="creator-note muted">
      Creator liked this template
    </p>
  </div>
</template>

<style scoped>
.template-likes {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem 0.65rem;
}
.template-likes.compact {
  gap: 0.35rem 0.5rem;
}
.like-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 2rem;
  padding: 0.2rem 0.55rem;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--bg-input);
  color: var(--text);
  font: inherit;
  font-size: 0.85rem;
  cursor: pointer;
}
.like-btn:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--danger) 45%, var(--border));
}
.like-btn.liked {
  border-color: color-mix(in srgb, var(--danger) 55%, var(--border));
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 12%, var(--bg-input));
}
.heart {
  font-size: 0.95rem;
  line-height: 1;
}
.count {
  font-variant-numeric: tabular-nums;
  min-width: 0.75rem;
}
.likers {
  display: inline-flex;
  align-items: center;
}
.liker {
  display: inline-flex;
  margin-left: -0.35rem;
  border-radius: 50%;
  border: 2px solid var(--bg-elevated);
  line-height: 0;
}
.liker:first-child {
  margin-left: 0;
}
.liker.creator {
  box-shadow: 0 0 0 1px var(--accent);
}
.avatar {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  object-fit: cover;
  display: block;
  background: var(--bg-input);
}
.avatar-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.65rem;
  font-weight: 600;
  color: var(--muted);
}
.more {
  margin-left: 0.25rem;
  font-size: 0.8rem;
}
.creator-note {
  flex-basis: 100%;
  margin: 0;
  font-size: 0.8rem;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
</style>
