"use client";

import * as React from "react";
import { LoaderCircle, ShieldCheck, UsersRound } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from "@/components/ui/combobox";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { listAdminUsers, type PermissionGroup } from "@/features/admin/api";
import type { AdminStatisticsSubject } from "@/features/admin/hooks/use-admin-statistics";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { cn } from "@/lib/utils";
import type { AdminUserDTO } from "@/features/admin/api/admin.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";

type SubjectMode = "user" | "permission-group";

type SubjectFilterOption =
  | { key: "all"; type: "all" }
  | { key: string; type: "user"; user: AdminUserDTO }
  | { key: string; type: "permission-group"; permissionGroup: PermissionGroup };

const ALL_SUBJECT_OPTION: SubjectFilterOption = { key: "all", type: "all" };

function userPrimaryLabel(user: AdminUserDTO): string {
  return user.displayName.trim() || user.username.trim() || `#${user.id}`;
}

function userSecondaryLabel(user: AdminUserDTO): string {
  const values = [user.username.trim(), user.email.trim(), `#${user.id}`].filter(Boolean);
  return values.join(" · ");
}

function subjectLabel(subject: AdminStatisticsSubject, allUsersLabel: string): string {
  if (subject.type === "user") return userPrimaryLabel(subject.user);
  if (subject.type === "permission-group") return subject.permissionGroup.name;
  return allUsersLabel;
}

