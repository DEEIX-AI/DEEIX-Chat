import * as React from "react";

import type { Issue, Schema } from "./schema";
import { validate } from "./schema";
import type { UIBlockState } from "./ui-block-state";

// Parsing lives in @deeix/core (parseBlockSource); this module holds the
// registry: each catalog component's props schema, renderer and skeleton.

export type UIBlockRenderProps<P> = {
  id: string;
  props: P;
  definition: UIBlockDefinition<P>;
};

export type UIBlockSkeletonProps = { items?: number };

export type UIBlockDefinition<P = unknown> = {
  name: string;
  version: number;
  schema: Schema;
  Component: React.ComponentType<UIBlockRenderProps<P>>;
  // Skeletons get a live item count read from the streaming JSON so they can
  // reserve the component's real height row by row.
  Skeleton: React.ComponentType<UIBlockSkeletonProps>;
  // Present for sandbox-rendered components; the module-level SandboxComponent
  // reads it from the definition instead of closing over it.
  sandbox?: { source: string; title: string };
  // Readable Markdown of what the component shows with the given stored state,
  // used when a reply is copied. Without it the block is copied as source.
  toText?: (props: P, state: UIBlockState | undefined) => string;
};

export type UIBlockRenderResult = { ok: true; element: React.ReactElement } | { ok: false; issues: Issue[] };

// A definition with its props type erased. Each builtin has its own props type,
// so the registry cannot hold them under one `UIBlockDefinition<P>`; instead the
// typed component is closed over and only ever receives props its schema accepted.
export type RegisteredUIBlock = {
  name: string;
  version: number;
  Skeleton: React.ComponentType<UIBlockSkeletonProps>;
  render: (id: string, props: unknown) => UIBlockRenderResult;
  // null when the props do not validate or the component has no text form.
  toText: (props: unknown, state: UIBlockState | undefined) => string | null;
};

export function registerUIBlock<P>(definition: UIBlockDefinition<P>): RegisteredUIBlock {
  return {
    name: definition.name,
    version: definition.version,
    Skeleton: definition.Skeleton,
    render: (id, props) => {
      const validated = validate<P>(definition.schema, props);
      if (!validated.ok) {
        return validated;
      }
      return { ok: true, element: React.createElement(definition.Component, { id, props: validated.value, definition }) };
    },
    toText: (props, state) => {
      const toText = definition.toText;
      if (!toText) {
        return null;
      }
      const validated = validate<P>(definition.schema, props);
      return validated.ok ? toText(validated.value, state) : null;
    },
  };
}

export type UIBlockRegistry = Map<string, RegisteredUIBlock>;

function registryKey(name: string, version: number): string {
  return `${name}@${version}`;
}

export function createRegistry(definitions: readonly RegisteredUIBlock[]): UIBlockRegistry {
  return new Map(definitions.map((definition) => [registryKey(definition.name, definition.version), definition]));
}

export function resolveDefinition(registry: UIBlockRegistry, name: string, version: number): RegisteredUIBlock | undefined {
  return registry.get(registryKey(name, version));
}

// Components nested in a live block carry no version; the newest one is used.
export function resolveLatestDefinition(registry: UIBlockRegistry, name: string): RegisteredUIBlock | undefined {
  let latest: RegisteredUIBlock | undefined;
  for (const definition of registry.values()) {
    if (definition.name === name && (!latest || definition.version > latest.version)) {
      latest = definition;
    }
  }
  return latest;
}

// Props of catalog components shown verbatim inside live blocks: a `{` in a
// diff is content, not a formula.
export const LIVE_COMPILE_OPTIONS = {
  rawProps: (component: string) => (component === "diff" ? ["before", "after"] : undefined),
};
