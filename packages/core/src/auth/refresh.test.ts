import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { classifyAuthError } from "./errors.ts";
import { type AuthHost, createAuthClient, type SessionCredentials, type SessionStore } from "./refresh.ts";

class MemoryStore implements SessionStore {
  accessToken = "";
  sessionID = "";
  revision = 0;
  cleared = 0;

  readAccessToken() {
    return this.accessToken;
  }
  readRevision() {
    return this.revision;
  }
  write(credentials: SessionCredentials) {
    this.accessToken = credentials.accessToken;
    this.sessionID = credentials.sessionID;
    this.revision += 1;
  }
  clear() {
    this.accessToken = "";
    this.sessionID = "";
    this.revision += 1;
    this.cleared += 1;
  }
}

type HttpError = { status: number; errorCode?: string };

const unauthorized: HttpError = { status: 401 };
const terminated: HttpError = { status: 401, errorCode: "auth.session_invalid" };
const serverError: HttpError = { status: 500 };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function makeHost(overrides: Partial<AuthHost> = {}) {
  const store = new MemoryStore();
  const calls = { refresh: 0 };
  const host: AuthHost = {
    store,
    refreshSession: async () => {
      calls.refresh += 1;
      return { accessToken: `token-${calls.refresh}`, sessionID: "s1" };
    },
    classifyError: (error) => classifyAuthError(error as HttpError),
    ...overrides,
  };
  return { host, store, calls, client: createAuthClient(host) };
}

describe("refreshAccessToken", () => {
  it("stores and returns the new token", async () => {
    const { client, store } = makeHost();
    assert.equal(await client.refreshAccessToken(), "token-1");
    assert.equal(store.accessToken, "token-1");
    assert.equal(store.sessionID, "s1");
  });

  it("deduplicates concurrent refreshes", async () => {
    const gate = deferred<SessionCredentials | null>();
    const { client, calls } = makeHost({
      refreshSession: () => {
        calls.refresh += 1;
        return gate.promise;
      },
    });
    const a = client.refreshAccessToken("old");
    const b = client.refreshAccessToken("old");
    gate.resolve({ accessToken: "fresh", sessionID: "s" });
    assert.deepEqual(await Promise.all([a, b]), ["fresh", "fresh"]);
    assert.equal(calls.refresh, 1);
  });

  it("allows a new refresh after the previous one settles", async () => {
    const { client, calls } = makeHost();
    const first = await client.refreshAccessToken();
    await client.refreshAccessToken(first);
    assert.equal(calls.refresh, 2);
  });

  it("skips the network when the store already holds a different token", async () => {
    const { client, store, calls } = makeHost();
    store.write({ accessToken: "rotated", sessionID: "s" });
    assert.equal(await client.refreshAccessToken("stale"), "rotated");
    assert.equal(calls.refresh, 0);
  });

  it("clears the session and resolves empty when the server returns no token", async () => {
    const { client, store } = makeHost({ refreshSession: async () => null });
    store.write({ accessToken: "x", sessionID: "s" });
    assert.equal(await client.refreshAccessToken("x"), "");
    assert.equal(store.accessToken, "");
    assert.equal(store.cleared, 1);
  });

  it("clears the session on a session-terminating error", async () => {
    const { client, store } = makeHost({
      refreshSession: async () => {
        throw terminated;
      },
    });
    store.write({ accessToken: "x", sessionID: "s" });
    assert.equal(await client.refreshAccessToken("x"), "");
    assert.equal(store.cleared, 1);
  });

  it("propagates other errors and keeps the session", async () => {
    const { client, store } = makeHost({
      refreshSession: async () => {
        throw serverError;
      },
    });
    store.write({ accessToken: "x", sessionID: "s" });
    await assert.rejects(client.refreshAccessToken("x"), (error) => error === serverError);
    assert.equal(store.accessToken, "x");
    assert.equal(store.cleared, 0);
  });

  it("does not clear a session that was replaced while refreshing", async () => {
    const gate = deferred<SessionCredentials | null>();
    const { client, store } = makeHost({ refreshSession: () => gate.promise });
    store.write({ accessToken: "x", sessionID: "s" });
    const pending = client.refreshAccessToken("x");
    // Another tab logs in with a new session before our refresh answers.
    store.write({ accessToken: "new-login", sessionID: "s2" });
    gate.resolve(null);
    assert.equal(await pending, "");
    assert.equal(store.accessToken, "new-login");
    assert.equal(store.cleared, 0);
  });

  it("runs the refresh inside the provided lock", async () => {
    const order: string[] = [];
    const { client } = makeHost({
      lock: {
        run: async (fn) => {
          order.push("lock:acquire");
          try {
            return await fn();
          } finally {
            order.push("lock:release");
          }
        },
      },
      refreshSession: async () => {
        order.push("refresh");
        return { accessToken: "t", sessionID: "s" };
      },
    });
    await client.refreshAccessToken();
    assert.deepEqual(order, ["lock:acquire", "refresh", "lock:release"]);
  });
});

