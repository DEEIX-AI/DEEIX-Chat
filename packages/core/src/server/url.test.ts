import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { normalizeApiBaseUrl, resolveApiBaseUrl } from "./url.ts";

describe("normalizeApiBaseUrl", () => {
  it("strips whitespace and trailing slashes", () => {
    assert.equal(normalizeApiBaseUrl("  https://api.example.com/// "), "https://api.example.com");
    assert.equal(normalizeApiBaseUrl("https://api.example.com/base/"), "https://api.example.com/base");
  });

  it("handles long runs of slashes in linear time", () => {
    const path = `/${"/".repeat(50_000)}x`;
    const started = performance.now();
    assert.equal(normalizeApiBaseUrl(`https://api.example.com${path}`), `https://api.example.com${path}`);
    assert.equal(normalizeApiBaseUrl(`https://api.example.com${"/".repeat(50_000)}`), "https://api.example.com");
    assert.ok(performance.now() - started < 200);
  });

  it("keeps explicit ports and IPv6 hosts", () => {
    assert.equal(normalizeApiBaseUrl("http://localhost:8080"), "http://localhost:8080");
    assert.equal(normalizeApiBaseUrl("http://[::1]:8080/"), "http://[::1]:8080");
  });

  it("rejects empty, relative and non-http values", () => {
    assert.equal(normalizeApiBaseUrl(""), "");
    assert.equal(normalizeApiBaseUrl(undefined), "");
    assert.equal(normalizeApiBaseUrl("/api"), "");
    assert.equal(normalizeApiBaseUrl("api.example.com"), "");
    assert.equal(normalizeApiBaseUrl("javascript:alert(1)"), "");
    assert.equal(normalizeApiBaseUrl("ftp://api.example.com"), "");
  });

  it("rejects credentials, query and fragment", () => {
    assert.equal(normalizeApiBaseUrl("https://user:pw@api.example.com"), "");
    assert.equal(normalizeApiBaseUrl("https://api.example.com/?x=1"), "");
    assert.equal(normalizeApiBaseUrl("https://api.example.com/#frag"), "");
  });
});

describe("resolveApiBaseUrl", () => {
  const page = { hostname: "chat.example.com", port: "", origin: "https://chat.example.com" };

  it("prefers the runtime override", () => {
    assert.equal(
      resolveApiBaseUrl({ runtimeOverride: "https://my.server/", configured: "https://cfg", location: page }),
      "https://my.server",
    );
  });

  it("falls back to build config, then to the page origin", () => {
    assert.equal(resolveApiBaseUrl({ configured: "https://cfg/", location: page }), "https://cfg");
    assert.equal(resolveApiBaseUrl({ location: page }), "https://chat.example.com");
  });

  it("ignores invalid override and config values", () => {
    assert.equal(resolveApiBaseUrl({ runtimeOverride: "nope", configured: "/api", location: page }), page.origin);
  });

  it("returns empty outside a browser with no config", () => {
    assert.equal(resolveApiBaseUrl({}), "");
  });

  it("points loopback dev pages at the Go server on 8080", () => {
    assert.equal(
      resolveApiBaseUrl({ location: { hostname: "localhost", port: "3000", origin: "http://localhost:3000" } }),
      "http://localhost:8080",
    );
    assert.equal(
      resolveApiBaseUrl({ location: { hostname: "::1", port: "3000", origin: "http://[::1]:3000" } }),
      "http://[::1]:8080",
    );
    assert.equal(
      resolveApiBaseUrl({ location: { hostname: "localhost", port: "8080", origin: "http://localhost:8080" } }),
      "http://localhost:8080",
    );
  });
});
