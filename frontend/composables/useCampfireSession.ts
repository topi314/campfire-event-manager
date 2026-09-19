import type { CampfireMe } from "~/types";
import { isCampfireTokenError } from "~/composables/useSessionToken";

const CA_BADGE_ALIAS = "PGO_COMMUNITY_AMBASSADOR";

/** Module-level coalescing for concurrent validateToken callers. */
let campfireValidateInflight: Promise<boolean> | null = null;

export function isCommunityAmbassador(me: CampfireMe | null | undefined): boolean {
  return !!me?.badges?.some((b) => b.alias === CA_BADGE_ALIAS);
}

export function useCampfireSession() {
  const { api } = useApi();
  const { token, authHeaders, invalidateToken, tokenError } = useSessionToken();

  const me = useState<CampfireMe | null>("campfireMe", () => null);
  const checking = useState("campfireTokenChecking", () => false);
  const validated = useState("campfireTokenValidated", () => false);
  const validatedForToken = useState<string | null>("campfireTokenValidatedFor", () => null);

  const isCA = computed(() => isCommunityAmbassador(me.value));

  async function validateToken(opts?: { openSettingsOnFail?: boolean; force?: boolean }) {
    if (!import.meta.client) return false;
    const current = token.value;
    if (!current) {
      me.value = null;
      validated.value = false;
      validatedForToken.value = null;
      return false;
    }

    if (
      !opts?.force &&
      validated.value &&
      me.value?.id &&
      validatedForToken.value === current
    ) {
      return true;
    }

    if (campfireValidateInflight) return campfireValidateInflight;

    checking.value = true;
    campfireValidateInflight = (async () => {
      try {
        const next = await api<CampfireMe>("/api/campfire/me", {
          headers: authHeaders(),
        });
        if (token.value !== current) return false;
        if (!next?.id) {
          throw new Error("invalid Campfire token");
        }
        me.value = next;
        tokenError.value = null;
        validated.value = true;
        validatedForToken.value = current;
        return true;
      } catch (e) {
        if (token.value !== current) return false;
        me.value = null;
        validated.value = false;
        validatedForToken.value = null;
        const msg = e instanceof Error ? e.message : String(e || "");
        if (isCampfireTokenError(e)) {
          if (opts?.openSettingsOnFail !== false) {
            invalidateToken(
              "Your Campfire token is invalid or expired. Paste a fresh one in Settings.",
            );
          } else {
            tokenError.value =
              "Your Campfire token is invalid or expired. Paste a fresh one in Settings.";
          }
        } else {
          tokenError.value = msg || "Could not verify Campfire token.";
        }
        return false;
      } finally {
        checking.value = false;
        campfireValidateInflight = null;
      }
    })();
    return campfireValidateInflight;
  }

  function clearCampfireProfile() {
    me.value = null;
    validated.value = false;
    validatedForToken.value = null;
  }

  return {
    me,
    isCA,
    checking,
    tokenError,
    validated,
    validateToken,
    clearCampfireProfile,
  };
}
