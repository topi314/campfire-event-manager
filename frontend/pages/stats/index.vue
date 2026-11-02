<script setup lang="ts">
const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();

type Stats = {
  meetupsCreated: number;
  meetupsEdited: number;
  meetupsDeleted: number;
  templatesCreated: number;
  templatesEdited: number;
  templatesDeleted: number;
  templates: number;
  templatesPublished: number;
  drafts: number;
  draftsCreated: number;
  draftsEdited: number;
  draftsDeleted: number;
  draftsPublished: number;
};

const stats = ref<Stats | null>(null);
const error = ref("");

const groups = computed(() => {
  const s = stats.value;
  if (!s) return [];
  return [
    {
      title: "Events",
      items: [
        { label: "Created", value: s.meetupsCreated },
        { label: "Edited", value: s.meetupsEdited },
        { label: "Deleted", value: s.meetupsDeleted },
      ],
    },
    {
      title: "Drafts",
      items: [
        { label: "Drafts", value: s.drafts },
        { label: "Published", value: s.draftsPublished },
        { label: "Created", value: s.draftsCreated },
        { label: "Edited", value: s.draftsEdited },
        { label: "Deleted", value: s.draftsDeleted },
      ],
    },
    {
      title: "Templates",
      items: [
        { label: "Templates", value: s.templates },
        { label: "Published", value: s.templatesPublished },
        { label: "Created", value: s.templatesCreated },
        { label: "Edits", value: s.templatesEdited },
        { label: "Deletes", value: s.templatesDeleted },
      ],
    },
  ];
});

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  try {
    stats.value = await api<Stats>("/api/stats");
  } catch (e: any) {
    error.value = e.message || "Failed to load stats";
  }
});

function formatCount(n: number) {
  return n.toLocaleString();
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel">
      <div class="page-head">
        <h2>Stats</h2>
      </div>
      <p v-if="error" class="error">{{ error }}</p>
      <div v-else-if="stats" class="stats-groups">
        <section v-for="group in groups" :key="group.title" class="stats-group">
          <h3>{{ group.title }}</h3>
          <ul class="stats-grid" :style="{ '--cols': group.items.length }">
            <li v-for="item in group.items" :key="item.label">
              <span class="stats-label">{{ item.label }}</span>
              <span class="stats-value">{{ formatCount(item.value) }}</span>
            </li>
          </ul>
        </section>
      </div>
      <p v-else class="muted">Loading…</p>
    </div>
  </AppShell>
</template>

<style scoped>
.page-head h2 {
  margin: 0 0 1rem;
  font-size: 1.15rem;
}
.stats-group + .stats-group {
  margin-top: 1.35rem;
}
.stats-group h3 {
  margin: 0 0 0.7rem;
  font-size: 0.95rem;
}
.stats-grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
  gap: 0.65rem;
}
.stats-grid li {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
  padding: 0.7rem 0.85rem 0.75rem;
}
.stats-label {
  display: block;
  color: var(--muted);
  font-size: 0.8rem;
}
.stats-value {
  display: block;
  margin-top: 0.2rem;
  font-size: 1.55rem;
  font-weight: 600;
  line-height: 1.15;
  font-variant-numeric: tabular-nums;
}
@media (max-width: 720px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
