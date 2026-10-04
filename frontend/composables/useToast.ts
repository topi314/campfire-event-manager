export type ToastTone = "success" | "error";

export type ToastMessage = {
  id: number;
  title: string;
  detail?: string;
  tone: ToastTone;
  durationMs: number;
};

const DEFAULT_DURATION_MS = 3500;

/** Client-only dismiss timers; not part of reactive SSR state. */
const timers = new Map<number, ReturnType<typeof setTimeout>>();

export function useToast() {
  const toasts = useState<ToastMessage[]>("appToasts", () => []);
  const nextId = useState("appToastNextId", () => 1);

  function dismiss(id: number) {
    const timer = timers.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.delete(id);
    }
    toasts.value = toasts.value.filter((t) => t.id !== id);
  }

  function show(opts: {
    title: string;
    detail?: string;
    tone?: ToastTone;
    durationMs?: number;
  }) {
    const id = nextId.value++;
    const durationMs = opts.durationMs ?? DEFAULT_DURATION_MS;
    const toast: ToastMessage = {
      id,
      title: opts.title,
      detail: opts.detail,
      tone: opts.tone ?? "success",
      durationMs,
    };
    toasts.value = [...toasts.value, toast];
    if (import.meta.client && durationMs > 0) {
      timers.set(
        id,
        setTimeout(() => dismiss(id), durationMs),
      );
    }
    return id;
  }

  function success(title: string, detail?: string) {
    return show({ title, detail, tone: "success" });
  }

  function error(title: string, detail?: string) {
    return show({ title, detail, tone: "error" });
  }

  return { toasts, show, success, error, dismiss };
}
