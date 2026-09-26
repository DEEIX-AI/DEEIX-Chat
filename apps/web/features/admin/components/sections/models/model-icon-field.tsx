"use client";

import { Image, ImagePlus, ImageUp, Search, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import {
  deleteAdminLLMModelIcon,
  listAdminLLMModelIcons,
  uploadAdminLLMModelIcon,
} from "@/features/admin/api";
import type { AdminLLMModelIconAssetListItem } from "@/features/admin/api/llm.types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { ApiError } from "@/shared/api/http-client";
import { ModelIcon } from "@/shared/components/model-icon";
import { resolveModelIconURL } from "@/shared/lib/model-identity";

const MAX_ICON_BYTES = 1 << 20;
const ALLOWED_ICON_TYPES = new Set(["image/png", "image/jpeg", "image/webp"]);

type LobeHubIconOption = { id: string; name: string; src: string };

type ModelIconFieldProps = {
  id: string;
  value: string;
  placeholder?: string;
  help?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  onUploadingChange?: (uploading: boolean) => void;
};

export function ModelIconField({
  id,
  value,
  placeholder,
  help,
  disabled = false,
  onChange,
  onUploadingChange,
}: ModelIconFieldProps) {
  const t = useTranslations("adminModels.iconAsset");
  const commonT = useTranslations("common");
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const mountedRef = React.useRef(true);
  const [uploading, setUploading] = React.useState(false);
  const [pickerOpen, setPickerOpen] = React.useState(false);
  const [iconQuery, setIconQuery] = React.useState("");
  const [lobehubIcons, setLobehubIcons] = React.useState<LobeHubIconOption[] | null>(null);
  const [lobehubLoadFailed, setLobehubLoadFailed] = React.useState(false);
  const [uploadedIcons, setUploadedIcons] = React.useState<AdminLLMModelIconAssetListItem[] | null>(null);
  const [uploadedIconsLoading, setUploadedIconsLoading] = React.useState(false);
  const [uploadedIconsLoadFailed, setUploadedIconsLoadFailed] = React.useState(false);
  const [uploadedIconsReloadKey, setUploadedIconsReloadKey] = React.useState(0);
  const [deleteTarget, setDeleteTarget] = React.useState<AdminLLMModelIconAssetListItem | null>(null);
  const [deleting, setDeleting] = React.useState(false);
  const previewURL = resolveModelIconURL(value);
  const normalizedQuery = iconQuery.trim().toLowerCase();
  const matchedUploadedIcons = React.useMemo(() => (uploadedIcons ?? []).filter(
    (item) => !normalizedQuery || item.publicID.toLowerCase().includes(normalizedQuery),
  ), [normalizedQuery, uploadedIcons]);
  const matchedIcons = React.useMemo(() => (lobehubIcons ?? [])
    .filter((item) => !normalizedQuery || item.id.includes(normalizedQuery) || item.name.toLowerCase().includes(normalizedQuery))
    .slice(0, 120), [lobehubIcons, normalizedQuery]);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      onUploadingChange?.(false);
    };
  }, [onUploadingChange]);

  React.useEffect(() => {
    if (!pickerOpen || lobehubIcons !== null || lobehubLoadFailed) {
      return;
    }
    let canceled = false;
    void import("@/shared/generated/lobehub-icon-manifest")
      .then(({ lobehubIconManifest }) => {
        if (!canceled) {
          setLobehubIcons(lobehubIconManifest.filter(
            (item) => !/-(?:brand|brand-color|color|text|text-cn)$/u.test(item.id),
          ));
        }
      })
      .catch(() => {
        if (!canceled) {
          setLobehubIcons([]);
          setLobehubLoadFailed(true);
        }
      });
    return () => {
      canceled = true;
    };
  }, [lobehubIcons, lobehubLoadFailed, pickerOpen]);

  React.useEffect(() => {
    if (!pickerOpen) {
      return;
    }
    let canceled = false;
    setUploadedIcons(null);
    setUploadedIconsLoadFailed(false);
    setUploadedIconsLoading(true);
    void resolveAccessToken()
      .then(async (token) => {
        if (!token) {
          throw new Error("session expired");
        }
        return listAdminLLMModelIcons(token, { page: 1, pageSize: 100 });
      })
      .then((page) => {
        if (!canceled) {
          setUploadedIcons(page.results);
          setUploadedIconsLoadFailed(false);
        }
      })
      .catch(() => {
        if (!canceled) {
          setUploadedIcons([]);
          setUploadedIconsLoadFailed(true);
        }
      })
      .finally(() => {
        if (!canceled) {
          setUploadedIconsLoading(false);
        }
      });
    return () => {
      canceled = true;
    };
  }, [pickerOpen, uploadedIconsReloadKey]);

  const setUploadState = React.useCallback((next: boolean) => {
    if (!mountedRef.current) {
      return;
    }
    setUploading(next);
    onUploadingChange?.(next);
  }, [onUploadingChange]);

  const handleFile = React.useCallback(async (file: File) => {
    if (file.type && !ALLOWED_ICON_TYPES.has(file.type)) {
      toast.error(t("invalidType"));
      return;
    }
    if (file.size <= 0 || file.size > MAX_ICON_BYTES) {
      toast.error(t("fileTooLarge"));
      return;
    }

    setUploadState(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      const asset = await uploadAdminLLMModelIcon(token, file);
      if (mountedRef.current) {
        onChange(asset.ref);
        setPickerOpen(false);
        toast.success(asset.reused ? t("reused") : t("uploaded"));
      }
    } catch (error) {
      if (mountedRef.current) {
        toast.error(t("uploadFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      setUploadState(false);
    }
  }, [onChange, setUploadState, t]);

  const handleDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"));
        return;
      }
      await deleteAdminLLMModelIcon(token, deleteTarget.publicID);
      if (mountedRef.current) {
        setUploadedIcons((current) => current?.filter((item) => item.publicID !== deleteTarget.publicID) ?? current);
        if (value.trim().toLowerCase() === deleteTarget.ref.toLowerCase()) {
          onChange("");
        }
        setDeleteTarget(null);
        toast.success(t("removed"));
      }
    } catch (error) {
      if (mountedRef.current) {
        const referenceCount = error instanceof ApiError && error.errorCode === "llm.model_icon_asset_in_use"
          && typeof error.details === "object" && error.details !== null && "referenceCount" in error.details
          && typeof error.details.referenceCount === "number"
          ? error.details.referenceCount
          : 0;
        if (referenceCount > 0) {
          toast.error(t("removeInUse", { count: referenceCount }));
        } else {
          toast.error(t("removeFailed"), { description: resolveAdminErrorMessage(error) });
        }
      }
    } finally {
      if (mountedRef.current) {
        setDeleting(false);
      }
    }
  }, [deleteTarget, onChange, t, value]);

  const helpID = help ? `${id}-help` : undefined;

  return (
    <div className="min-w-0">
      <input
        ref={fileInputRef}
        type="file"
        className="hidden"
        accept=".png,.jpg,.jpeg,.webp,image/png,image/jpeg,image/webp"
        disabled={disabled || uploading}
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = "";
          if (file) {
            void handleFile(file);
          }
        }}
      />
      <InputGroup className="h-9 bg-background">
        <InputGroupAddon align="inline-start" className="pr-0 pl-2">
          <span className="flex size-6 shrink-0 items-center justify-center rounded-sm bg-muted/70 text-muted-foreground">
            {previewURL ? (
              <ModelIcon key={previewURL} iconUrl={previewURL} label={value} size={16} />
            ) : (
              <Image className="size-3.5 stroke-1.5" />
            )}
          </span>
        </InputGroupAddon>
        <InputGroupInput
          id={id}
          value={value}
          disabled={disabled || uploading}
          placeholder={placeholder}
          aria-describedby={helpID}
          className="h-9"
          onChange={(event) => onChange(event.target.value)}
        />
        <InputGroupAddon align="inline-end" className="pr-1 pl-0 has-[>button]:mr-0">
          <Popover
            modal
            open={pickerOpen}
            onOpenChange={setPickerOpen}
          >
            <PopoverTrigger asChild>
              <InputGroupButton
                size="icon-xs"
                disabled={disabled || uploading}
                aria-label={t("choose")}
                title={t("choose")}
                className="bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                {uploading ? <Spinner className="size-3.5" /> : <ImagePlus className="size-3.5 stroke-1.5" />}
              </InputGroupButton>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-80 p-2">
              <div className="mb-2 flex items-center gap-2">
                <div className="relative min-w-0 flex-1">
                  <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={iconQuery}
                    className="h-8 pl-8 text-xs"
                    placeholder={t("searchIcons")}
                    onChange={(event) => setIconQuery(event.target.value)}
                  />
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-8 shrink-0 bg-muted/60 px-2.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                  disabled={uploading}
                  onClick={() => fileInputRef.current?.click()}
                >
                  {uploading ? <Spinner className="size-3.5" /> : <ImageUp className="size-3.5 stroke-1.5" />}
                  {t("uploadShort")}
                </Button>
              </div>
              <div
                className="max-h-72 touch-pan-y overflow-y-auto overscroll-contain pr-1"
                onWheel={(event) => event.stopPropagation()}
              >
                {uploadedIconsLoading ? (
                  <div className="flex h-12 items-center justify-center text-muted-foreground">
                    <Spinner className="size-3.5" />
                  </div>
                ) : uploadedIconsLoadFailed ? (
                  <div className="flex h-12 items-center justify-center gap-1 text-xs text-muted-foreground">
                    <span>{t("loadUploadedFailed")}</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => setUploadedIconsReloadKey((current) => current + 1)}
                    >
                      {commonT("actions.retry")}
                    </Button>
                  </div>
                ) : matchedUploadedIcons.length > 0 ? (
                  <section className="pb-2">
                    <p className="px-1 pb-1.5 text-[11px] font-medium text-muted-foreground">{t("uploadedIcons")}</p>
                    <div className="grid grid-cols-6 gap-1">
                      {matchedUploadedIcons.map((item) => {
                        const iconURL = resolveModelIconURL(item.ref);
                        return (
                          <div key={item.publicID} className="group relative aspect-square">
                            <button
                              type="button"
                              className={`flex size-full items-center justify-center rounded-md transition-colors hover:bg-muted ${
                                value.trim().toLowerCase() === item.ref.toLowerCase()
                                  ? "bg-muted"
                                  : "bg-transparent"
                              }`}
                              title={t("uploadedIconTitle")}
                              aria-label={t("uploadedIconTitle")}
                              onClick={() => {
                                onChange(item.ref);
                                setPickerOpen(false);
                              }}
                            >
                              {iconURL ? <ModelIcon iconUrl={iconURL} label={t("uploadedIconTitle")} size={20} /> : null}
                            </button>
                            <button
                              type="button"
                              className="absolute top-1 right-1 flex size-4 items-center justify-center rounded-full bg-destructive/90 text-destructive-foreground opacity-70 transition-[opacity,transform] hover:scale-105 hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring group-hover:opacity-100 sm:opacity-0"
                              title={t("remove")}
                              aria-label={t("remove")}
                              onClick={() => {
                                setPickerOpen(false);
                                setDeleteTarget(item);
                              }}
                            >
                              <Trash2 className="size-2.5 stroke-2" />
                            </button>
                          </div>
                        );
                      })}
                    </div>
                  </section>
                ) : null}

                {lobehubLoadFailed ? (
                  <div className="flex h-20 flex-col items-center justify-center gap-2 text-xs text-muted-foreground">
                    <span>{t("loadIconLibraryFailed")}</span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => {
                        setLobehubLoadFailed(false);
                        setLobehubIcons(null);
                      }}
                    >
                      {t("retryIconLibrary")}
                    </Button>
                  </div>
                ) : lobehubIcons === null && !uploadedIconsLoading ? (
                  <div className="flex h-20 items-center justify-center text-muted-foreground">
                    <Spinner className="size-4" />
                  </div>
                ) : matchedIcons.length > 0 ? (
                  <section>
                    <p className="px-1 pb-1.5 text-[11px] font-medium text-muted-foreground">{t("builtInIcons")}</p>
                    <div className="grid grid-cols-6 gap-1">
                      {matchedIcons.map((item) => (
                        <button
                          key={item.id}
                          type="button"
                          className={`flex aspect-square items-center justify-center rounded-md transition-colors hover:bg-muted ${
                            value.trim().toLowerCase() === item.id ? "bg-muted" : "bg-transparent"
                          }`}
                          title={`${item.name} (${item.id})`}
                          aria-label={`${item.name} (${item.id})`}
                          onClick={() => {
                            onChange(item.id);
                            setPickerOpen(false);
                          }}
                        >
                          <ModelIcon iconUrl={item.src} label={item.name} size={20} />
                        </button>
                      ))}
                    </div>
                  </section>
                ) : lobehubIcons !== null && !uploadedIconsLoading && matchedUploadedIcons.length === 0 ? (
                  <div className="flex h-20 items-center justify-center text-xs text-muted-foreground">{t("noIconResults")}</div>
                ) : null}
                {matchedIcons.length === 120 ? (
                  <p className="pt-2 text-center text-[11px] text-muted-foreground">{t("refineIconSearch")}</p>
                ) : null}
              </div>
              {help ? <p className="mt-2 border-t border-border/60 px-1 pt-2 text-[11px] leading-4 text-muted-foreground">{help}</p> : null}
            </PopoverContent>
          </Popover>
        </InputGroupAddon>
      </InputGroup>
      {help ? <span id={helpID} className="sr-only">{help}</span> : null}
      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => !open && !deleting && setDeleteTarget(null)}>
        <AlertDialogContent size="compact">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("removeTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("removeDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleting}
              onClick={(event) => {
                event.preventDefault();
                void handleDelete();
              }}
            >
              {deleting ? (
                <>
                  <Spinner className="size-3.5" />
                  {t("removing")}
                </>
              ) : t("confirmRemove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