describe("recoverAccessToken", () => {
  it("returns the current token when it differs from the failed one", async () => {
    const { client, store, calls } = makeHost();
    store.write({ accessToken: "current", sessionID: "s" });
    assert.equal(await client.recoverAccessToken("failed"), "current");
    assert.equal(calls.refresh, 0);
  });

  it("refreshes when the current token is the failed one", async () => {
    const { client, store, calls } = makeHost();
    store.write({ accessToken: "failed", sessionID: "s" });
    assert.equal(await client.recoverAccessToken("failed"), "token-1");
    assert.equal(calls.refresh, 1);
  });
});

describe("withAuthRetry", () => {
  it("returns the first successful result without refreshing", async () => {
    const { client, calls } = makeHost();
    const result = await client.withAuthRetry({ accessToken: "a", execute: async (token) => `ok:${token}` });
    assert.equal(result, "ok:a");
    assert.equal(calls.refresh, 0);
  });

  it("refreshes once and retries on 401", async () => {
    const { client, store } = makeHost();
    store.write({ accessToken: "a", sessionID: "s" });
    const seen: string[] = [];
    const result = await client.withAuthRetry({
      accessToken: "a",
      execute: async (token) => {
        seen.push(token);
        if (token === "a") throw unauthorized;
        return `ok:${token}`;
      },
    });
    assert.equal(result, "ok:token-1");
    assert.deepEqual(seen, ["a", "token-1"]);
  });

  it("rethrows the original error when refresh yields no token", async () => {
    const { client } = makeHost({ refreshSession: async () => null });
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async () => {
          throw unauthorized;
        },
      }),
      (error) => error === unauthorized,
    );
  });

  it("does not refresh for non-401 errors", async () => {
    const { client, calls } = makeHost();
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async () => {
          throw serverError;
        },
      }),
      (error) => error === serverError,
    );
    assert.equal(calls.refresh, 0);
  });

  it("does not refresh when allowRefresh is false", async () => {
    const { client, calls } = makeHost();
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        allowRefresh: false,
        execute: async () => {
          throw unauthorized;
        },
      }),
      (error) => error === unauthorized,
    );
    assert.equal(calls.refresh, 0);
  });

  it("clears the session when the retry fails with a terminating error", async () => {
    const { client, store } = makeHost();
    store.write({ accessToken: "a", sessionID: "s" });
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async (token) => {
          throw token === "a" ? unauthorized : terminated;
        },
      }),
      (error) => error === terminated,
    );
    assert.equal(store.accessToken, "");
  });

  it("keeps the session when the retry fails with a non-terminating error", async () => {
    const { client, store } = makeHost();
    store.write({ accessToken: "a", sessionID: "s" });
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async (token) => {
          throw token === "a" ? unauthorized : serverError;
        },
      }),
      (error) => error === serverError,
    );
    assert.equal(store.accessToken, "token-1");
  });

  it("throws the abort error instead of refreshing when the request was aborted", async () => {
    const abort = new Error("aborted");
    const { client, calls } = makeHost();
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async () => {
          throw unauthorized;
        },
        resolveAbort: (error) => (error ? abort : null),
      }),
      (error) => error === abort,
    );
    assert.equal(calls.refresh, 0);
  });

  it("throws the abort error after refresh when aborted meanwhile", async () => {
    const abort = new Error("aborted");
    let aborted = false;
    const { client, calls } = makeHost({
      refreshSession: async () => {
        calls.refresh += 1;
        aborted = true;
        return { accessToken: "t", sessionID: "s" };
      },
    });
    const seen: string[] = [];
    await assert.rejects(
      client.withAuthRetry({
        accessToken: "a",
        execute: async (token) => {
          seen.push(token);
          throw unauthorized;
        },
        resolveAbort: (error) => (error ? null : aborted ? abort : null),
      }),
      (error) => error === abort,
    );
    assert.deepEqual(seen, ["a"]);
    assert.equal(calls.refresh, 1);
  });
});
