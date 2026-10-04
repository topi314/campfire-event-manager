<script setup lang="ts">
import witchUrl from "~/assets/images/witch-broom.jpg";
import wandCursorUrl from "~/assets/images/zauberstab-cursor.svg";

/** Fly across, linger briefly, then leave. */
const VISIBLE_MS = 5200;
const SPARKLE_COUNT = 28;
const WAND_CURSOR_CLASS = "bug-hexe-wand-cursor";

const { user } = useAuth();
const { bugHexeUserIds } = useClientConfig();
const { bugHexeEnabled } = usePreferences();

const show = ref(false);
const played = useState("bugHexePlayed", () => false);
let hideTimer: ReturnType<typeof setTimeout> | null = null;

const eligible = computed(
  () =>
    !!user.value &&
    bugHexeEnabled.value &&
    bugHexeUserIds.value.includes(user.value.id),
);

const sparkles = Array.from({ length: SPARKLE_COUNT }, (_, i) => ({
  id: i,
  left: `${4 + ((i * 37) % 92)}%`,
  top: `${6 + ((i * 53) % 84)}%`,
  size: `${0.18 + (i % 5) * 0.08}rem`,
  delay: `${(i % 12) * 0.12}s`,
  duration: `${1.4 + (i % 6) * 0.25}s`,
}));

function setWandCursor(on: boolean) {
  if (!import.meta.client) return;
  document.documentElement.classList.toggle(WAND_CURSOR_CLASS, on);
  document.documentElement.style.setProperty(
    "--bug-hexe-wand-cursor",
    on ? `url("${wandCursorUrl}") 8 8, auto` : "",
  );
}

function dismiss() {
  if (hideTimer) {
    clearTimeout(hideTimer);
    hideTimer = null;
  }
  show.value = false;
}

watch(
  eligible,
  (ok) => {
    setWandCursor(ok);
    if (!ok || played.value || show.value) return;
    played.value = true;
    show.value = true;
    if (hideTimer) clearTimeout(hideTimer);
    hideTimer = setTimeout(dismiss, VISIBLE_MS);
  },
  { immediate: true },
);

watch(bugHexeEnabled, (enabled) => {
  if (!enabled) dismiss();
});

onBeforeUnmount(() => {
  if (hideTimer) clearTimeout(hideTimer);
  setWandCursor(false);
});
</script>

<template>
  <Teleport to="body">
    <Transition name="bug-hexe-layer">
      <div
        v-if="show"
        class="bug-hexe-layer"
        aria-label="Bug Hexe"
      >
        <div class="bug-hexe-sparkles" aria-hidden="true">
          <span
            v-for="s in sparkles"
            :key="s.id"
            class="bug-hexe-sparkle"
            :style="{
              left: s.left,
              top: s.top,
              width: s.size,
              height: s.size,
              animationDelay: s.delay,
              animationDuration: s.duration,
            }"
          />
        </div>

        <button
          type="button"
          class="bug-hexe"
          title="Dismiss Bug Hexe"
          aria-label="Dismiss Bug Hexe"
          @click="dismiss"
        >
          <img class="bug-hexe-img" :src="witchUrl" alt="" />
          <span class="bug-hexe-label">Bug Hexe</span>
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.bug-hexe-layer {
  position: fixed;
  inset: 0;
  z-index: 70;
  pointer-events: none;
  overflow: hidden;
}

.bug-hexe-sparkles {
  position: absolute;
  inset: 0;
}

.bug-hexe-sparkle {
  position: absolute;
  border-radius: 999px;
  background: #f0e2ff;
  box-shadow:
    0 0 6px 2px rgba(196, 150, 255, 0.85),
    0 0 14px 4px rgba(155, 109, 255, 0.35);
  opacity: 0;
  animation-name: bug-hexe-twinkle;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
}

.bug-hexe {
  position: absolute;
  left: 0;
  top: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
  padding: 0;
  border: none;
  background: transparent;
  color: inherit;
  cursor: pointer;
  pointer-events: auto;
  user-select: none;
  width: max-content;
  animation: bug-hexe-fly 5.2s cubic-bezier(0.35, 0.1, 0.25, 1) both;
}

