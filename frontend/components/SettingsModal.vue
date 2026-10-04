<script setup lang="ts">
import type { Club } from "~/types";

const props = defineProps<{
  open: boolean;
  /** Jump scroll / highlight to the JWT section (missing or invalidated token). */
  focusToken?: boolean;
}>();

const emit = defineEmits<{
  close: [];
}>();

const { api } = useApi();
const { token, authHeaders, save, clearToken, tokenError } = useSessionToken();
const {
  timeZone,
  setTimeZone,
  listTimeZones,
  browserTimeZone,
  preferredClubId,
  setPreferredClubId,
  bugHexeEnabled,
  setBugHexeEnabled,
} = usePreferences();
const { me, isCA, checking, validateToken } = useCampfireSession();
const { user, logout } = useAuth();
const { bugHexeUserIds } = useClientConfig();
const toast = useToast();

const draftToken = ref("");
const draftTz = ref("");
const draftClubId = ref("");
const draftBugHexe = ref(true);
const showBugHexeSetting = computed(
  () => !!user.value && bugHexeUserIds.value.includes(user.value.id),
);
const zones = ref<string[]>([]);
const clubs = ref<Club[]>([]);
const clubsLoading = ref(false);
const clubsError = ref("");
const tokenSection = ref<HTMLElement | null>(null);
const loggingOut = ref(false);
const tokenVisible = ref(false);
const saving = ref(false);
const saveError = ref("");
const tokenHowtoBrowser = ref<"chrome" | "edge" | "firefox" | "safari">("chrome");

async function loadClubs() {
  if (!token.value) {
    clubs.value = [];
    clubsError.value = "";
    return;
  }
  clubsLoading.value = true;
  clubsError.value = "";
  try {
    clubs.value = await api<Club[]>("/api/campfire/clubs", { headers: authHeaders() });
  } catch (e: any) {
    clubs.value = [];
    clubsError.value = e?.message || "Could not load clubs.";
  } finally {
    clubsLoading.value = false;
  }
}

watch(
  () =>
    [
      props.open,
      props.focusToken,
      token.value,
      timeZone.value,
      preferredClubId.value,
      bugHexeEnabled.value,
    ] as const,
  ([open], prev) => {
    if (open && !prev?.[0]) tokenVisible.value = false;
    if (!open) return;
    draftToken.value = token.value;
    draftTz.value = timeZone.value;
    draftClubId.value = preferredClubId.value;
    draftBugHexe.value = bugHexeEnabled.value;
    zones.value = listTimeZones();
    if (!zones.value.includes(draftTz.value)) {
      zones.value = [...zones.value, draftTz.value].sort((a, b) => a.localeCompare(b));
    }
    saveError.value = "";
    void loadClubs();
    if (props.focusToken) {
      nextTick(() => {
        tokenSection.value?.scrollIntoView({ behavior: "smooth", block: "nearest" });
      });
    }
  },
  { immediate: true },
);

async function onSave() {
  setTimeZone(draftTz.value || browserTimeZone());
  setPreferredClubId(draftClubId.value);
  if (showBugHexeSetting.value) setBugHexeEnabled(draftBugHexe.value);
  saveError.value = "";
  const nextToken = draftToken.value.trim();
  if (nextToken) {
    saving.value = true;
    try {
      if (!save(nextToken)) {
        saveError.value = "Could not save token.";
        return;
      }
      const ok = await validateToken({ openSettingsOnFail: false });
      if (!ok) {
        saveError.value =
          tokenError.value ||
          "Campfire rejected this token. Copy a fresh sessionToken and try again.";
        return;
      }
    } finally {
      saving.value = false;
    }
  }
  emit("close");
  toast.success("Settings saved");
}

function onClearToken() {
  clearToken();
  draftToken.value = "";
  saveError.value = "";
}

