<script setup lang="ts">
defineProps<{
  allInvited: boolean;
}>();

const emit = defineEmits<{
  "update:allInvited": [boolean];
}>();
</script>

<template>
  <div class="invite-picker">
    <label class="radio">
      <input
        type="radio"
        name="inviteMode"
        :checked="allInvited"
        @change="emit('update:allInvited', true)"
      />
      Everybody (entire club)
    </label>
    <label class="radio">
      <input
        type="radio"
        name="inviteMode"
        :checked="!allInvited"
        @change="emit('update:allInvited', false)"
      />
      Nobody
    </label>
  </div>
</template>

<style scoped>
.invite-picker {
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
