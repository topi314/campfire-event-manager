<script setup lang="ts">
import {
  TEMPLATE_LANGUAGES,
  guessTemplateLanguage,
  isTemplateLanguageCode,
  type TemplateLanguageCode,
} from "~/utils/languages";

const props = defineProps<{
  open: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();

const { setPreferredLanguage, preferredLanguage } = usePreferences();
const { openSettings } = useSessionToken();

const draftLanguage = ref<TemplateLanguageCode>(guessTemplateLanguage());

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    draftLanguage.value = isTemplateLanguageCode(preferredLanguage.value)
      ? preferredLanguage.value
      : guessTemplateLanguage();
  },
  { immediate: true },
);

function confirm() {
  setPreferredLanguage(
    isTemplateLanguageCode(draftLanguage.value)
      ? draftLanguage.value
      : guessTemplateLanguage(),
  );
  emit("close");
}

function confirmAndOpenSettings() {
  setPreferredLanguage(
    isTemplateLanguageCode(draftLanguage.value)
      ? draftLanguage.value
      : guessTemplateLanguage(),
  );
  emit("close");
  openSettings();
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.open) confirm();
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    aria-labelledby="lang-prompt-title"
    @click.self="confirm"
  >
    <div class="modal">
      <h2 id="lang-prompt-title">Choose your language</h2>
      <p class="muted">
        Used for Pokémon name translation in meetup placeholders. You can change it later in
        Settings.
      </p>
      <div class="field">
        <label for="lang-prompt-select">Language</label>
        <select id="lang-prompt-select" v-model="draftLanguage">
          <option v-for="l in TEMPLATE_LANGUAGES" :key="l.code" :value="l.code">
            {{ l.label }}
          </option>
        </select>
      </div>
      <div class="modal-actions">
        <button type="button" @click="confirmAndOpenSettings">Settings…</button>
        <button type="button" class="primary" @click="confirm">Continue</button>
      </div>
    </div>
  </div>
</template>
