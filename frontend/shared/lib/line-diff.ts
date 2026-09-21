// Line-level diff via longest common subsequence. Sized for the text a chat
// reply carries (hundreds of lines), not for whole repositories.

export type DiffLine = { kind: "same" | "added" | "removed"; text: string; oldNo?: number; newNo?: number };

const MAX_LINES = 2000;

export function diffLines(before: string, after: string): DiffLine[] {
  const a = before.split("\n");
  const b = after.split("\n");
  if (a.length > MAX_LINES || b.length > MAX_LINES) {
    return [
      ...a.map((text, index): DiffLine => ({ kind: "removed", text, oldNo: index + 1 })),
      ...b.map((text, index): DiffLine => ({ kind: "added", text, newNo: index + 1 })),
    ];
  }

  // lcs[i][j] = LCS length of a[i..] and b[j..]
  const lcs: Uint16Array[] = Array.from({ length: a.length + 1 }, () => new Uint16Array(b.length + 1));
  for (let i = a.length - 1; i >= 0; i -= 1) {
    const row = lcs[i] as Uint16Array;
    const next = lcs[i + 1] as Uint16Array;
    for (let j = b.length - 1; j >= 0; j -= 1) {
      row[j] = a[i] === b[j] ? (next[j + 1] ?? 0) + 1 : Math.max(next[j] ?? 0, row[j + 1] ?? 0);
    }
  }

  const result: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      result.push({ kind: "same", text: a[i] ?? "", oldNo: i + 1, newNo: j + 1 });
      i += 1;
      j += 1;
    } else if ((lcs[i + 1]?.[j] ?? 0) >= (lcs[i]?.[j + 1] ?? 0)) {
      result.push({ kind: "removed", text: a[i] ?? "", oldNo: i + 1 });
      i += 1;
    } else {
      result.push({ kind: "added", text: b[j] ?? "", newNo: j + 1 });
      j += 1;
    }
  }
  for (; i < a.length; i += 1) {
    result.push({ kind: "removed", text: a[i] ?? "", oldNo: i + 1 });
  }
  for (; j < b.length; j += 1) {
    result.push({ kind: "added", text: b[j] ?? "", newNo: j + 1 });
  }
  return result;
}

export type DiffSegment = { changed: boolean; text: string };

const MAX_TOKENS = 400;
const MIN_SIMILARITY = 0.5;

// Word-level highlight for a changed line pair: tokens split on word / space /
// punctuation boundaries, aligned with the same LCS as the lines.
export function diffWords(before: string, after: string): { before: DiffSegment[]; after: DiffSegment[] } {
  const whole = { before: [{ changed: true, text: before }], after: [{ changed: true, text: after }] };
  const a = before.match(/\w+|\s+|[^\w\s]/g) ?? [];
  const b = after.match(/\w+|\s+|[^\w\s]/g) ?? [];
  if (a.length > MAX_TOKENS || b.length > MAX_TOKENS) {
    return whole;
  }
  const lcs: Uint16Array[] = Array.from({ length: a.length + 1 }, () => new Uint16Array(b.length + 1));
  for (let i = a.length - 1; i >= 0; i -= 1) {
    const row = lcs[i] as Uint16Array;
    const next = lcs[i + 1] as Uint16Array;
    for (let j = b.length - 1; j >= 0; j -= 1) {
      row[j] = a[i] === b[j] ? (next[j + 1] ?? 0) + 1 : Math.max(next[j] ?? 0, row[j + 1] ?? 0);
    }
  }
  const left: DiffSegment[] = [];
  const right: DiffSegment[] = [];
  const push = (list: DiffSegment[], changed: boolean, text: string) => {
    const last = list[list.length - 1];
    if (last && last.changed === changed) {
      last.text += text;
    } else {
      list.push({ changed, text });
    }
  };
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      push(left, false, a[i] ?? "");
      push(right, false, b[j] ?? "");
      i += 1;
      j += 1;
    } else if ((lcs[i + 1]?.[j] ?? 0) >= (lcs[i]?.[j + 1] ?? 0)) {
      push(left, true, a[i] ?? "");
      i += 1;
    } else {
      push(right, true, b[j] ?? "");
      j += 1;
    }
  }
  for (; i < a.length; i += 1) push(left, true, a[i] ?? "");
  for (; j < b.length; j += 1) push(right, true, b[j] ?? "");
  // A rewritten line with a few coincidental matches reads worse fragmented
  // than plainly marked as changed; require meaningful non-space overlap.
  const kept = left.filter((segment) => !segment.changed).reduce((sum, segment) => sum + segment.text.trim().length, 0);
  const longest = Math.max(before.trim().length, after.trim().length, 1);
  if (kept / longest < MIN_SIMILARITY) {
    return whole;
  }
  return { before: left, after: right };
}

// Share of non-space text two lines have in common (0–1); the threshold
// diffWords uses to decide whether a pair is worth highlighting.
export function lineSimilarity(before: string, after: string): number {
  const words = diffWords(before, after);
  if (words.before.length === 1 && words.before[0]?.changed) {
    return 0;
  }
  const kept = words.before.filter((segment) => !segment.changed).reduce((sum, segment) => sum + segment.text.trim().length, 0);
  return kept / Math.max(before.trim().length, after.trim().length, 1);
}
