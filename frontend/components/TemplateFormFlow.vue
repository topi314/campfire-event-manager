<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate } from "~/types";

const props = defineProps<{
  /** Omit or null to create a new template. */
  templateId?: number | null;
}>();

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const { authHeaders } = useSessionToken();

const editing = ref<MeetupTemplate | null>(null);
const error = ref("");
const saving = ref(false);
const ready = ref(false);

const isEdit = computed(() => props.templateId != null && props.templateId > 0);

onMounted(async () => {
  if (!(await ensureAuth())) {
    await navigateTo("/login");
    return;
  }
  if (isEdit.value) {
    await loadTemplate(props.templateId!);
  } else {
    ready.value = true;
  }
});

watch(
  () => props.templateId,
  async (id) => {
    if (id == null || id <= 0) {
      editing.value = null;
      ready.value = true;
      return;
    }
    await loadTemplate(id);
  },
);

function parsePayload(t: MeetupTemplate): MeetupPayload {
  return (typeof t.payload === "string" ? JSON.parse(t.payload) : t.payload) as MeetupPayload;
}

async function loadTemplate(id: number) {
  error.value = "";
  ready.value = false;
  try {
    const t = await api<MeetupTemplate>(`/api/templates/${id}`);
    editing.value = t;
    ready.value = true;
  } catch (e: any) {
    error.value = e.message || "Template not found";
    await navigateTo("/templates");
  }
}

async function onSave(data: { name: string; payload: MeetupPayload }) {
  saving.value = true;
  error.value = "";
  try {
    if (isEdit.value && editing.value) {
      await api(`/api/templates/${editing.value.id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      });
    } else {
      await api("/api/templates", {
        method: "POST",
        body: JSON.stringify(data),
      });
    }
    await navigateTo("/templates");
  } catch (e: any) {
    error.value = e.message || "Failed to save template";
  } finally {
    saving.value = false;
  }
}

function cancelEditor() {
  navigateTo("/templates");
}
</script>

<template>
  <AppShell>
    <div v-if="loaded && user" class="panel template-flow">
      <div class="page-head">
        <h2>{{ isEdit ? "Edit template" : "New template" }}</h2>
      </div>
      <p v-if="editing?.origin" class="origin muted">
        <template v-if="editing.synced">
          Synced with
          <strong>{{ editing.origin.name }}</strong>
          <template v-if="editing.origin.publisher">
            by
            <NuxtLink :to="`/profiles/${editing.origin.publisher.id}`">
              {{ editing.origin.publisher.displayName || editing.origin.publisher.username }}
            </NuxtLink>
          </template>
          — saving your edits will unsync this copy.
        </template>
        <template v-else>
          Based on
          <strong>{{ editing.origin.name }}</strong>
          <template v-if="editing.origin.publisher">
            by
            <NuxtLink :to="`/profiles/${editing.origin.publisher.id}`">
              {{ editing.origin.publisher.displayName || editing.origin.publisher.username }}
            </NuxtLink>
          </template>
          — edits stay on your unsynced copy.
        </template>
      </p>
      <p v-if="error" class="error">{{ error }}</p>
      <TemplateEditor
        v-if="ready"
        :template-name="editing?.name || ''"
        :model-value="editing ? parsePayload(editing) : null"
        :saving="saving"
        :auth-headers="authHeaders()"
        :submit-label="isEdit ? 'Update template' : 'Create template'"
        @save="onSave"
        @cancel="cancelEditor"
      />
    </div>
  </AppShell>
</template>

<style scoped>
.page-head {
  margin-bottom: 0.35rem;
}
.page-head h2 {
  margin: 0;
  font-size: 1.1rem;
}
.origin {
  margin: 0 0 0.85rem;
}
.template-flow {
  max-width: none;
}
</style>
