const COMMAND_RECENTS_STORAGE_KEY = "deeix.chat.command.recents.v1";
const COMMAND_RECENTS_MAX_ENTRIES_PER_KIND = 20;

export type CommandRecentsKind = "model" | "file" | "tool" | "skill" | "prompt";

export type CommandRecentsEntry = {
  id: string;
  usedAt: number;
};

type CommandRecentsState = Partial<Record<CommandRecentsKind, CommandRecentsEntry[]>>;

let memoryState: CommandRecentsState | null = null;

function isCommandRecentsEntry(value: unknown): value is CommandRecentsEntry {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const entry = value as { id?: unknown; usedAt?: unknown };
  return typeof entry.id === "string" && entry.id.length > 0 && typeof entry.usedAt === "number";
}

function sanitizeState(value: unknown): CommandRecentsState {
  if (typeof value !== "object" || value === null) {
    return {};
  }
  const state: CommandRecentsState = {};
  for (const kind of ["model", "file", "tool", "skill", "prompt"] as const) {
    const entries = (value as Record<string, unknown>)[kind];
    if (!Array.isArray(entries)) {
      continue;
    }
    const seen = new Set<string>();
    const sanitized: CommandRecentsEntry[] = [];
    for (const entry of entries) {
      if (!isCommandRecentsEntry(entry) || seen.has(entry.id)) {
        continue;
      }
      seen.add(entry.id);
      sanitized.push({ id: entry.id, usedAt: entry.usedAt });
    }
    sanitized.sort((a, b) => b.usedAt - a.usedAt);
    state[kind] = sanitized.slice(0, COMMAND_RECENTS_MAX_ENTRIES_PER_KIND);
  }
  return state;
}

function loadState(): CommandRecentsState {
  if (memoryState) {
    return memoryState;
  }
  if (typeof window === "undefined") {
    return {};
  }
  try {
    const raw = window.localStorage.getItem(COMMAND_RECENTS_STORAGE_KEY);
    memoryState = raw ? sanitizeState(JSON.parse(raw) as unknown) : {};
  } catch {
    memoryState = {};
  }
  return memoryState;
}

function persistState(state: CommandRecentsState) {
  memoryState = state;
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(COMMAND_RECENTS_STORAGE_KEY, JSON.stringify(state));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

/** Most-recent-first entries for one kind. Do not call during render; read in effects or handlers. */
export function readCommandRecents(kind: CommandRecentsKind): CommandRecentsEntry[] {
  return (loadState()[kind] ?? []).map((entry) => ({ ...entry }));
}

export function recordCommandRecentUsage(kind: CommandRecentsKind, id: string) {
  const normalizedID = id.trim();
  if (!normalizedID) {
    return;
  }
  const state = loadState();
  const entries = (state[kind] ?? []).filter((entry) => entry.id !== normalizedID);
  entries.unshift({ id: normalizedID, usedAt: Date.now() });
  persistState({
    ...state,
    [kind]: entries.slice(0, COMMAND_RECENTS_MAX_ENTRIES_PER_KIND),
  });
}
