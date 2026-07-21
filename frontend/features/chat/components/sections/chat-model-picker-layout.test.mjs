import assert from "node:assert/strict";
import test from "node:test";

import {
  resolveDesktopMenuListMaxHeight,
  resolveDesktopModelMenuListMaxHeight,
} from "./chat-model-picker-layout.ts";

test("limits the list to the space remaining after panel chrome", () => {
  assert.equal(
    resolveDesktopModelMenuListMaxHeight({
      viewportTop: 24,
      viewportBottom: 144,
      sideOffset: 8,
      verticalChrome: 40,
    }),
    80,
  );
});

test("uses all normal viewport space remaining after panel chrome", () => {
  assert.equal(
    resolveDesktopModelMenuListMaxHeight({
      viewportTop: 24,
      viewportBottom: 424,
      sideOffset: 8,
      verticalChrome: 40,
    }),
    360,
  );
});

test("limits a positioned submenu to its remaining viewport space", () => {
  assert.equal(resolveDesktopMenuListMaxHeight(72, 12), 60);
});

test("uses the larger space below the trigger and deducts the side offset", () => {
  assert.equal(
    resolveDesktopModelMenuListMaxHeight({
      viewportTop: 24,
      viewportBottom: 424,
      triggerTop: 96,
      triggerBottom: 124,
      sideOffset: 8,
      verticalChrome: 40,
    }),
    252,
  );
});

test("uses the larger space above the trigger", () => {
  assert.equal(
    resolveDesktopModelMenuListMaxHeight({
      viewportTop: 24,
      viewportBottom: 424,
      triggerTop: 324,
      triggerBottom: 352,
      sideOffset: 8,
      verticalChrome: 40,
    }),
    252,
  );
});

test("returns zero when panel chrome consumes all available space", () => {
  assert.equal(
    resolveDesktopModelMenuListMaxHeight({
      viewportTop: 24,
      viewportBottom: 64,
      sideOffset: 8,
      verticalChrome: 40,
    }),
    0,
  );
});
