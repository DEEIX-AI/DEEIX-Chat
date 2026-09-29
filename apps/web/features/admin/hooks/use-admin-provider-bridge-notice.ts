import * as React from "react";

import { getLoginOptions } from "@/shared/api/auth";
import { useCapabilities } from "@/shared/capabilities";

const DISMISSED_STORAGE_KEY = "deeix-chat:admin:provider-bridge-notice";

/**
 * Decides whether the provider-bridge warning should be shown: identity
 * providers exist but the server cannot run the OAuth handoff. Dismissal is
 * remembered for the browser session so the warning appears at most once.
 */
export function useAdminProviderBridgeNotice() {
  const { flags, loaded } = useCapabilities();
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    if (!loaded || !flags.identityProviders || window.sessionStorage.getItem(DISMISSED_STORAGE_KEY)) {
      return;
    }
    let cancelled = false;
    void getLoginOptions()
      .then((options) => {
        if (!cancelled && options.providers.length > 0 && !options.providerAuthBridge.enabled) {
          setOpen(true);
        }
      })
      .catch(() => {
        // The login settings page reports load failures itself.
      });
    return () => {
      cancelled = true;
    };
  }, [flags.identityProviders, loaded]);

  const dismiss = React.useCallback(() => {
    window.sessionStorage.setItem(DISMISSED_STORAGE_KEY, "1");
    setOpen(false);
  }, []);

  return { open, setOpen, dismiss };
}
