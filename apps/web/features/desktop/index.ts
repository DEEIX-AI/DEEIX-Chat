// Public entry of the desktop feature. The (app) layout mounts the bootstrap and
// the update and download notifiers; settings shows the Network tab and, on the
// About page, the distribution details; the (shell) route group mounts the shell
// providers and tab strip.
// Unused re-exports are dropped at build time because package.json declares
// `sideEffects`, so each route bundles only what it imports.
export { DesktopBootstrap } from "@/features/desktop/components/desktop-bootstrap";
export { DesktopDownloadNotifier } from "@/features/desktop/components/desktop-download-notifier";
export { DesktopDistributionDetails } from "@/features/desktop/components/desktop-distribution-details";
export { DesktopNetworkSettings } from "@/features/desktop/components/desktop-network-settings";
export { useDesktopNetworkAvailable } from "@/features/desktop/hooks/use-desktop-network-available";
export { DesktopUpdateNotifier } from "@/features/desktop/components/desktop-update-notifier";
export { ShellProviders } from "@/features/desktop/components/shell-providers";
export { TabStrip } from "@/features/desktop/components/tab-strip";