async function onLogout() {
  loggingOut.value = true;
  try {
    emit("close");
    await logout();
    await navigateTo("/login");
  } finally {
    loggingOut.value = false;
  }
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.open) emit("close");
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
    aria-labelledby="settings-title"
    @click.self="emit('close')"
  >
    <div class="modal modal-settings">
      <div class="settings-header">
        <h2 id="settings-title">Settings</h2>
        <button type="button" class="icon-close" aria-label="Close" @click="emit('close')">
          ×
        </button>
      </div>

      <div class="settings-section">
        <h3>Timezone</h3>
        <p class="muted">
          Meetup start/end times and template clock times are interpreted in this zone when
          posting to Campfire.
        </p>
        <div class="field" style="margin-bottom: 0">
          <label for="settings-tz">IANA timezone</label>
          <select id="settings-tz" v-model="draftTz">
            <option v-for="z in zones" :key="z" :value="z">{{ z }}</option>
          </select>
        </div>
        <p class="hint muted">Detected browser zone: {{ browserTimeZone() }}</p>
      </div>

      <div class="settings-section">
        <h3>Default club</h3>
        <p class="muted">
          Auto-selected on the create/edit flow when you open the app. Falls back to the first
          club in the list if unset or unavailable.
        </p>
        <div class="field" style="margin-bottom: 0">
          <label for="settings-club">Preferred club</label>
          <select
            id="settings-club"
            v-model="draftClubId"
            :disabled="!token || clubsLoading"
          >
            <option value="">First available club</option>
            <option
              v-if="draftClubId && !clubs.some((c) => c.id === draftClubId)"
              :value="draftClubId"
            >
              Saved club (not in current list)
            </option>
            <option v-for="c in clubs" :key="c.id" :value="c.id">
              {{ c.name }}
            </option>
          </select>
        </div>
        <p v-if="!token" class="hint muted">Add a Campfire session token below to load clubs.</p>
        <p v-else-if="clubsLoading" class="hint muted">Loading clubs…</p>
        <p v-else-if="clubsError" class="hint error">{{ clubsError }}</p>
        <p v-else-if="!clubs.length" class="hint muted">No clubs where you are an admin.</p>
      </div>

      <div
        ref="tokenSection"
        class="settings-section"
        :class="{ highlight: focusToken }"
      >
        <h3>Campfire session token</h3>
        <p class="muted">
          Required to load clubs, live events, and create meetups. Stored only in this browser
          under <code>campfire-event-manager.sessionToken</code>.
        </p>
        <p v-if="focusToken || tokenError || saveError" class="error" style="margin: 0">
          {{
            saveError ||
            tokenError ||
            "Your Campfire token is missing or no longer valid. Paste a fresh JWT below."
          }}
        </p>
        <details class="token-howto">
          <summary>
            <span class="token-howto-chevron" aria-hidden="true"></span>
            <span class="token-howto-summary-text">
              <span class="token-howto-title">How to get a token</span>
              <span class="token-howto-sub">Copy it from Campfire Local Storage</span>
            </span>
          </summary>
          <div class="token-howto-body">
            <div class="token-howto-browsers" role="tablist" aria-label="Browser">
              <button
                type="button"
                role="tab"
                :class="{ active: tokenHowtoBrowser === 'chrome' }"
                :aria-selected="tokenHowtoBrowser === 'chrome'"
                @click="tokenHowtoBrowser = 'chrome'"
              >
                Chrome
              </button>
              <button
                type="button"
                role="tab"
                :class="{ active: tokenHowtoBrowser === 'edge' }"
                :aria-selected="tokenHowtoBrowser === 'edge'"
                @click="tokenHowtoBrowser = 'edge'"
              >
                Edge
              </button>
              <button
                type="button"
                role="tab"
                :class="{ active: tokenHowtoBrowser === 'firefox' }"
                :aria-selected="tokenHowtoBrowser === 'firefox'"
                @click="tokenHowtoBrowser = 'firefox'"
              >
                Firefox
              </button>
              <button
                type="button"
                role="tab"
                :class="{ active: tokenHowtoBrowser === 'safari' }"
                :aria-selected="tokenHowtoBrowser === 'safari'"
                @click="tokenHowtoBrowser = 'safari'"
              >
                Safari
              </button>
            </div>

            <ol v-if="tokenHowtoBrowser === 'chrome'" class="token-howto-steps">
              <li>
                <span class="token-howto-step">1</span>
                <span>
                  Open
                  <a href="https://campfire.scopely.com/discover" target="_blank" rel="noreferrer"
                    >campfire.scopely.com/discover</a
                  >
                  and sign in.
                </span>
              </li>
              <li>
                <span class="token-howto-step">2</span>
                <span>
                  Press <kbd>F12</kbd> (or right-click → Inspect) and open the
                  <strong>Application</strong> tab.
                </span>
              </li>
              <li>
                <span class="token-howto-step">3</span>
                <span>
                  In the left sidebar, expand <strong>Local Storage</strong> and select
                  <code>https://campfire.scopely.com</code>.
                </span>
              </li>
              <li>
                <span class="token-howto-step">4</span>
                <span>
                  Find <code>CapacitorStorage.sessionToken</code> and copy its value
                  (<code>eyJ…</code>).
                </span>
              </li>
              <li>
                <span class="token-howto-step">5</span>
                <span>
                  Paste it below. Surrounding quotes are fine.
                </span>
              </li>
            </ol>

            <ol v-else-if="tokenHowtoBrowser === 'edge'" class="token-howto-steps">
              <li>
                <span class="token-howto-step">1</span>
                <span>
                  Open
                  <a href="https://campfire.scopely.com/discover" target="_blank" rel="noreferrer"
                    >campfire.scopely.com/discover</a
                  >
                  and sign in.
                </span>
              </li>
              <li>
                <span class="token-howto-step">2</span>
                <span>
                  Press <kbd>F12</kbd> (or right-click → Inspect) and open the
                  <strong>Application</strong> tab. If you don’t see it, open the
                  <strong>≫</strong> menu in the DevTools toolbar.
                </span>
              </li>
              <li>
                <span class="token-howto-step">3</span>
                <span>
                  In the left sidebar, expand <strong>Local Storage</strong> and select
                  <code>https://campfire.scopely.com</code>.
                </span>
              </li>
              <li>
                <span class="token-howto-step">4</span>
                <span>
                  Find <code>CapacitorStorage.sessionToken</code> and copy its value
                  (<code>eyJ…</code>).
                </span>
              </li>
              <li>
                <span class="token-howto-step">5</span>
                <span>
                  Paste it below. Surrounding quotes are fine.
                </span>
              </li>
            </ol>

            <ol v-else-if="tokenHowtoBrowser === 'firefox'" class="token-howto-steps">
              <li>
                <span class="token-howto-step">1</span>
                <span>
                  Open
                  <a href="https://campfire.scopely.com/discover" target="_blank" rel="noreferrer"
                    >campfire.scopely.com/discover</a
                  >
                  and sign in.
                </span>
              </li>
              <li>
                <span class="token-howto-step">2</span>
                <span>
                  Press <kbd>F12</kbd> (or right-click → Inspect) and open the
                  <strong>Storage</strong> tab.
                </span>
              </li>
              <li>
                <span class="token-howto-step">3</span>
                <span>
                  In the left sidebar, expand <strong>Local Storage</strong> and select
                  <code>https://campfire.scopely.com</code>.
                </span>
              </li>
              <li>
                <span class="token-howto-step">4</span>
                <span>
                  Find <code>CapacitorStorage.sessionToken</code> and copy its value
                  (<code>eyJ…</code>).
                </span>
              </li>
              <li>
                <span class="token-howto-step">5</span>
                <span>
                  Paste it below. Surrounding quotes are fine.
                </span>
              </li>
            </ol>

            <ol v-else class="token-howto-steps">
              <li>
                <span class="token-howto-step">1</span>
                <span>
                  Open
                  <a href="https://campfire.scopely.com/discover" target="_blank" rel="noreferrer"
                    >campfire.scopely.com/discover</a
                  >
                  and sign in.
                </span>
              </li>
              <li>
                <span class="token-howto-step">2</span>
                <span>
                  Enable the Develop menu if needed:
                  <strong>Safari → Settings → Advanced → Show features for web developers</strong>.
                </span>
              </li>
              <li>
                <span class="token-howto-step">3</span>
                <span>
                  Choose <strong>Develop → Show Web Inspector</strong> (or
                  <kbd>⌥⌘I</kbd>), then open the <strong>Storage</strong> tab.
                </span>
              </li>
              <li>
                <span class="token-howto-step">4</span>
                <span>
                  Under <strong>Local Storage</strong>, select
                  <code>https://campfire.scopely.com</code>, then find
                  <code>CapacitorStorage.sessionToken</code> and copy its value
                  (<code>eyJ…</code>).
                </span>
              </li>
              <li>
                <span class="token-howto-step">5</span>
                <span>
                  Paste it below. Surrounding quotes are fine.
                </span>
              </li>
            </ol>
          </div>
        </details>
        <div class="field" style="margin-bottom: 0">
          <div class="token-label-row">
            <label for="settings-jwt">JWT</label>
            <button
              type="button"
              class="token-reveal"
              :aria-pressed="tokenVisible"
              @click="tokenVisible = !tokenVisible"
            >
              {{ tokenVisible ? "Hide" : "Show" }}
            </button>
          </div>
          <textarea
            id="settings-jwt"
            v-model="draftToken"
            rows="4"
            autocomplete="off"
            spellcheck="false"
            placeholder="eyJ…"
            :class="{ 'token-masked': !tokenVisible }"
          />
        </div>
        <p v-if="checking || saving" class="muted" style="margin: 0">Checking token with Campfire…</p>
        <p v-else-if="token && me" class="success" style="margin: 0">
          Signed in to Campfire as {{ me.displayName || me.username }}
          <span v-if="isCA"> · Community Ambassador</span>
        </p>
        <p v-else-if="token" class="muted" style="margin: 0">A token is saved in this browser.</p>
        <p v-else class="muted" style="margin: 0">No token saved yet.</p>
      </div>

      <div v-if="showBugHexeSetting" class="settings-section">
        <h3>Bug Hexe</h3>
        <p class="muted">
          A little witch flies through when you open the app. Turn this off if you’d rather not.
        </p>
        <label class="bug-hexe-toggle">
          <input v-model="draftBugHexe" type="checkbox" />
          <span>Show Bug Hexe on visit</span>
        </label>
      </div>

      <div class="modal-actions">
        <button type="button" :disabled="(!token && !draftToken) || saving" @click="onClearToken">
          Clear token
        </button>
        <button type="button" :disabled="saving" @click="emit('close')">Cancel</button>
        <button type="button" class="primary" :disabled="saving" @click="onSave">
          {{ saving ? "Saving…" : "Save" }}
        </button>
      </div>
      <div class="settings-footer-actions">
        <button
          type="button"
          class="danger"
          :disabled="loggingOut"
          @click="onLogout"
        >
          {{ loggingOut ? "Logging out…" : "Log out" }}
        </button>
      </div>

      <AppFooter />
    </div>
  </div>
