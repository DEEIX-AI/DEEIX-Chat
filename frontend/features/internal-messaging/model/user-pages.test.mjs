import assert from "node:assert/strict";
import { test } from "node:test";
import { loadUserPages } from "./user-pages.ts";

test("refresh updates avatars on every loaded directory page without dropping users", async () => {
  const users = Array.from({ length: 75 }, (_, id) => ({
    publicID: `user_${id}`, username: `user${id}`, displayName: `User ${id}`,
    avatarURL: "file:file_old",
  }));
  const fetchPage = async (page) => ({
    results: users.slice((page - 1) * 50, page * 50), total: users.length, hasMore: page === 1,
  });
  const first = await loadUserPages(fetchPage, 1, new AbortController().signal);
  assert.equal(first.snapshot.results.length, 50);
  assert.equal(first.snapshot.hasMore, true);
  const all = await loadUserPages(fetchPage, 2, new AbortController().signal);
  users[0] = { ...users[0], avatarURL: "generated:github:42" };
  users[74] = { ...users[74], avatarURL: "file:file_new", displayName: "Updated" };
  users[60] = { ...users[60], avatarURL: "" };
  const refreshed = await loadUserPages(fetchPage, all.loaded, new AbortController().signal);
  assert.equal(refreshed.snapshot.results.length, 75);
  assert.equal(refreshed.snapshot.results[0].avatarURL, "generated:github:42");
  assert.equal(refreshed.snapshot.results[74].avatarURL, "file:file_new");
  assert.equal(refreshed.snapshot.results[74].displayName, "Updated");
  assert.equal(refreshed.snapshot.results[60].avatarURL, "");
  assert.equal(refreshed.snapshot.hasMore, false);
});

test("a cancelled directory search cannot return a stale profile snapshot", async () => {
  const controller = new AbortController();
  let calls = 0;
  await assert.rejects(loadUserPages(async () => {
    calls++;
    controller.abort();
    return { results: [{ publicID: "old-query-user" }], total: 2, hasMore: true };
  }, 2, controller.signal), { name: "AbortError" });
  assert.equal(calls, 1);
});

test("directory refresh does not publish a partial snapshot when a later page fails", async () => {
  await assert.rejects(loadUserPages(async (page) => {
    if (page === 2) throw new Error("offline");
    return { results: [{ publicID: "first-page-user" }], total: 2, hasMore: true };
  }, 2, new AbortController().signal), /offline/);
});

test("directory changes between pages do not duplicate users and shrink loaded page count", async () => {
  const { snapshot, loaded } = await loadUserPages(async (page) => ({
    results: [{ publicID: "same-user", avatarURL: `file:file_${page}` }],
    total: 1, hasMore: page === 1,
  }), 3, new AbortController().signal);
  assert.equal(snapshot.results.length, 1);
  assert.equal(snapshot.results[0].avatarURL, "file:file_2");
  assert.equal(loaded, 2);
});
