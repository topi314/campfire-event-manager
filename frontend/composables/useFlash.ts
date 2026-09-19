const FLASH_KEY = "campfire-event-manager.flash";

export type FlashMessage = {
  text: string;
  tone?: "success" | "error";
};

export function useFlash() {
  const message = useState<FlashMessage | null>("flashMessage", () => null);

  function consumeFlash() {
    if (!import.meta.client) return null;
    try {
      const raw = sessionStorage.getItem(FLASH_KEY);
      if (raw) {
        sessionStorage.removeItem(FLASH_KEY);
        const parsed = JSON.parse(raw) as FlashMessage;
        if (parsed?.text) {
          message.value = parsed;
          return parsed;
        }
      }
    } catch {
      /* ignore */
    }
    return message.value;
  }

  function setFlash(text: string, tone: FlashMessage["tone"] = "success") {
    const next = { text, tone };
    message.value = next;
    if (import.meta.client) {
      try {
        sessionStorage.setItem(FLASH_KEY, JSON.stringify(next));
      } catch {
        /* ignore */
      }
    }
  }

  function clearFlash() {
    message.value = null;
    if (import.meta.client) {
      try {
        sessionStorage.removeItem(FLASH_KEY);
      } catch {
        /* ignore */
      }
    }
  }

  return { message, setFlash, consumeFlash, clearFlash };
}
