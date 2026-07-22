import assert from "node:assert/strict";
import test from "node:test";

import { resolveLiveAnchorMessageKey } from "./chat-scroll.ts";

test("anchors a newly pending user message so sending returns to the live turn", () => {
  assert.equal(
    resolveLiveAnchorMessageKey([
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

test("returns no anchor when the conversation has no live message", () => {
  assert.equal(
    resolveLiveAnchorMessageKey([
      { key: "user", role: "user" },
      { key: "assistant", role: "assistant" },
    ]),
    "",
  );
});
