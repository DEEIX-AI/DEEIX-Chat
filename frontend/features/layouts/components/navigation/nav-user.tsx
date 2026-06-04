"use client"

import * as React from "react"
import { useRouter } from "next/navigation"
import { useTranslations } from "next-intl"
import {
  Check,
  ChevronDown,
} from "lucide-react"

import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@/components/ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItemIcon,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { SpinnerLabel } from "@/components/ui/spinner"
import { logout, patchMe } from "@/shared/api/auth"
import { useAuthSession } from "@/shared/auth/auth-session-context"
import { clearSessionAndRedirectToLogin } from "@/shared/auth/session"
import { dispatchUserProfileUpdated } from "@/shared/auth/user-profile-events"
import { useAppLocale } from "@/i18n/app-i18n-provider"
import { APP_LOCALE_LABELS, APP_LOCALES, type AppLocale } from "@/i18n/config"

export function NavUser({
  user,
}: {
  user: {
    name: string
    email: string
    avatar: string
    role?: string
  }
}) {
  const t = useTranslations("common.navigation")
  const { isMobile } = useSidebar()
  const { locale, setLocale } = useAppLocale()
  const router = useRouter()
  const { accessToken, user: sessionUser } = useAuthSession()
  const [open, setOpen] = React.useState(false)
  const [loggingOut, setLoggingOut] = React.useState(false)
  const [savingLocale, setSavingLocale] = React.useState<AppLocale | null>(null)
  const skipTriggerFocusRef = React.useRef(false)
  const isAdmin = user.role === "admin" || user.role === "superadmin"

  const onLogout = React.useCallback(async () => {
    if (loggingOut) {
      return
    }
    setLoggingOut(true)
    try {
      if (accessToken) {
        await logout(accessToken)
      }
    } catch {
      // Ignore logout API errors and clear local session to ensure exit.
    } finally {
      clearSessionAndRedirectToLogin()
      setLoggingOut(false)
    }
  }, [accessToken, loggingOut])

  const navigateFromMenu = React.useCallback(
    (href: string) => (event: Event) => {
      event.preventDefault()
      skipTriggerFocusRef.current = true
      setOpen(false)
      router.push(href)
    },
    [router],
  )

  const onLocaleSelect = React.useCallback(
    async (nextLocale: AppLocale) => {
      if (nextLocale === locale && !savingLocale) {
        return
      }

      if (sessionUser) {
        dispatchUserProfileUpdated({ ...sessionUser, locale: nextLocale })
      }
      void setLocale(nextLocale)

      if (!accessToken) {
        return
      }

      setSavingLocale(nextLocale)
      try {
        const nextUser = await patchMe(accessToken, { locale: nextLocale })
        dispatchUserProfileUpdated(nextUser)
      } catch {
        // Keep the local language selection; a later profile refresh may retry or restore the server value.
      } finally {
        setSavingLocale((current) => (current === nextLocale ? null : current))
      }
    },
    [accessToken, locale, savingLocale, sessionUser, setLocale],
  )

  return (
    <SidebarMenu className="group-data-[collapsible=icon]:items-center">
      <SidebarMenuItem className="group-data-[collapsible=icon]:flex group-data-[collapsible=icon]:w-full group-data-[collapsible=icon]:justify-center">
        <DropdownMenu open={open} onOpenChange={setOpen}>
          <DropdownMenuTrigger asChild>
            <button
              id="sidebar-user-menu-trigger"
              type="button"
              className="peer/menu-button mb-1 flex h-12 w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left text-sm ring-sidebar-ring outline-hidden transition-[background-color,color,width,height,padding,margin] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 active:bg-sidebar-accent active:text-sidebar-accent-foreground data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:mb-0 group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 group-data-[collapsible=icon]:overflow-visible group-data-[collapsible=icon]:p-0!"
              aria-label={user.name}
            >
              <Avatar className="size-7 shrink-0 rounded-full">
                <AvatarImage src={user.avatar || undefined} alt={user.name} />
                <AvatarFallback className="rounded-full bg-foreground text-xs font-medium text-background">{user.name.charAt(0).toUpperCase()}</AvatarFallback>
              </Avatar>
              <div className="grid min-w-0 flex-1 gap-0.5 overflow-hidden pl-1.5 text-left text-sm leading-tight transition-opacity duration-200 ease-linear group-data-[collapsible=icon]:hidden">
                <span className="truncate font-medium text-foreground/95">{user.name}</span>
                <span className="truncate text-xs text-foreground/90">{user.email}</span>
              </div>
              <ChevronDown className="ml-auto size-4 stroke-1 transition-opacity duration-200 ease-linear group-data-[collapsible=icon]:hidden" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
            side={isMobile ? "top" : "top"}
            align="end"
            sideOffset={4}
            onCloseAutoFocus={(event) => {
              if (!skipTriggerFocusRef.current) {
                return
              }
              event.preventDefault()
              skipTriggerFocusRef.current = false
              requestAnimationFrame(() => {
                document.getElementById("sidebar-user-menu-trigger")?.blur()
              })
            }}
          >
            <DropdownMenuLabel className="px-2 py-2 font-normal text-muted-foreground">
              <span className="block truncate" title={user.email}>
                {user.email}
              </span>
            </DropdownMenuLabel>
            <DropdownMenuGroup>
              <DropdownMenuItem onSelect={navigateFromMenu("/setting/general")}>
                {t("settings")}
              </DropdownMenuItem>
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
                        event.preventDefault()
                        void onLocaleSelect(item as AppLocale)
                      }}
                    >
                      {APP_LOCALE_LABELS[item]}
                      {locale === item ? <DropdownMenuItemIcon icon={Check} className="ml-auto" /> : null}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuGroup>
              <DropdownMenuItem onSelect={navigateFromMenu("/setting/subscription")}>
                {t("upgradePlan")}
              </DropdownMenuItem>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            {isAdmin ? (
              <DropdownMenuItem onSelect={navigateFromMenu("/admin")}>
                {t("admin")}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem
              onSelect={(event) => {
                event.preventDefault()
                void onLogout()
              }}
              disabled={loggingOut}
            >
              {loggingOut ? <SpinnerLabel>{t("loggingOut")}</SpinnerLabel> : t("logout")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
