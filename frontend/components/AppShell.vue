<script setup lang="ts">
import caStarUrl from "~/assets/images/ca-star.png";

const { user, loaded, refresh } = useAuth();
const {
  token,
  settingsOpen,
  focusToken,
  openSettings,
  closeSettings,
} = useSessionToken();
const {
  isCA,
  tokenError,
  validateToken,
  clearCampfireProfile,
} = useCampfireSession();
const { timezonePromptOpen, maybePromptTimezone, closeTimezonePrompt } = usePreferences();

onMounted(async () => {
  await refresh();
  if (token.value) await validateToken({ openSettingsOnFail: false });
});

watch(
  user,
  (u) => {
    if (u) maybePromptTimezone();
  },
  { immediate: true },
);

watch(token, async (t, prev) => {
  if (!t) {
    clearCampfireProfile();
    return;
  }
  if (t !== prev) await validateToken({ openSettingsOnFail: false });
});
</script>

<template>
  <div class="layout">
    <header class="topbar">
      <div>
        <h1>Campfire Event Manager</h1>
        <nav class="primary-nav" aria-label="Main">
          <div class="nav-group">
            <NuxtLink to="/create">Create</NuxtLink>
            <NuxtLink to="/create/drafts">Drafts</NuxtLink>
          </div>
          <div class="nav-group">
            <NuxtLink to="/templates">My templates</NuxtLink>
            <NuxtLink to="/templates/browse">Browse templates</NuxtLink>
          </div>
        </nav>
      </div>
      <div class="topbar-actions">
        <button
          type="button"
          class="icon-btn"
          title="Settings"
          aria-label="Settings"
          @click="openSettings()"
        >
          <svg class="icon-gear" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="currentColor"
              d="M19.14 12.94c.04-.31.06-.63.06-.94s-.02-.63-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.07 7.07 0 0 0-1.63-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54c-.6.24-1.13.55-1.63.94l-2.39-.96a.5.5 0 0 0-.6.22L2.71 8.84a.5.5 0 0 0 .12.64l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94L2.83 14.5a.5.5 0 0 0-.12.64l1.92 3.32c.14.24.43.34.68.22l2.39-.96c.5.39 1.04.7 1.63.94l.36 2.54c.05.24.26.42.5.42h3.84c.24 0 .45-.18.5-.42l.36-2.54c.6-.24 1.13-.55 1.63-.94l2.39.96c.25.12.54.02.68-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.6A3.6 3.6 0 1 1 12 8.4a3.6 3.6 0 0 1 0 7.2Z"
            />
          </svg>
        </button>
        <NuxtLink
          v-if="user"
          :to="`/profiles/${user.id}`"
          class="user-chip"
          :title="
            isCA
              ? `${user.displayName || user.username} · Community Ambassador`
              : user.displayName || user.username
          "
        >
          <span class="user-avatar">
            <img v-if="user.avatarUrl" :src="user.avatarUrl" alt="" />
            <span v-else class="user-avatar-fallback" aria-hidden="true">
              {{ (user.displayName || user.username || "?").slice(0, 1).toUpperCase() }}
            </span>
            <img
              v-if="isCA"
              class="ca-star"
              :src="caStarUrl"
              alt=""
              title="Community Ambassador"
            />
          </span>
          <span class="user-chip-name">{{ user.displayName || user.username }}</span>
        </NuxtLink>
      </div>
    </header>

    <p v-if="loaded && !user" class="error">
      Not logged in. <NuxtLink to="/login">Login with Discord</NuxtLink>
    </p>
    <p v-else-if="tokenError" class="error token-banner">
      {{ tokenError }}
      <button
        type="button"
        class="primary"
        style="margin-left: 0.5rem"
        @click="openSettings({ focusToken: true })"
      >
        Open Settings
      </button>
    </p>
    <p v-else-if="!token" class="muted">
      Paste your Campfire session JWT in Settings to create meetups.
      <button
        type="button"
        class="primary"
        style="margin-left: 0.5rem"
        @click="openSettings({ focusToken: true })"
      >
        Open Settings
      </button>
    </p>

    <slot />

    <SettingsModal
      :open="settingsOpen"
      :focus-token="focusToken"
      @close="closeSettings"
    />
    <TimezonePrompt :open="timezonePromptOpen" @close="closeTimezonePrompt" />
  </div>
</template>
