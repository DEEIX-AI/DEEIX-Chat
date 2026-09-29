"use client";

import {
  Check,
  ChevronDown,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarTransitionContent,
  useSidebarHoverExpansionLock,
} from "@/components/ui/sidebar";
import { SpinnerLabel } from "@/components/ui/spinner";
import { NavDesktopDownload } from "@/features/shell/components/navigation/nav-desktop-download";
import { useShellUserMenuActions } from "@/features/shell/hooks/use-shell-user-menu-actions";
import { APP_LOCALE_LABELS, APP_LOCALES } from "@/i18n/config";
import { dispatchOpenAnnouncements, getAnnouncementUnread, subscribeAnnouncementUnreadChanged } from "@/entities/announcement";
import { FeatureGate } from "@/shared/capabilities";

export function NavUser({
  user,
}: {
  user: {
    name: string;
    email: string;
    avatar: string;
    role?: string;
  };
}) {
  const t = useTranslations("common.navigation");
  const router = useRouter();
  const { locale, loggingOut, savingLocale, onLogout, onLocaleSelect } = useShellUserMenuActions();
  const [open, setOpen] = React.useState(false);
  const [hasUnreadAnnouncement, setHasUnreadAnnouncement] = React.useState(() => getAnnouncementUnread());
  const skipTriggerFocusRef = React.useRef(false);
  const isAdmin = user.role === "admin" || user.role === "superadmin";

  useSidebarHoverExpansionLock(open);

  React.useEffect(() => subscribeAnnouncementUnreadChanged(setHasUnreadAnnouncement), []);

  const navigateFromMenu = React.useCallback(
    (href: string) => (event: Event) => {
      event.preventDefault();
      skipTriggerFocusRef.current = true;
      setOpen(false);
      router.push(href);
    },
    [router],
  );

  const closeAfterExternalOpen = React.useCallback(() => {
    skipTriggerFocusRef.current = true;
    setOpen(false);
  }, []);

  const openAnnouncementsFromMenu = React.useCallback((event: Event) => {
    event.preventDefault();
    skipTriggerFocusRef.current = true;
    setOpen(false);
    dispatchOpenAnnouncements();
  }, []);

  return (
    <SidebarMenu className="group-data-[collapsible=icon]:items-center">
      <SidebarMenuItem className="group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:w-full group-data-[collapsible=icon]:justify-center">
        <DropdownMenu open={open} onOpenChange={setOpen}>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              id="sidebar-user-menu-trigger"
              type="button"
              size="lg"
              className="mb-1 overflow-visible pr-2 pl-2.5 transition-[background-color,color,height,padding,margin] data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:mb-0 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0"
              aria-label={user.name}
            >
              <Avatar className="size-7 shrink-0 rounded-full">
                <AvatarImage src={user.avatar || undefined} alt={user.name} />
                <AvatarFallback className="rounded-full bg-foreground text-xs font-medium text-background">{user.name.charAt(0).toUpperCase()}</AvatarFallback>
              </Avatar>
              {/* opacity, not visibility: WebKit misprints a `truncate` box after visibility:hidden → visible. */}
              <div className="grid min-w-0 flex-1 gap-0.5 overflow-hidden pl-1.5 text-left text-sm leading-tight group-data-[collapsible=icon]:hidden group-data-[resizing=true]:opacity-0">
                <span className="truncate font-medium text-foreground/95">{user.name}</span>
              </div>
              <SidebarTransitionContent asChild>
                <ChevronDown aria-hidden className="ml-auto size-4 stroke-1 group-data-[collapsible=icon]:hidden" />
              </SidebarTransitionContent>
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
            side="top"
            align="start"
            sideOffset={4}
            onCloseAutoFocus={(event) => {
              if (!skipTriggerFocusRef.current) {
                return;
              }
              event.preventDefault();
              skipTriggerFocusRef.current = false;
              requestAnimationFrame(() => {
                document.getElementById("sidebar-user-menu-trigger")?.blur();
              });
            }}
          >
            <DropdownMenuLabel className="px-2 py-2 font-normal text-muted-foreground">
              <span className="block truncate" title={user.email}>
                {user.email}
              </span>
            </DropdownMenuLabel>
            <DropdownMenuGroup>
              <DropdownMenuItem onSelect={navigateFromMenu("/settings/general")}>
                {t("settings")}
              </DropdownMenuItem>
              <FeatureGate feature="announcements">
                <DropdownMenuItem onSelect={openAnnouncementsFromMenu}>
                  <span className="min-w-0 flex-1 truncate">{t("announcements")}</span>
                  <span className="ml-auto flex size-4 shrink-0 items-center justify-center">
                    {hasUnreadAnnouncement ? <span aria-hidden="true" className="size-1.5 rounded-full bg-destructive" /> : null}
                  </span>
                </DropdownMenuItem>
              </FeatureGate>
              <DropdownMenuSub>
                <DropdownMenuSubTrigger className="focus:bg-accent/40 data-[state=open]:bg-accent/40">
                  {t("language")}
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="min-w-32 p-1.5">
                  {APP_LOCALES.map((item) => (
                    <DropdownMenuItem
                      key={item}
                      disabled={savingLocale === item}
                      onSelect={(event) => {
                        event.preventDefault();
                        void onLocaleSelect(item);
                      }}
                    >
                      {APP_LOCALE_LABELS[item]}
                      {locale === item ? <DropdownMenuItemIcon icon={Check} className="ml-auto" /> : null}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuGroup>
            <FeatureGate feature="billingGating">
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem onSelect={navigateFromMenu("/settings/subscription")}>
                  {t("upgradePlan")}
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </FeatureGate>
            {/* Outside the billing gate: servers without billing still offer the download. */}
            <NavDesktopDownload onOpenPage={closeAfterExternalOpen} />
            <DropdownMenuSeparator />
            {isAdmin ? (
              <DropdownMenuItem onSelect={navigateFromMenu("/admin")}>
                {t("admin")}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault();
                void onLogout();
              }}
              disabled={loggingOut}
            >
              {loggingOut ? <SpinnerLabel>{t("loggingOut")}</SpinnerLabel> : t("logout")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
