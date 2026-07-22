import assert from "node:assert/strict";
import test from "node:test";

import * as chatScroll from "./chat-scroll.ts";

const { resolveLiveAnchorMessageKey } = chatScroll;

test("keeps the previous user as the stream resize anchor", () => {
  assert.equal(
    resolveLiveAnchorMessageKey([
      { key: "previous-user", role: "user" },
      { key: "previous-assistant", role: "assistant" },
      { key: "pending-user", role: "user", isPending: true },
      { key: "pending-assistant", role: "assistant", isPending: true },
    ]),
    "previous-user",
  );
});

test("uses a newly pending user turn as the explicit scroll-to-bottom trigger", () => {
  assert.equal(typeof chatScroll.resolvePendingUserScrollKey, "function");
  assert.equal(
    chatScroll.resolvePendingUserScrollKey([
      { key: "previous-user", role: "user" },
      { key: "previous-assistant", role: "assistant" },
      { key: "pending-user", role: "user", isPending: true },
      { key: "pending-assistant", role: "assistant", isPending: true },
    ]),
    "pending-user",
  );
});

test("anchors an assistant-only live run to its parent user message", () => {
  assert.equal(
    resolveLiveAnchorMessageKey([
      { key: "parent-user", role: "user" },
      { key: "pending-assistant", role: "assistant", isStreaming: true },
    ]),
    "parent-user",
  );
});

test("returns no live anchor or submit trigger when the conversation has no live message", () => {
  const messages = [
    { key: "user", role: "user" },
    { key: "assistant", role: "assistant" },
  ];

  assert.equal(resolveLiveAnchorMessageKey(messages), "");
  assert.equal(typeof chatScroll.resolvePendingUserScrollKey, "function");
  assert.equal(chatScroll.resolvePendingUserScrollKey(messages), "");
});