.bug-hexe:hover {
  border-color: transparent;
}

.bug-hexe:focus-visible {
  outline: 2px solid color-mix(in srgb, #9b6dff 70%, var(--accent));
  outline-offset: 4px;
  border-radius: 1rem;
}

.bug-hexe-img {
  width: min(14rem, 46vw);
  height: auto;
  aspect-ratio: 4 / 3;
  object-fit: contain;
  object-position: center;
  border-radius: 1rem;
  background: #0f1419;
  border: 1px solid color-mix(in srgb, #9b6dff 35%, var(--border));
  box-shadow:
    0 14px 36px rgba(0, 0, 0, 0.45),
    0 0 24px rgba(155, 109, 255, 0.18);
  filter: drop-shadow(0 0 10px rgba(155, 109, 255, 0.25));
}

.bug-hexe-label {
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--bg-elevated) 88%, #6b3fd4);
  border: 1px solid color-mix(in srgb, #9b6dff 35%, var(--border));
  color: #e8ddff;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.02em;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.3);
}

/* Flight path: enter from left, arc across, pause bottom-right, exit up-right. */
@keyframes bug-hexe-fly {
  0% {
    opacity: 0;
    transform: translate3d(-30vw, 58vh, 0) rotate(-12deg) scale(0.78);
  }
  12% {
    opacity: 1;
  }
  38% {
    transform: translate3d(28vw, 22vh, 0) rotate(-6deg) scale(1);
  }
  58% {
    transform: translate3d(62vw, 48vh, 0) rotate(2deg) scale(1.02);
  }
  72% {
    transform: translate3d(calc(100vw - 15rem), calc(100vh - 12rem), 0) rotate(0deg) scale(1);
  }
  86% {
    opacity: 1;
    transform: translate3d(calc(100vw - 15rem), calc(100vh - 12.4rem), 0) rotate(-2deg) scale(1);
  }
  100% {
    opacity: 0;
    transform: translate3d(110vw, 18vh, 0) rotate(10deg) scale(0.92);
  }
}

@keyframes bug-hexe-twinkle {
  0%,
  100% {
    opacity: 0;
    transform: scale(0.35);
  }
  40% {
    opacity: 0.95;
    transform: scale(1);
  }
  70% {
    opacity: 0.35;
    transform: scale(0.7);
  }
}

.bug-hexe-layer-enter-active {
  transition: opacity 0.2s ease;
}

.bug-hexe-layer-leave-active {
  transition: opacity 0.35s ease;
}

.bug-hexe-layer-enter-from,
.bug-hexe-layer-leave-to {
  opacity: 0;
}

@media (max-width: 640px) {
  .bug-hexe-img {
    width: min(11rem, 58vw);
  }

  @keyframes bug-hexe-fly {
    0% {
      opacity: 0;
      transform: translate3d(-40vw, 60vh, 0) rotate(-12deg) scale(0.78);
    }
    12% {
      opacity: 1;
    }
    40% {
      transform: translate3d(18vw, 28vh, 0) rotate(-6deg) scale(1);
    }
    62% {
      transform: translate3d(42vw, 52vh, 0) rotate(2deg) scale(1);
    }
    76% {
      transform: translate3d(calc(100vw - 11.5rem), calc(100vh - 10.5rem), 0) rotate(0deg)
        scale(1);
    }
    88% {
      opacity: 1;
      transform: translate3d(calc(100vw - 11.5rem), calc(100vh - 10.8rem), 0) rotate(-2deg)
        scale(1);
    }
    100% {
      opacity: 0;
      transform: translate3d(110vw, 22vh, 0) rotate(10deg) scale(0.92);
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .bug-hexe {
    animation: none;
    left: auto;
    right: 1rem;
    top: auto;
    bottom: 1rem;
    opacity: 1;
    transform: none;
  }

  .bug-hexe-sparkle {
    animation: none;
    opacity: 0.55;
  }
}
</style>
