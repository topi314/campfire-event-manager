<script setup lang="ts">
export type IconName =
  | "post"
  | "edit"
  | "trash"
  | "publish"
  | "unpublish"
  | "export"
  | "add"
  | "plus"
  | "search"
  | "clear"
  | "save"
  | "create";

defineProps<{
  name: IconName;
}>();
</script>

<template>
  <svg class="ui-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <!-- Post / send -->
    <template v-if="name === 'post'">
      <path
        d="M4.5 12.5 19 5l-3.5 14-3.2-5.2L4.5 12.5Z"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
      />
      <path
        d="M12.3 13.8 19 5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Edit / pencil -->
    <template v-else-if="name === 'edit'">
      <path
        d="M4 20h4.2L18.6 9.6a2 2 0 0 0 0-2.8l-1.4-1.4a2 2 0 0 0-2.8 0L4 15.8V20Z"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
      />
      <path
        d="m13 7 4 4"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Trash -->
    <template v-else-if="name === 'trash'">
      <path d="M5 7h14" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
      <path
        d="M9 7V5.8A1.8 1.8 0 0 1 10.8 4h2.4A1.8 1.8 0 0 1 15 5.8V7"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="M8 7h8l-.7 11.2a1.8 1.8 0 0 1-1.8 1.7h-3a1.8 1.8 0 0 1-1.8-1.7L8 7Z"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
      />
    </template>

    <!-- Publish / share up -->
    <template v-else-if="name === 'publish'">
      <path
        d="M12 16V5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="m8 8.5 4-3.5 4 3.5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5 15v2.5A1.5 1.5 0 0 0 6.5 19h11a1.5 1.5 0 0 0 1.5-1.5V15"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Unpublish / eye off-ish down -->
    <template v-else-if="name === 'unpublish'">
      <path
        d="M12 8v11"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="m8 15.5 4 3.5 4-3.5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5 9V6.5A1.5 1.5 0 0 1 6.5 5h11A1.5 1.5 0 0 1 19 6.5V9"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Export / download -->
    <template v-else-if="name === 'export'">
      <path
        d="M12 4v10"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="m8.5 10.5 3.5 3.5 3.5-3.5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5 17v1.5A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5V17"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Add to mine / copy-plus -->
    <template v-else-if="name === 'add'">
      <path
        d="M9 4H7.5A2.5 2.5 0 0 0 5 6.5v11A2.5 2.5 0 0 0 7.5 20H15a2.5 2.5 0 0 0 2.5-2.5V15"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="M10 4h4.5A2.5 2.5 0 0 1 17 6.5V13"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path d="M14 10h6" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
      <path d="M17 7v6" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
    </template>

    <!-- Plus -->
    <template v-else-if="name === 'plus' || name === 'create'">
      <path d="M12 5v14" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
      <path d="M5 12h14" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
    </template>

    <!-- Search -->
    <template v-else-if="name === 'search'">
      <circle cx="11" cy="11" r="6.25" stroke="currentColor" stroke-width="1.75" />
      <path
        d="m16 16 3.5 3.5"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
    </template>

    <!-- Clear / broom-ish X stack -->
    <template v-else-if="name === 'clear'">
      <path
        d="M5 7h14"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linecap="round"
      />
      <path
        d="M8 7h8v10.2a1.8 1.8 0 0 1-1.8 1.8H9.8A1.8 1.8 0 0 1 8 17.2V7Z"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
      />
      <path d="M10 11v5" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
      <path d="M14 11v5" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
    </template>

    <!-- Save / disk -->
    <template v-else-if="name === 'save'">
      <path
        d="M6 4h10.5L19 6.5V18a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z"
        stroke="currentColor"
        stroke-width="1.75"
        stroke-linejoin="round"
      />
      <path d="M8 4v5h7V4" stroke="currentColor" stroke-width="1.75" stroke-linejoin="round" />
      <path d="M8 16h8" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" />
    </template>
  </svg>
</template>

<style scoped>
.ui-icon {
  width: 1.15rem;
  height: 1.15rem;
  display: block;
  flex-shrink: 0;
}
</style>
