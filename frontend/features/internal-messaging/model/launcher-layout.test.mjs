import assert from "node:assert/strict";
import { test } from "node:test";

import {
  clampLauncherPoint,
  dockLauncherPoint,
  parseMessagingLayout,
  restoreLauncherPosition,
} from "./launcher-layout.ts";

test("invalid persisted JSON can be replaced while valid window bounds survive launcher saves", () => {
  for (const raw of [null, "{broken", "null", "[]", "true"]) {
    assert.deepEqual(parseMessagingLayout(raw), {});
  }
  const stored = parseMessagingLayout('{"window":{"x":20,"y":40,"width":400,"height":600}}');
  const updated = { ...stored, button: { x: 8, y: 240 }, buttonEdge: "left" };
  assert.deepEqual(updated.window, { x: 20, y: 40, width: 400, height: 600 });
});

test("docking chooses the nearest edge by button center and preserves height", () => {
  assert.deepEqual(dockLauncherPoint({ x: 300, y: 240 }, 1000, 800), {
    edge: "left", point: { x: 8, y: 240 },
  });
  assert.deepEqual(dockLauncherPoint({ x: 479, y: 240 }, 1000, 800), {
    edge: "right", point: { x: 948, y: 240 },
  });
});

test("dragging is constrained to the viewport before release", () => {
  assert.deepEqual(clampLauncherPoint({ x: -100, y: 1000 }, 1000, 800), { x: 8, y: 748 });
  assert.deepEqual(clampLauncherPoint({ x: 2000, y: -100 }, 1000, 800), { x: 948, y: 8 });
});

test("resizing preserves the docked edge even when the old position crosses the new midpoint", () => {
  assert.deepEqual(dockLauncherPoint({ x: 650, y: 900 }, 1800, 700, "right"), {
    edge: "right", point: { x: 1748, y: 648 },
  });
});

test("legacy free-floating positions migrate to an edge without losing height", () => {
  assert.deepEqual(restoreLauncherPosition({ button: { x: 120, y: 320 } }, 1000, 800), {
    edge: "left", point: { x: 8, y: 320 },
  });
});

test("saved edge takes precedence after a viewport change", () => {
  assert.deepEqual(restoreLauncherPosition({
    button: { x: 700, y: 320 }, buttonEdge: "right", window: { x: 20, y: 20 },
  }, 1800, 800), { edge: "right", point: { x: 1748, y: 320 } });
});

test("missing or corrupt storage falls back to a visible bottom-right button", () => {
  for (const stored of [undefined, null, "bad", {}, { button: null }, { button: { x: "8", y: 20 } }, { button: { x: NaN, y: Infinity } }]) {
    assert.deepEqual(restoreLauncherPosition(stored, 1000, 800), {
      edge: "right", point: { x: 948, y: 736 },
    });
  }
});
