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

test("animates submitted messages to the bottom over a controlled duration", () => {
  assert.equal(chatScroll.CHAT_SEND_SCROLL_DURATION_MS, 700);
  assert.equal(typeof chatScroll.animateChatScrollToBottom, "function");

  const viewport = { scrollTop: 100, scrollHeight: 1100, clientHeight: 100 };
  const frames = [];
  let nextFrameID = 0;
  const cancelled = [];
  const cancel = chatScroll.animateChatScrollToBottom(
    viewport,
    (callback) => {
      frames.push(callback);
      nextFrameID += 1;
      return nextFrameID;
    },
    (frameID) => cancelled.push(frameID),
  );

  frames.shift()(0);
  assert.equal(viewport.scrollTop, 100);
  frames.shift()(350);
  assert.equal(viewport.scrollTop, 550);
  frames.shift()(700);
  assert.equal(viewport.scrollTop, 1000);

  cancel();
  assert.deepEqual(cancelled, [3]);
});

test("starts smooth scrolling after pending-message layout observers settle", () => {
  assert.equal(typeof chatScroll.schedulePendingUserScroll, "function");

  const frames = [];
  const cancelled = [];
  let nextFrameID = 0;
  let scrollCount = 0;
  const cancel = chatScroll.schedulePendingUserScroll(
    (callback) => {
      frames.push(callback);
      nextFrameID += 1;
      return nextFrameID;
    },
    (frameID) => cancelled.push(frameID),
    () => {
      scrollCount += 1;
    },
  );

  assert.equal(scrollCount, 0);
  frames.shift()(0);
  assert.equal(scrollCount, 0);
  frames.shift()(16);
  assert.equal(scrollCount, 1);

  cancel();
  assert.deepEqual(cancelled, [1, 2]);
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