export function AdminStatisticsSubjectFilter({
  value,
  permissionGroups,
  onChange,
  disabled = false,
  label,
  triggerClassName,
}: {
  value: AdminStatisticsSubject;
  permissionGroups: PermissionGroup[];
  onChange: (subject: AdminStatisticsSubject) => void;
  disabled?: boolean;
  label: string;
  triggerClassName?: string;
}) {
  const t = useTranslations("adminStatistics.filters.subjectSelect");
  const [open, setOpen] = React.useState(false);
  const [mode, setMode] = React.useState<SubjectMode>("user");
  const [query, setQuery] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [users, setUsers] = React.useState<AdminUserDTO[]>([]);
  const [loading, setLoading] = React.useState(false);
  const requestSequenceRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  React.useEffect(() => {
    if (!open || mode !== "user") return;
    const requestSequence = requestSequenceRef.current + 1;
    requestSequenceRef.current = requestSequence;
    setLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const result = await listAdminUsers(token, {
          page: 1,
          pageSize: 20,
          query: debouncedQuery,
        });
        if (requestSequence === requestSequenceRef.current) {
          setUsers(result.results);
        }
      } catch (error) {
        if (requestSequence === requestSequenceRef.current) {
          toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
        }
      } finally {
        if (requestSequence === requestSequenceRef.current) setLoading(false);
      }
    })();
  }, [debouncedQuery, mode, open, t]);

  const options = React.useMemo<SubjectFilterOption[]>(() => {
    if (mode === "permission-group") {
      const normalizedQuery = query.trim().toLocaleLowerCase();
      const groupOptions = permissionGroups
        .filter((group) => {
          if (!normalizedQuery) return true;
          return `${group.name} ${group.description}`.toLocaleLowerCase().includes(normalizedQuery);
        })
        .map((permissionGroup) => ({
          key: `permission-group:${permissionGroup.id}`,
          type: "permission-group" as const,
          permissionGroup,
        }));
      return normalizedQuery ? groupOptions : [ALL_SUBJECT_OPTION, ...groupOptions];
    }

    const userOptions: SubjectFilterOption[] = users.map((user) => ({
      key: `user:${user.id}`,
      type: "user",
      user,
    }));
    if (query.trim()) return userOptions;
    if (value.type === "user" && !userOptions.some((option) => option.type === "user" && option.user.id === value.user.id)) {
      userOptions.unshift({ key: `user:${value.user.id}`, type: "user", user: value.user });
    }
    return [ALL_SUBJECT_OPTION, ...userOptions];
  }, [mode, permissionGroups, query, users, value]);

  const selectedOption: SubjectFilterOption = value.type === "user"
    ? { key: `user:${value.user.id}`, type: "user", user: value.user }
    : value.type === "permission-group"
      ? {
          key: `permission-group:${value.permissionGroup.id}`,
          type: "permission-group",
          permissionGroup: value.permissionGroup,
        }
      : ALL_SUBJECT_OPTION;

  return (
    <Combobox
      items={options}
      value={selectedOption}
      inputValue={query}
      open={open}
      filter={null}
      autoComplete="none"
      disabled={disabled}
      itemToStringLabel={(option) => {
        if (option.type === "user") return userSecondaryLabel(option.user);
        if (option.type === "permission-group") return option.permissionGroup.name;
        return t("allUsers");
      }}
      isItemEqualToValue={(option, selected) => option.key === selected.key}
      onInputValueChange={setQuery}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen);
        if (nextOpen) {
          setMode(value.type === "permission-group" ? "permission-group" : "user");
        } else {
          setQuery("");
        }
      }}
      onValueChange={(option) => {
        if (!option || option.type === "all") {
          onChange({ type: "all" });
        } else if (option.type === "user") {
          onChange({ type: "user", user: option.user });
        } else {
          onChange({ type: "permission-group", permissionGroup: option.permissionGroup });
        }
        setQuery("");
        setOpen(false);
      }}
    >
      <ComboboxTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            disabled={disabled}
            className={cn(
              "h-8 w-full min-w-0 justify-start gap-2 rounded-md px-2 text-xs font-normal shadow-none data-[popup-open]:bg-accent/50 [&_[data-slot=combobox-trigger-icon]]:ml-0 [&_[data-slot=combobox-trigger-icon]]:size-3.5 [&_[data-slot=combobox-trigger-icon]]:-rotate-90",
              triggerClassName,
            )}
          >
            <UsersRound className="size-3.5 shrink-0 text-muted-foreground" />
            <span>{label}</span>
            <span className="ml-auto max-w-32 truncate text-[11px] text-muted-foreground">
              {subjectLabel(value, t("allUsers"))}
            </span>
          </Button>
        }
      />
      <ComboboxContent side="right" align="start" sideOffset={8} className="w-[min(300px,calc(100vw-32px))]">
        <Tabs
          value={mode}
          className="gap-0 px-1 pt-1"
          onValueChange={(nextMode) => {
            setMode(nextMode as SubjectMode);
            setQuery("");
          }}
        >
          <TabsList className="w-full">
            <TabsTrigger value="user">{t("users")}</TabsTrigger>
            <TabsTrigger value="permission-group">{t("permissionGroups")}</TabsTrigger>
          </TabsList>
        </Tabs>
        <ComboboxInput
          placeholder={mode === "user" ? t("searchUsers") : t("searchPermissionGroups")}
          showTrigger={false}
          showClear={false}
          disabled={disabled}
        >
          {mode === "user" && loading ? (
            <LoaderCircle className="pointer-events-none absolute right-3 size-3.5 animate-spin text-muted-foreground" />
          ) : null}
        </ComboboxInput>
        <ComboboxEmpty>{mode === "user" ? t("emptyUsers") : t("emptyPermissionGroups")}</ComboboxEmpty>
        <ComboboxList>
          {(option: SubjectFilterOption) => {
            if (option.type === "user") {
              return (
                <ComboboxItem
                  key={option.key}
                  value={option}
                  className="group/subject h-8 py-0 pr-14"
                  title={userSecondaryLabel(option.user)}
                >
                  <Avatar className="size-5">
                    <AvatarImage src={resolveAvatarImageSrc(option.user.avatarURL, option.user) || undefined} alt="" />
                    <AvatarFallback className="text-[9px] font-medium">
                      {userPrimaryLabel(option.user).charAt(0).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <span className="flex min-w-0 flex-1 items-baseline gap-1">
                    <span className="max-w-[45%] shrink-0 truncate font-medium">{userPrimaryLabel(option.user)}</span>
                    {option.user.username.trim() ? (
                      <span className="min-w-0 truncate text-[10px] text-muted-foreground">(@{option.user.username.trim()})</span>
                    ) : null}
                  </span>
                  <span className="absolute right-2 w-10 text-right font-mono text-[10px] tabular-nums text-muted-foreground group-data-[selected]/subject:hidden">
                    #{option.user.id}
                  </span>
                </ComboboxItem>
              );
            }
            if (option.type === "permission-group") {
              return (
                <ComboboxItem key={option.key} value={option} className="group/subject h-8 py-0 pr-16">
                  <Avatar className="size-5">
                    <AvatarFallback>
                      <ShieldCheck className="size-3 text-foreground" />
                    </AvatarFallback>
                  </Avatar>
                  <span className="min-w-0 flex-1 truncate font-medium">{option.permissionGroup.name}</span>
                  <span className="absolute right-2 text-[10px] tabular-nums text-muted-foreground group-data-[selected]/subject:hidden">
                    {t("memberCount", { count: option.permissionGroup.userCount })}
                  </span>
                </ComboboxItem>
              );
            }
            return (
              <ComboboxItem key={option.key} value={option} className="h-8 py-0">
                <Avatar className="size-5">
                  <AvatarFallback>
                    <UsersRound className="size-3 text-foreground" />
                  </AvatarFallback>
                </Avatar>
                <span className="min-w-0 flex-1 truncate font-medium">{t("allUsers")}</span>
              </ComboboxItem>
            );
          }}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}
