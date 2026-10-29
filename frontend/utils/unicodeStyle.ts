/** Letter styles stored as Unicode mathematical alphanumeric characters. */
export type LetterStyle = "plain" | "bold" | "italic" | "boldItalic" | "mono";

export type FormatAction = "bold" | "italic" | "mono" | "underline" | "strike";

const UNDERLINE = 0x0332;
const STRIKE = 0x0336;

const KEY_RE =
  /^[a-zA-Z][a-zA-Z0-9_-]*(?:\[[0-9]+\])?(?::[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)*)?$/;

type Unit = {
  /** ASCII base, or the original character when it is not a styled letter or digit. */
  plain: string;
  style: LetterStyle;
  underline: boolean;
  strike: boolean;
  /** Combining marks other than underline and strikethrough. */
  extra: string;
};

export type StyledToken = {
  start: number;
  end: number;
  key: string;
  style: LetterStyle;
  underline: boolean;
  strike: boolean;
};

export type FormatState = {
  bold: boolean;
  italic: boolean;
  mono: boolean;
  underline: boolean;
  strike: boolean;
};

function isCombining(cp: number): boolean {
  return (
    (cp >= 0x0300 && cp <= 0x036f) ||
    (cp >= 0x1ab0 && cp <= 0x1aff) ||
    (cp >= 0x1dc0 && cp <= 0x1dff) ||
    (cp >= 0x20d0 && cp <= 0x20ff) ||
    (cp >= 0xfe20 && cp <= 0xfe2f)
  );
}

function cpLen(cp: number): number {
  return cp > 0xffff ? 2 : 1;
}

/** Map one code point to an ASCII letter or digit and its letter style. */
export function foldCodePoint(cp: number): { plain: string; style: LetterStyle } | null {
  if (cp >= 0x41 && cp <= 0x5a) return { plain: String.fromCharCode(cp), style: "plain" };
  if (cp >= 0x61 && cp <= 0x7a) return { plain: String.fromCharCode(cp), style: "plain" };
  if (cp >= 0x30 && cp <= 0x39) return { plain: String.fromCharCode(cp), style: "plain" };

  if (cp >= 0x1d400 && cp <= 0x1d419) {
    return { plain: String.fromCharCode(0x41 + (cp - 0x1d400)), style: "bold" };
  }
  if (cp >= 0x1d41a && cp <= 0x1d433) {
    return { plain: String.fromCharCode(0x61 + (cp - 0x1d41a)), style: "bold" };
  }
  if (cp >= 0x1d7ce && cp <= 0x1d7d7) {
    return { plain: String.fromCharCode(0x30 + (cp - 0x1d7ce)), style: "bold" };
  }

  if (cp >= 0x1d434 && cp <= 0x1d44d) {
    return { plain: String.fromCharCode(0x41 + (cp - 0x1d434)), style: "italic" };
  }
  if (cp === 0x210e) return { plain: "h", style: "italic" };
  if (cp >= 0x1d44e && cp <= 0x1d454) {
    return { plain: String.fromCharCode(0x61 + (cp - 0x1d44e)), style: "italic" };
  }
  if (cp >= 0x1d456 && cp <= 0x1d467) {
    return { plain: String.fromCharCode(0x69 + (cp - 0x1d456)), style: "italic" };
  }

  if (cp >= 0x1d468 && cp <= 0x1d481) {
    return { plain: String.fromCharCode(0x41 + (cp - 0x1d468)), style: "boldItalic" };
  }
  if (cp >= 0x1d482 && cp <= 0x1d49b) {
    return { plain: String.fromCharCode(0x61 + (cp - 0x1d482)), style: "boldItalic" };
  }

  if (cp >= 0x1d670 && cp <= 0x1d689) {
    return { plain: String.fromCharCode(0x41 + (cp - 0x1d670)), style: "mono" };
  }
  if (cp >= 0x1d68a && cp <= 0x1d6a3) {
    return { plain: String.fromCharCode(0x61 + (cp - 0x1d68a)), style: "mono" };
  }
  if (cp >= 0x1d7f6 && cp <= 0x1d7ff) {
    return { plain: String.fromCharCode(0x30 + (cp - 0x1d7f6)), style: "mono" };
  }
  return null;
}

