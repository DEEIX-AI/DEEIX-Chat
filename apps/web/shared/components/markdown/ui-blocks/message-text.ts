// A reply as text for the clipboard: each interactive block becomes readable
// Markdown in the state the user left it; everything else is the reply as
// written. A block that cannot be described (unknown or invalid component,
// a custom sandbox component, still streaming) is kept as source.

import { parseBlockSource, readMessageUIBlocks, replaceUIBlockFences, UI_BLOCK_FENCE_LANGUAGE } from "@deeix/core";

import { resolveDefinition, type UIBlockRegistry } from "./block";
import { liveBlockText } from "./live/live-text";
import type { UIBlockState } from "./ui-block-state";

export function messageUIBlocksText(content: string, registry: UIBlockRegistry, stateFor: (blockKey: string) => UIBlockState | undefined): string {
  if (!content.includes(UI_BLOCK_FENCE_LANGUAGE)) {
    return content;
  }
  // Repeated keys never store state (see useMessageUIBlockStateBinding).
  const { duplicates } = readMessageUIBlocks(content);
  return replaceUIBlockFences(content, (fence) => {
    // Illustrations are program source: kept as written.
    if (!fence.closed || fence.kind === "widget") {
      return null;
    }
    const parsed = parseBlockSource(fence.body, false);
    if (parsed.status !== "ok" || !parsed.block.complete) {
      return null;
    }
    const { block } = parsed;
    const state = block.key && !duplicates.has(block.key) ? stateFor(block.key) : undefined;
    try {
      if (block.live) {
        return liveBlockText(block, state, registry) || null;
      }
      return resolveDefinition(registry, block.component, block.version)?.toText(block.root.props, state) || null;
    } catch {
      // Copying must never fail because one block could not be described.
      return null;
    }
  });
}
