import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  ALL_ENABLED,
  FEATURE_NAMES,
  disabledFeatureOf,
  resolveCapabilities,
} from "./capabilities.ts";

describe("resolveCapabilities", () => {
  it("reads every flag from a full payload", () => {
    const flags = resolveCapabilities({
      features: { ...ALL_ENABLED, multiUser: false, billingGating: false },
    });
    assert.equal(flags.multiUser, false);
    assert.equal(flags.billingGating, false);
    assert.equal(flags.usageMetering, true);
  });

  it("accepts the response envelope as well as the bare object", () => {
    const flags = resolveCapabilities({ errorMsg: "", data: { features: { announcements: false } } });
    assert.equal(flags.announcements, false);
  });

  it("falls back to enabled for anything it cannot read", () => {
    for (const input of [undefined, null, 42, "x", [], {}, { features: null }, { features: { multiUser: "no" } }]) {
      assert.deepEqual(resolveCapabilities(input), ALL_ENABLED, JSON.stringify(input));
    }
  });

  it("ignores keys it does not know and keeps the flag set complete", () => {
    const flags = resolveCapabilities({ features: { futureThing: false } });
    assert.deepEqual(Object.keys(flags).sort(), [...FEATURE_NAMES].sort());
    assert.equal("futureThing" in flags, false);
  });

  it("returns frozen objects", () => {
    assert.ok(Object.isFrozen(resolveCapabilities({})));
    assert.ok(Object.isFrozen(ALL_ENABLED));
  });
});

describe("disabledFeatureOf", () => {
  it("names the feature from a feature.disabled error", () => {
    assert.equal(
      disabledFeatureOf({ errorCode: "feature.disabled", details: { feature: "multiUser" } }),
      "multiUser",
    );
  });

  it("returns null for other errors or unknown features", () => {
    assert.equal(disabledFeatureOf({ errorCode: "resource.not_found" }), null);
    assert.equal(disabledFeatureOf({ errorCode: "feature.disabled", details: { feature: "nope" } }), null);
    assert.equal(disabledFeatureOf({ errorCode: "feature.disabled" }), null);
    assert.equal(disabledFeatureOf(new Error("x")), null);
    assert.equal(disabledFeatureOf(null), null);
  });
});
