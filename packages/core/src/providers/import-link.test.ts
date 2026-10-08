import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatProviderImportLink, maskProviderApiKey, parseProviderImportLink } from "./import-link.ts";

describe("parseProviderImportLink", () => {
  it("reads the New API template output", () => {
    // New API url-encodes {address} and prefixes {key} with sk-.
    const result = parseProviderImportLink("#v=1&url=https%3A%2F%2Fapi.relay.com&key=sk-abc123XYZ");
    assert.equal(result.ok, true);
    if (!result.ok) return;
    assert.deepEqual(result.link, {
      baseURL: "https://api.relay.com",
      apiKey: "sk-abc123XYZ",
      protocol: null,
      name: "",
      host: "api.relay.com",
    });
  });

  it("normalizes trailing slashes and host case", () => {
    const result = parseProviderImportLink("url=https%3A%2F%2FAPI.Relay.com%2Fv1%2F&key=sk-x");
    assert.equal(result.ok, true);
    if (!result.ok) return;
    assert.equal(result.link.baseURL, "https://api.relay.com/v1");
    assert.equal(result.link.host, "api.relay.com");
  });

  it("decodes encoded keys, protocols and names", () => {
    const result = parseProviderImportLink(
      "v=1&url=https%3A%2F%2Fapi.example.com%2Fv1&key=sk-a%2Bb%2Fc%3Dd&protocol=anthropic_messages&name=My%20relay",
    );
    assert.equal(result.ok, true);
    if (!result.ok) return;
    assert.equal(result.link.apiKey, "sk-a+b/c=d");
    assert.equal(result.link.protocol, "anthropic_messages");
    assert.equal(result.link.name, "My relay");
  });

  it("rejects links it should not guess about", () => {
    const cases: Array<[string, string]> = [
      ["", "empty"],
      ["#", "empty"],
      ["v=2&url=https%3A%2F%2Fa.com&key=k", "unsupported_version"],
      ["url=https%3A%2F%2Fa.com&key=k&headers=x", "unknown_parameter"],
      ["url=https%3A%2F%2Fa.com&url=https%3A%2F%2Fb.com&key=k", "duplicate_parameter"],
      ["key=k", "missing_url"],
      ["url=http%3A%2F%2Fa.com&key=k", "insecure_url"],
      ["url=javascript%3Aalert(1)&key=k", "invalid_url"],
      ["url=https%3A%2F%2Fuser%3Apass%40a.com&key=k", "invalid_url"],
      ["url=https%3A%2F%2Fa.com%3Fx%3D1&key=k", "invalid_url"],
      ["url=https%3A%2F%2Fa.com", "missing_key"],
      ["url=https%3A%2F%2Fa.com&key=a%0Ab", "invalid_key"],
      ["url=https%3A%2F%2Fa.com&key=k&protocol=openai_images", "invalid_protocol"],
      ["url=https%3A%2F%2Fa.com&key=k&name=a%0Ab", "invalid_name"],
      ["url=%E0%A4%A&key=k", "invalid_url"],
    ];
    for (const [fragment, error] of cases) {
      const result = parseProviderImportLink(fragment);
      assert.equal(result.ok, false, fragment);
      if (!result.ok) assert.equal(result.error, error, fragment);
    }
  });

  it("rejects oversized fragments outright", () => {
    const result = parseProviderImportLink(`url=https%3A%2F%2Fa.com&key=${"k".repeat(5000)}`);
    assert.equal(result.ok, false);
  });
});

describe("maskProviderApiKey", () => {
  it("never reveals short keys", () => {
    assert.equal(maskProviderApiKey(""), "");
    assert.equal(maskProviderApiKey("abc"), "••••");
    assert.equal(maskProviderApiKey("sk-short"), "••••••rt");
    assert.equal(maskProviderApiKey("sk-xxxxxxxx"), "••••••••xx");
    assert.equal(maskProviderApiKey("sk-abcdefghijklmnop1234"), "sk-••••••••1234");
  });
});

describe("formatProviderImportLink", () => {
  it("round-trips through the parser", () => {
    const link = formatProviderImportLink("https://chat.example.com/", {
      baseURL: "https://api.example.com/v1",
      apiKey: "sk-a+b/c=d&e",
      protocol: "anthropic_messages",
      name: "My relay",
    });
    assert.equal(link.startsWith("https://chat.example.com/import#v=1&url=https%3A%2F%2Fapi.example.com%2Fv1&key="), true);
    const result = parseProviderImportLink(link.slice(link.indexOf("#")));
    assert.equal(result.ok, true);
    if (!result.ok) return;
    assert.deepEqual(result.link, {
      baseURL: "https://api.example.com/v1",
      apiKey: "sk-a+b/c=d&e",
      protocol: "anthropic_messages",
      name: "My relay",
      host: "api.example.com",
    });
  });

  it("trims whitespace and any number of trailing slashes from the origin", () => {
    assert.equal(formatProviderImportLink("  https://chat.example.com///  "), "https://chat.example.com/import#v=1&url={address}&key={key}");
  });

  // CodeQL js/polynomial-redos: a regex like /\/+$/ is quadratic on a long run of slashes that does
  // not reach the end. The loop is linear, so even a pathological origin returns at once.
  it("stays linear on a long run of slashes", () => {
    const origin = `https://chat.example.com${"/".repeat(100_000)}x`;
    const started = performance.now();
    const link = formatProviderImportLink(origin);
    assert.equal(link.startsWith(origin), true);
    assert.equal(performance.now() - started < 1000, true);
  });

  it("omits optional parameters", () => {
    assert.equal(
      formatProviderImportLink("https://chat.example.com", { baseURL: "https://api.example.com", apiKey: "sk-x", name: " " }),
      "https://chat.example.com/import#v=1&url=https%3A%2F%2Fapi.example.com&key=sk-x",
    );
  });
});

describe("formatProviderImportLink without a URL or key", () => {
  it("renders the chat-link template, which the parser accepts once substituted", () => {
    const template = formatProviderImportLink("https://chat.example.com/");
    assert.equal(template, "https://chat.example.com/import#v=1&url={address}&key={key}");
    // What New API produces after substituting its placeholders.
    const rendered = template.replace("{address}", encodeURIComponent("https://api.relay.com")).replace("{key}", "sk-abc123");
    const result = parseProviderImportLink(rendered.slice(rendered.indexOf("#")));
    assert.equal(result.ok, true);
  });
});
