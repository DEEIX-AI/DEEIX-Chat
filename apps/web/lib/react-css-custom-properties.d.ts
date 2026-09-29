// Lets inline `style` objects set CSS custom properties (`{ "--sidebar-width": "16rem" }`)
// without asserting the whole object to React.CSSProperties, which would also hide typos
// in the regular properties next to them.
import type {} from "react";

declare module "react" {
  // biome-ignore lint/style/useConsistentTypeDefinitions: augmenting React.CSSProperties requires interface declaration merging.
  interface CSSProperties {
    [customProperty: `--${string}`]: string | number | undefined;
  }
}
