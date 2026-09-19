<script setup lang="ts">
const { user, loaded, ensureAuth } = useAuth();

onMounted(async () => {
  if (await ensureAuth()) {
    await navigateTo("/create");
  }
});
</script>

<template>
  <div class="login-page">
    <div class="panel login-card">
      <h1>Campfire Event Manager</h1>
      <p class="muted">
        Sign in with Discord. Access is limited to Community Ambassadors (configured guild or
        whitelist).
      </p>
      <p v-if="loaded && user" class="success">Already logged in. Redirecting…</p>
      <p v-else>
        <a class="btn primary" href="/auth/login?rd=/create">Login with Discord</a>
      </p>
    </div>
  </div>
</template>