</template>

<style scoped>
.settings-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.75rem;
  margin-bottom: 0.85rem;
}
.settings-header h2 {
  margin: 0;
  font-size: 1.15rem;
}
.icon-close {
  flex-shrink: 0;
  width: 2rem;
  height: 2rem;
  padding: 0;
  font-size: 1.35rem;
  line-height: 1;
}
.settings-section {
  margin-bottom: 1.15rem;
  padding-bottom: 1rem;
  border-bottom: 1px solid var(--border);
}
.settings-section:last-of-type {
  border-bottom: none;
  margin-bottom: 0.5rem;
  padding-bottom: 0;
}
.settings-section h3 {
  margin: 0 0 0.4rem;
  font-size: 0.95rem;
}
.settings-section.highlight {
  outline: 1px solid var(--accent);
  outline-offset: 6px;
  border-radius: var(--radius);
}
.hint {
  margin: 0.35rem 0 0;
  font-size: 0.85rem;
}
.bug-hexe-toggle {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  margin: 0;
  color: var(--text);
  font-size: 0.92rem;
  cursor: pointer;
}
.bug-hexe-toggle input {
  width: auto;
  margin: 0;
}
.token-howto {
  margin: 0.75rem 0 1rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
  overflow: hidden;
}
.token-howto summary {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  padding: 0.75rem 0.9rem;
  cursor: pointer;
  list-style: none;
  user-select: none;
  color: var(--text);
}
.token-howto summary::-webkit-details-marker {
  display: none;
}
.token-howto summary::marker {
  content: "";
}
.token-howto summary:hover {
  background: color-mix(in srgb, var(--bg-elevated) 70%, var(--bg));
}
.token-howto[open] summary {
  border-bottom: 1px solid var(--border);
  background: var(--bg-elevated);
}
.token-howto-chevron {
  flex-shrink: 0;
  width: 0.55rem;
  height: 0.55rem;
  border-right: 2px solid var(--muted);
  border-bottom: 2px solid var(--muted);
  transform: rotate(-45deg);
  transition: transform 0.15s ease;
  margin-left: 0.15rem;
}
.token-howto[open] .token-howto-chevron {
  transform: rotate(45deg);
  margin-top: -0.15rem;
}
.token-howto-summary-text {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}
.token-howto-title {
  font-weight: 600;
  font-size: 0.92rem;
  line-height: 1.3;
}
.token-howto-sub {
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 1.3;
}
.token-howto-body {
  padding: 0.75rem 0.9rem 0.25rem;
}
.token-howto-browsers {
  display: flex;
  flex-wrap: wrap;
  gap: 0.15rem;
  margin: 0 0 0.15rem;
  padding: 0.2rem;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-input);
}
.token-howto-browsers button {
  margin: 0;
  flex: 1 1 auto;
  min-width: 4.5rem;
  padding: 0.35rem 0.65rem;
  border: none;
  border-radius: calc(var(--radius) - 2px);
  background: transparent;
  color: var(--muted);
  font: inherit;
  font-size: 0.82rem;
  font-weight: 500;
  cursor: pointer;
}
.token-howto-browsers button.active {
  background: var(--bg);
  color: var(--text);
  box-shadow: 0 0 0 1px var(--border);
}
.token-howto-browsers button:hover:not(.active) {
  color: var(--text);
}
.token-howto-steps {
  margin: 0;
  padding: 0.7rem 0 1rem;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
}
.token-howto-steps li {
  display: flex;
  gap: 0.7rem;
  align-items: flex-start;
  margin: 0;
  color: var(--muted);
  font-size: 0.88rem;
  line-height: 1.45;
}
.token-howto-step {
  flex-shrink: 0;
  width: 1.45rem;
  height: 1.45rem;
  border-radius: 999px;
  display: grid;
  place-items: center;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  color: var(--text);
  font-size: 0.72rem;
  font-weight: 700;
  line-height: 1;
  margin-top: 0.05rem;
}
.token-howto a {
  color: var(--accent);
}
.token-howto code {
  font-size: 0.84em;
  word-break: break-all;
}
.token-howto kbd {
  display: inline-block;
  padding: 0.05rem 0.35rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg-input);
  color: var(--text);
  font-size: 0.8em;
  font-family: inherit;
}
.token-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.3rem;
}
.token-label-row label {
  margin-bottom: 0;
}
.token-reveal {
  padding: 0.15rem 0.55rem;
  font-size: 0.8rem;
}
.token-masked {
  -webkit-text-security: disc;
}
.settings-footer-actions {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
.settings-footer-actions .danger {
  width: 100%;
}
</style>