function styleCodePoint(plain: string, style: LetterStyle): string {
  if (plain.length !== 1 || style === "plain") return plain;
  const c = plain.charCodeAt(0);
  const isUpper = c >= 0x41 && c <= 0x5a;
  const isLower = c >= 0x61 && c <= 0x7a;
  const isDigit = c >= 0x30 && c <= 0x39;
  if (!isUpper && !isLower && !isDigit) return plain;

  if (isDigit) {
    if (style === "bold" || style === "boldItalic") return String.fromCodePoint(0x1d7ce + (c - 0x30));
    if (style === "mono") return String.fromCodePoint(0x1d7f6 + (c - 0x30));
    return plain;
  }

  const offset = isUpper ? c - 0x41 : c - 0x61;
  if (style === "bold") {
    return String.fromCodePoint((isUpper ? 0x1d400 : 0x1d41a) + offset);
  }
  if (style === "italic") {
    if (!isUpper && plain === "h") return "\u210e";
    if (isUpper) return String.fromCodePoint(0x1d434 + offset);
    if (offset < 7) return String.fromCodePoint(0x1d44e + offset);
    return String.fromCodePoint(0x1d456 + (offset - 8));
  }
  if (style === "boldItalic") {
    return String.fromCodePoint((isUpper ? 0x1d468 : 0x1d482) + offset);
  }
  if (style === "mono") {
    return String.fromCodePoint((isUpper ? 0x1d670 : 0x1d68a) + offset);
  }
  return plain;
}

/** Fold styled letters the same way as server/placeholders/unicode_style.go. */
export function foldPlain(text: string): string {
  let out = "";
  for (const unit of parseUnits(text)) {
    out += unit.plain;
  }
  return out;
}

function readCluster(
  text: string,
  i: number,
): { unit: Unit; next: number } | null {
  if (i >= text.length) return null;
  const cp = text.codePointAt(i)!;
  if (isCombining(cp)) return null;
  const folded = foldCodePoint(cp);
  const unit: Unit = {
    plain: folded ? folded.plain : String.fromCodePoint(cp),
    style: folded ? folded.style : "plain",
    underline: false,
    strike: false,
    extra: "",
  };
  let next = i + cpLen(cp);
  while (next < text.length) {
    const mark = text.codePointAt(next)!;
    if (!isCombining(mark)) break;
    if (mark === UNDERLINE) unit.underline = true;
    else if (mark === STRIKE) unit.strike = true;
    else unit.extra += String.fromCodePoint(mark);
    next += cpLen(mark);
  }
  return { unit, next };
}

function parseUnits(text: string): Unit[] {
  const units: Unit[] = [];
  let i = 0;
  while (i < text.length) {
    const cluster = readCluster(text, i);
    if (!cluster) {
      const cp = text.codePointAt(i)!;
      units.push({
        plain: String.fromCodePoint(cp),
        style: "plain",
        underline: cp === UNDERLINE,
        strike: cp === STRIKE,
        extra: "",
      });
      i += cpLen(cp);
      continue;
    }
    units.push(cluster.unit);
    i = cluster.next;
  }
  return units;
}

function writeUnit(unit: Unit): string {
  let out = styleCodePoint(unit.plain, unit.style);
  if (!/^[A-Za-z0-9]$/.test(unit.plain)) out = unit.plain;
  out += unit.extra;
  if (unit.underline) out += "\u0332";
  if (unit.strike) out += "\u0336";
  return out;
}

function isLetterOrDigit(plain: string): boolean {
  return /^[A-Za-z0-9]$/.test(plain);
}

function toggleBoldStyle(style: LetterStyle, enable: boolean): LetterStyle {
  if (enable) {
    if (style === "italic" || style === "boldItalic") return "boldItalic";
    return "bold";
  }
  if (style === "boldItalic") return "italic";
  if (style === "bold") return "plain";
  return style;
}

function toggleItalicStyle(style: LetterStyle, enable: boolean): LetterStyle {
  if (enable) {
    if (style === "bold" || style === "boldItalic") return "boldItalic";
    return "italic";
  }
  if (style === "boldItalic") return "bold";
  if (style === "italic") return "plain";
  return style;
}

function relevant(units: Unit[]): Unit[] {
  return units.filter((u) => isLetterOrDigit(u.plain));
}

/** Apply or remove one format on a slice of text. */
export function toggleFormat(text: string, action: FormatAction): string {
  const units = parseUnits(text);
  if (!units.length) return text;
  const letters = relevant(units);

  if (action === "underline" || action === "strike") {
    const flag = action === "underline" ? "underline" : "strike";
    const enable = units.some((u) => !u[flag]);
    for (const unit of units) unit[flag] = enable;
    return units.map(writeUnit).join("");
  }

  if (!letters.length) return text;

  if (action === "bold") {
    const enable = letters.some((u) => u.style !== "bold" && u.style !== "boldItalic");
    for (const unit of letters) unit.style = toggleBoldStyle(unit.style, enable);
  } else if (action === "italic") {
    const enable = letters.some((u) => u.style !== "italic" && u.style !== "boldItalic");
    for (const unit of letters) unit.style = toggleItalicStyle(unit.style, enable);
  } else if (action === "mono") {
    const enable = letters.some((u) => u.style !== "mono");
    for (const unit of letters) unit.style = enable ? "mono" : "plain";
  }
  return units.map(writeUnit).join("");
}

