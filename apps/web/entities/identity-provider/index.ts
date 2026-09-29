// Public entry of the identity-provider entity; code outside entities/identity-provider/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export { IdentityProviderIcon } from "@/entities/identity-provider/components/identity-provider-icon";
export {
  resolveIdentityProviderIconKey,
  resolveIdentityProviderIconURL,
  resolveSafeIdentityProviderIconURL,
} from "@/entities/identity-provider/lib/identity-provider-icons";
