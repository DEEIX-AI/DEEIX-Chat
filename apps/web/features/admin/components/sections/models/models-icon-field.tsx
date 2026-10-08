"use client";

import { Image, ImagePlus, ImageUp, Search, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
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
import type { AdminLLMModelIconAssetListItem } from "@/features/admin/api/llm-types";
import { useAdminModelsIcons } from "@/features/admin/hooks/use-admin-models-icons";
import { listLobehubIconOptions, lobehubIconURL, ModelIcon, resolveModelIconURL } from "@/entities/model";

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
  const [pickerOpen, setPickerOpen] = React.useState(false);
  const [iconQuery, setIconQuery] = React.useState("");
  const [deleteTarget, setDeleteTarget] = React.useState<AdminLLMModelIconAssetListItem | null>(null);
  const {
    uploading,
    uploadedIcons,
    uploadedIconsLoading,
    uploadedIconsLoadFailed,
    reloadUploadedIcons,
    deleting,
    uploadIcon,
    deleteIcon,
  } = useAdminModelsIcons({ pickerOpen, value, onChange, onUploadingChange });
  const previewURL = resolveModelIconURL(value);
  const normalizedQuery = iconQuery.trim().toLowerCase();
  const matchedUploadedIcons = React.useMemo(() => (uploadedIcons ?? []).filter(
    (item) => !normalizedQuery || item.publicID.toLowerCase().includes(normalizedQuery),
  ), [normalizedQuery, uploadedIcons]);
  const matchedIcons = React.useMemo(() => listLobehubIconOptions()
    .filter((item) => !normalizedQuery || item.id.includes(normalizedQuery) || item.name.toLowerCase().includes(normalizedQuery))
    .slice(0, 120), [normalizedQuery]);


  const handleFile = React.useCallback(async (file: File) => {
    await uploadIcon(file, (ref) => {
      onChange(ref);
      setPickerOpen(false);
    });
  }, [onChange, uploadIcon]);

  const handleDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    await deleteIcon(deleteTarget, () => setDeleteTarget(null));
  }, [deleteIcon, deleteTarget]);

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
        <InputGroupAddon align="inline-end" className="pr-2 pl-0 has-[>button]:mr-0">
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
                      onClick={reloadUploadedIcons}
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

                {matchedIcons.length > 0 ? (
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
                          <ModelIcon iconUrl={lobehubIconURL(item.id)} label={item.name} size={20} />
                        </button>
                      ))}
                    </div>
                  </section>
                ) : !uploadedIconsLoading && matchedUploadedIcons.length === 0 ? (
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
