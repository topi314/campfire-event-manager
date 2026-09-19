<script setup lang="ts">
defineProps<{
  modelValue: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [string];
}>();

const options = [
  { value: "ORGANIZERS_ONLY", label: "Organizers only" },
  { value: "ALL_INVITED", label: "Any attendee" },
  { value: "NO_ONE", label: "No one" },
] as const;
</script>

<template>
  <div class="comments-picker">
    <label v-for="opt in options" :key="opt.value" class="radio">
      <input
        type="radio"
        name="commentsPermissions"
        :checked="modelValue === opt.value"
        @change="emit('update:modelValue', opt.value)"
      />
      {{ opt.label }}
    </label>
  </div>
</template>

<style scoped>
.comments-picker {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 1rem;
  min-height: 2.1rem;
}
.radio {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: fit-content;
  max-width: 100%;
  margin: 0;
  color: var(--text);
  font-size: 0.9rem;
  cursor: pointer;
}
.radio input {
  flex: 0 0 auto;
}
</style>
