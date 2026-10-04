<script setup lang="ts">
const { toasts, dismiss } = useToast();
</script>

<template>
  <div class="app-toast-region" aria-live="polite" aria-relevant="additions">
    <TransitionGroup name="app-toast">
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="app-toast"
        :class="toast.tone"
        role="status"
      >
        <div class="app-toast-icon" aria-hidden="true">
          <svg v-if="toast.tone === 'success'" viewBox="0 0 24 24" fill="none">
            <path
              d="M20 7 10.2 17 4 10.8"
              stroke="currentColor"
              stroke-width="2.25"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <svg v-else viewBox="0 0 24 24" fill="none">
            <path
              d="M12 8v5.5M12 16.5h.01"
              stroke="currentColor"
              stroke-width="2.25"
              stroke-linecap="round"
            />
            <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.75" />
          </svg>
        </div>
        <div class="app-toast-copy">
          <p class="app-toast-title">{{ toast.title }}</p>
          <p v-if="toast.detail" class="app-toast-detail">{{ toast.detail }}</p>
        </div>
        <button
          type="button"
          class="app-toast-dismiss"
          aria-label="Dismiss"
          @click="dismiss(toast.id)"
        >
          ×
        </button>
        <span
          class="app-toast-progress"
          :style="{ animationDuration: `${toast.durationMs}ms` }"
          aria-hidden="true"
        />
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.app-toast-region {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 1.25rem;
  z-index: 80;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.55rem;
  padding: 0 1rem;
  pointer-events: none;
}

.app-toast {
  pointer-events: auto;
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  width: min(26rem, 100%);
  padding: 0.85rem 0.95rem 0.95rem;
  border: 1px solid var(--border);
  border-radius: calc(var(--radius) + 2px);
  background: color-mix(in srgb, var(--bg-elevated) 92%, #000);
  box-shadow:
    0 12px 32px rgba(0, 0, 0, 0.35),
    0 0 0 1px rgba(255, 255, 255, 0.03) inset;
  overflow: hidden;
}

.app-toast.success {
  border-color: color-mix(in srgb, var(--success) 45%, var(--border));
}

.app-toast.error {
  border-color: color-mix(in srgb, var(--danger) 45%, var(--border));
}

.app-toast-icon {
  flex-shrink: 0;
  width: 1.85rem;
  height: 1.85rem;
  border-radius: 999px;
  display: grid;
  place-items: center;
  margin-top: 0.05rem;
}

.app-toast.success .app-toast-icon {
  color: var(--success);
  background: color-mix(in srgb, var(--success) 16%, transparent);
}

.app-toast.error .app-toast-icon {
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 16%, transparent);
}

.app-toast-icon svg {
  width: 1.05rem;
  height: 1.05rem;
}

.app-toast-copy {
  min-width: 0;
  flex: 1;
  padding-right: 0.25rem;
}

.app-toast-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  line-height: 1.3;
  color: var(--text);
}

.app-toast-detail {
  margin: 0.2rem 0 0;
  font-size: 0.85rem;
  line-height: 1.35;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-toast-dismiss {
  flex-shrink: 0;
  width: 1.6rem;
  height: 1.6rem;
  margin: -0.15rem -0.2rem 0 0;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 1.15rem;
  line-height: 1;
}

.app-toast-dismiss:hover {
  color: var(--text);
  border-color: transparent;
  background: color-mix(in srgb, var(--bg) 70%, transparent);
}

.app-toast-progress {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 2px;
  transform-origin: left center;
  animation-name: app-toast-progress;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

.app-toast.success .app-toast-progress {
  background: color-mix(in srgb, var(--success) 80%, #fff);
}

.app-toast.error .app-toast-progress {
  background: color-mix(in srgb, var(--danger) 80%, #fff);
}

@keyframes app-toast-progress {
  from {
    transform: scaleX(1);
  }
  to {
    transform: scaleX(0);
  }
}

.app-toast-enter-active,
.app-toast-leave-active {
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
}

.app-toast-enter-from,
.app-toast-leave-to {
  opacity: 0;
  transform: translateY(0.7rem) scale(0.98);
}

.app-toast-move {
  transition: transform 0.22s ease;
}
</style>
