"use client";

import * as React from "react";

import type { UIComponentDTO } from "@/shared/api/ui-components.types";
import { createRegistry, type UIBlockDefinition, type UIBlockRegistry } from "./block";
import { calculatorDefinition } from "./calculator";
import { cardGridDefinition } from "./card-grid";
import { chartDefinition } from "./chart";
import { dataTableDefinition } from "./data-table";
import { decisionTreeDefinition } from "./decision-tree";
import { diffDefinition } from "./diff";
import { functionPlotDefinition } from "./function-plot";
import { ganttDefinition } from "./gantt";
import { quizDefinition } from "./quiz";
import { schemaFromJSONSchema } from "./json-schema";
import { SandboxComponent } from "./sandbox-host";
import { s } from "./schema";
import { statGridDefinition } from "./stat-grid";
import { UIBlockFrame } from "./ui-block-frame";

// Builtin catalog. internal/domain/uicomponent.Builtin() seeds the same
// name@version pairs into ui_components; the backend owns prompt text, this
// side owns validation and rendering.
export const BUILTIN_UI_BLOCKS = [
  cardGridDefinition,
  statGridDefinition,
  chartDefinition,
  functionPlotDefinition,
  dataTableDefinition,
  ganttDefinition,
  diffDefinition,
  calculatorDefinition,
  decisionTreeDefinition,
  quizDefinition,
] as unknown as readonly UIBlockDefinition[];

export const builtinUIBlockRegistry = createRegistry(BUILTIN_UI_BLOCKS);

const UIBlockRegistryContext = React.createContext<UIBlockRegistry>(builtinUIBlockRegistry);

export function useUIBlockRegistry(): UIBlockRegistry {
  return React.useContext(UIBlockRegistryContext);
}

// Builds a registry from the visible catalog: builtin rows dispatch to the
// React implementations, sandbox rows wrap their HTML source in an iframe.
export function createCatalogRegistry(components: readonly UIComponentDTO[]): UIBlockRegistry {
  const definitions: UIBlockDefinition[] = [];
  for (const component of components) {
    if (!component.enabled) {
      continue;
    }
    if (component.rendererKind === "builtin") {
      const builtin = BUILTIN_UI_BLOCKS.find((definition) => definition.name === component.name && definition.version === component.version);
      if (builtin) {
        definitions.push(builtin);
      }
      continue;
    }
    definitions.push(sandboxDefinition(component));
  }
  return createRegistry(definitions);
}

function SandboxSkeleton() {
  return <UIBlockFrame className="h-24 animate-pulse bg-muted/30" />;
}

function sandboxDefinition(component: UIComponentDTO): UIBlockDefinition {
  return {
    name: component.name,
    version: component.version,
    schema: schemaFromJSONSchema(component.propsSchema) ?? s.any(),
    Component: SandboxComponent,
    Skeleton: SandboxSkeleton,
    sandbox: { source: component.rendererSource, title: component.description || component.name },
  };
}

export function UIBlockRegistryProvider({ components, children }: { components: readonly UIComponentDTO[] | null; children: React.ReactNode }) {
  const registry = React.useMemo(() => (components ? createCatalogRegistry(components) : builtinUIBlockRegistry), [components]);
  return <UIBlockRegistryContext.Provider value={registry}>{children}</UIBlockRegistryContext.Provider>;
}
