import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { classifyAuthError } from "./errors.ts";

describe("classifyAuthError", () => {
  it("treats non-401 as other", () => {
    assert.equal(classifyAuthError({ status: 500 }), "other");
    assert.equal(classifyAuthError({ status: 403, errorCode: "auth.invalid_token" }), "other");
    assert.equal(classifyAuthError(null), "other");
    assert.equal(classifyAuthError({}), "other");
  });

  it("treats plain 401 as recoverable", () => {
    assert.equal(classifyAuthError({ status: 401 }), "unauthorized");
    assert.equal(classifyAuthError({ status: 401, errorCode: "auth.token_expired" }), "unauthorized");
  });

  it("recognises session-terminating codes", () => {
    for (const code of ["auth.invalid_token", "auth.invalid_refresh_token", "auth.session_invalid"]) {
      assert.equal(classifyAuthError({ status: 401, errorCode: code }), "session_terminated");
    }
  });
});
