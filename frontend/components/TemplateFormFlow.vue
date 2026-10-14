<script setup lang="ts">
import type { MeetupPayload, MeetupTemplate } from "~/types";

const props = defineProps<{
  /** Omit or null to create a new template. */
  templateId?: number | null;
}>();

const { user, loaded, ensureAuth } = useAuth();
const { api } = useApi();
const toast = useToast();

const editing = ref<MeetupTemplate | null>(null);
const error = ref("");
const saving = ref(false);
const unlinking = ref(false);
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

async function onSave(data: {
  name: string;
  payload: MeetupPayload;
  language: string;
  overriddenFields: string[];
}) {
  saving.value = true;
  error.value = "";
  try {
    if (isEdit.value && editing.value) {
      await api(`/api/templates/${editing.value.id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      });
      toast.success("Template updated", data.name || undefined);
    } else {
      await api("/api/templates", {
        method: "POST",
        body: JSON.stringify({
          name: data.name,
          payload: data.payload,
          language: data.language,
        }),
      });
      toast.success("Template saved", data.name || undefined);
    }
    await navigateTo("/templates");
  } catch (e: any) {
    error.value = e.message || "Failed to save template";
  } finally {
    saving.value = false;
  }
}

async function unlinkFromOriginal() {
  if (!editing.value?.origin || unlinking.value) return;
  const ok = window.confirm(
    "Unlink from original? This copy will keep its current values and no longer follow updates.",
  );
  if (!ok) return;
  unlinking.value = true;
  error.value = "";
  try {
    const t = await api<MeetupTemplate>(`/api/templates/${editing.value.id}/unlink`, {
      method: "POST",
    });
    editing.value = t;
    toast.success("Unlinked from original", t.name || undefined);
  } catch (e: any) {
    error.value = e.message || "Failed to unlink template";
  } finally {
    unlinking.value = false;
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
          — fields follow the original until you change them.
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
          — customized fields stay local; reset any field to follow the original again.
        </template>
        <button
          type="button"
          class="unlink-btn"
          :disabled="unlinking || saving"
          @click="unlinkFromOriginal"
        >
          {{ unlinking ? "Unlinking…" : "Unlink from original" }}
        </button>
      </p>
      <p v-if="error" class="error">{{ error }}</p>
      <TemplateEditor
        v-if="ready"
        :template-name="editing?.name || ''"
        :template-language="editing?.language || null"
        :model-value="editing ? parsePayload(editing) : null"
        :origin="editing?.origin || null"
        :overridden-fields="editing?.overriddenFields || []"
        :saving="saving"
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
.unlink-btn {
  display: inline-block;
  margin-left: 0.65rem;
  border: 0;
  background: none;
  padding: 0;
  color: var(--accent, #3d7eff);
  cursor: pointer;
  font: inherit;
  font-size: inherit;
  text-decoration: underline;
}
.unlink-btn:disabled {
  opacity: 0.6;
  cursor: default;
}
.template-flow {
  max-width: none;
}
</style>