/** Toolbar pressed-state for a selection. */
export function formatState(text: string): FormatState {
  const units = parseUnits(text);
  const letters = relevant(units);
  const every = (pred: (u: Unit) => boolean) => letters.length > 0 && letters.every(pred);
  return {
    bold: every((u) => u.style === "bold" || u.style === "boldItalic"),
    italic: every((u) => u.style === "italic" || u.style === "boldItalic"),
    mono: every((u) => u.style === "mono"),
    underline: units.length > 0 && units.every((u) => u.underline),
    strike: units.length > 0 && units.every((u) => u.strike),
  };
}

/** Render letters and digits in a letter style, then add decorations to every character. */
export function applyLetterStyle(
  text: string,
  style: LetterStyle,
  underline: boolean,
  strike: boolean,
): string {
  if (!text) return text;
  if (style === "plain" && !underline && !strike) return text;
  const units = parseUnits(text);
  for (const unit of units) {
    if (style !== "plain" && isLetterOrDigit(unit.plain)) unit.style = style;
    if (underline) unit.underline = true;
    if (strike) unit.strike = true;
  }
  return units.map(writeUnit).join("");
}

function tokenFrom(text: string, start: number): StyledToken | null {
  const first = readCluster(text, start);
  if (!first || first.unit.plain !== "{") return null;
  const second = readCluster(text, first.next);
  if (!second || second.unit.plain !== "{") return null;

  const inner: { unit: Unit }[] = [];
  let i = second.next;
  while (i < text.length) {
    const cluster = readCluster(text, i);
    if (!cluster) return null;
    if (cluster.unit.plain === "\n") return null;
    if (cluster.unit.plain === "}") {
      const close = readCluster(text, cluster.next);
      if (close && close.unit.plain === "}") {
        const folded = inner.map((c) => c.unit.plain).join("");
        const key = folded.trim();
        if (!KEY_RE.test(key)) return null;
        let style: LetterStyle = "plain";
        let underline = false;
        let strike = false;
        for (const part of inner) {
          if (!isLetterOrDigit(part.unit.plain) && !/[_:\-\[\]]/.test(part.unit.plain)) continue;
          if (style === "plain" && part.unit.style !== "plain") style = part.unit.style;
          if (part.unit.underline) underline = true;
          if (part.unit.strike) strike = true;
        }
        return { start, end: close.next, key, style, underline, strike };
      }
    }
    inner.push(cluster);
    i = cluster.next;
  }
  return null;
}

/** Placeholder tokens, including ones whose letters use a Unicode letter style. */
export function findStyledTokens(text: string): StyledToken[] {
  const tokens: StyledToken[] = [];
  let i = 0;
  while (i < text.length) {
    const cp = text.codePointAt(i)!;
    if (cp === 0x7b) {
      const tok = tokenFrom(text, i);
      if (tok) {
        tokens.push(tok);
        i = tok.end;
        continue;
      }
    }
    const cluster = readCluster(text, i);
    i = cluster ? cluster.next : i + cpLen(cp);
  }
  return tokens;
}

/** Range replaced when accepting a placeholder suggestion at the caret. */
export function autocompleteRange(
  text: string,
  caret: number,
): { start: number; end: number; query: string } | null {
  const pos = Math.max(0, Math.min(caret, text.length));
  let i = 0;
  let openAt = -1;
  let queryStart = -1;
  while (i < pos) {
    const cluster = readCluster(text, i);
    if (!cluster || cluster.next > pos) break;
    if (cluster.unit.plain === "{") {
      const second = readCluster(text, cluster.next);
      if (second && second.unit.plain === "{" && second.next <= pos) {
        openAt = i;
        queryStart = second.next;
        i = second.next;
        continue;
      }
    }
    if (cluster.unit.plain === "}" && openAt >= 0) {
      const second = readCluster(text, cluster.next);
      if (second && second.unit.plain === "}" && second.next <= pos) {
        openAt = -1;
        queryStart = -1;
        i = second.next;
        continue;
      }
    }
    i = cluster.next;
  }
  if (openAt < 0 || queryStart < 0) return null;

  const tokens = findStyledTokens(text);
  const enclosing = tokens.find((t) => t.start === openAt && pos <= t.end);
  const end = enclosing ? enclosing.end : pos;
  const query = foldPlain(text.slice(queryStart, pos)).trim();
  if (!/^[a-zA-Z0-9_:\-\[\]]*$/.test(query)) return null;
  return { start: openAt, end, query };
}
