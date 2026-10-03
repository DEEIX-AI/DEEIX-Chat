import { isRecord } from "@/shared/lib/type-guards";
import { MAX_PINNED_MODEL_CONTROLS, type ModelControlPlacement } from "@/entities/model";

// User overrides of where a control is shown, keyed by control id (shared by all models).
export const MODEL_CONTROL_PLACEMENTS_SETTING_KEY = "chat.model_control_placements";

export type ModelControlPlacements = Record<string, ModelControlPlacement>;

export function parseModelControlPlacements(raw: string | null | undefined): ModelControlPlacements {
  try {
    const parsed: unknown = raw ? JSON.parse(raw) : {};
    if (!isRecord(parsed)) return {};
    const result: ModelControlPlacements = {};
    for (const [id, placement] of Object.entries(parsed)) {
      if (placement === "toolbar" || placement === "menu") result[id] = placement;
    }
    return result;
  } catch {
    return {};
  }
}

// A composer item the user can place: a model control or a provider-native tool toggle.
export type ComposerControlItem = {
  id: string;
  defaultPlacement: ModelControlPlacement;
  pinnable: boolean;
};

/**
 * Splits items into toolbar and menu. User overrides win over the administrator default; at most
 * MAX_PINNED_MODEL_CONTROLS items reach the toolbar, in declaration order.
 */
export function splitComposerControlItems<T extends ComposerControlItem>(
  items: readonly T[],
  placements: ModelControlPlacements,
): { toolbar: T[]; menu: T[] } {
  const toolbar: T[] = [];
  const menu: T[] = [];
  for (const item of items) {
    const placement = item.pinnable ? placements[item.id] ?? item.defaultPlacement : "menu";
    if (placement === "toolbar" && toolbar.length < MAX_PINNED_MODEL_CONTROLS) {
      toolbar.push(item);
    } else {
      menu.push(item);
    }
  }
  return { toolbar, menu };
}

/**
 * Next placements after pinning or unpinning an item. Only differences from the item default are
 * stored; pinning beyond the limit returns null.
 */
export function nextModelControlPlacements<T extends ComposerControlItem>(
  items: readonly T[],
  placements: ModelControlPlacements,
  item: T,
  pinned: boolean,
): ModelControlPlacements | null {
  const next: ModelControlPlacements = { ...placements };
  const target: ModelControlPlacement = pinned ? "toolbar" : "menu";
  if (target === item.defaultPlacement) {
    delete next[item.id];
  } else {
    next[item.id] = target;
  }
  const pinnedCount = items.filter((candidate) =>
    candidate.pinnable && (next[candidate.id] ?? candidate.defaultPlacement) === "toolbar"
  ).length;
  if (pinned && pinnedCount > MAX_PINNED_MODEL_CONTROLS) {
    return null;
  }
  // Placements of controls from other models are kept, but only the pinned ones count toward the
  // server-side limit, so drop stale pins that would push the total over it.
  const visibleIDs = new Set(items.map((candidate) => candidate.id));
  const stalePins = Object.entries(next).filter(([id, placement]) => placement === "toolbar" && !visibleIDs.has(id));
  let overflow = Object.values(next).filter((placement) => placement === "toolbar").length - MAX_PINNED_MODEL_CONTROLS;
  for (const [id] of stalePins) {
    if (overflow <= 0) break;
    delete next[id];
    overflow -= 1;
  }
  return next;
}
