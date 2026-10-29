/** Size a textarea from one line up to maxLines, then scroll. */
export function fitTextarea(el: HTMLTextAreaElement, maxLines = 3) {
  const style = getComputedStyle(el);
  const fontSize = parseFloat(style.fontSize) || 16;
  const parsedLine = parseFloat(style.lineHeight);
  const line = Number.isFinite(parsedLine) ? parsedLine : fontSize * 1.45;
  const pad = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
  const border =
    (parseFloat(style.borderTopWidth) || 0) + (parseFloat(style.borderBottomWidth) || 0);
  const min = line + pad + border;
  const max = line * maxLines + pad + border;
  el.style.height = "auto";
  const next = Math.min(Math.max(el.scrollHeight, min), max);
  el.style.height = `${next}px`;
  el.style.overflowY = el.scrollHeight > max + 1 ? "auto" : "hidden";
}

export const vGrow = {
  mounted(el: HTMLTextAreaElement) {
    fitTextarea(el);
  },
  updated(el: HTMLTextAreaElement) {
    fitTextarea(el);
  },
};
