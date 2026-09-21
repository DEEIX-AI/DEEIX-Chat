"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogHeightTransition, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { SandboxComponent } from "@/shared/components/markdown/ui-blocks/sandbox-host";
import { UIBlockFrame } from "@/shared/components/markdown/ui-blocks/ui-block-frame";
import {
  isValidUIComponentName,
  UI_COMPONENT_LIMITS,
  type UIComponentFormValue,
  uiComponentSchemaIsValid,
} from "@/shared/model/ui-components";

type UIComponentEditorDialogProps = {
  open: boolean;
  saving: boolean;
  // Non-owner views (builtin/platform in the user library) are read-only.
  readOnly?: boolean;
  form: UIComponentFormValue;
  onOpenChange: (open: boolean) => void;
  onFormChange: React.Dispatch<React.SetStateAction<UIComponentFormValue>>;
  onSave: () => void;
};

const DEFAULT_PREVIEW_PROPS = `{\n  "title": "示例"\n}`;

function PreviewSkeleton() {
  return <UIBlockFrame className="min-h-24" />;
}

export function UIComponentEditorDialog({ open, saving, readOnly = false, form, onOpenChange, onFormChange, onSave }: UIComponentEditorDialogProps) {
  const t = useTranslations("uiComponents.editor");
  const builtin = form.scope === "builtin";
  const contractLocked = builtin || readOnly;
  const [previewPropsText, setPreviewPropsText] = React.useState(DEFAULT_PREVIEW_PROPS);
  // Preview source is applied on demand; it starts from the form and is reset
  // whenever a different component is opened (keyed by form.id below).
  const [previewSource, setPreviewSource] = React.useState(form.rendererSource);

  const previewProps = React.useMemo<{ ok: true; value: unknown } | { ok: false }>(() => {
    try {
      const parsed: unknown = JSON.parse(previewPropsText);
      return typeof parsed === "object" && parsed !== null && !Array.isArray(parsed) ? { ok: true, value: parsed } : { ok: false };
    } catch {
      return { ok: false };
    }
  }, [previewPropsText]);

  const previewDefinition = React.useMemo(
    () => ({
      name: form.name || "preview",
      version: 1,
      schema: { kind: "any" as const },
      Component: SandboxComponent,
      Skeleton: PreviewSkeleton,
      sandbox: { source: previewSource, title: form.description || form.name || "preview" },
    }),
    [form.description, form.name, previewSource],
  );

  const nameInvalid = form.name.trim() !== "" && !isValidUIComponentName(form.name);
  const schemaInvalid = !uiComponentSchemaIsValid(form.propsSchema);
  const field = (key: keyof UIComponentFormValue) => (value: string | boolean) => onFormChange((current) => ({ ...current, [key]: value }));

  return (
    <Dialog open={open} onOpenChange={(next) => !saving && onOpenChange(next)}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-[960px]">
        <DialogHeightTransition contentClassName="max-h-[min(90vh,820px)]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{form.id ? t("editTitle") : t("createTitle")}</DialogTitle>
            <DialogDescription>{readOnly ? t("readOnlyDescription") : builtin ? t("builtinDescription") : t("description")}</DialogDescription>
          </DialogHeader>

          <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 overflow-y-auto px-5 py-2 md:grid-cols-2">
            <div className="space-y-3">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.name")}</p>
                <Input
                  value={form.name}
                  placeholder="stock-glance"
                  maxLength={UI_COMPONENT_LIMITS.name}
                  disabled={contractLocked}
                  aria-invalid={nameInvalid}
                  className="font-mono"
                  onChange={(event) => field("name")(event.target.value)}
                />
                <p className={cn("text-[11px]", nameInvalid ? "text-destructive" : "text-muted-foreground")}>{t("fields.nameHint")}</p>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.description")}</p>
                <Input value={form.description} maxLength={UI_COMPONENT_LIMITS.description} disabled={contractLocked} onChange={(event) => field("description")(event.target.value)} />
                <p className="text-[11px] text-muted-foreground">{t("fields.descriptionHint")}</p>
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("fields.propsSummary")}</p>
                <Input
                  value={form.propsSummary}
                  placeholder="{symbol, name?}"
                  maxLength={UI_COMPONENT_LIMITS.propsSummary}
                  disabled={contractLocked}
                  className="font-mono"
                  onChange={(event) => field("propsSummary")(event.target.value)}
                />
                <p className="text-[11px] text-muted-foreground">{t("fields.propsSummaryHint")}</p>
              </div>
              {!builtin ? (
                <Tabs defaultValue="source">
                  <TabsList className="h-7">
                    <TabsTrigger value="source">{t("fields.rendererSource")}</TabsTrigger>
                    <TabsTrigger value="schema">{t("fields.propsSchema")}</TabsTrigger>
                  </TabsList>
                  <TabsContent value="source" className="space-y-1">
                    <Textarea
                      value={form.rendererSource}
                      spellCheck={false}
                      readOnly={readOnly}
                      className="h-64 resize-none overflow-y-auto font-mono text-xs [field-sizing:fixed]"
                      onChange={(event) => field("rendererSource")(event.target.value)}
                    />
                    <p className="text-[11px] text-muted-foreground">{t("fields.rendererSourceHint")}</p>
                  </TabsContent>
                  <TabsContent value="schema" className="space-y-1">
                    <Textarea
                      value={form.propsSchema}
                      spellCheck={false}
                      placeholder='{"type":"object","required":["symbol"],"properties":{"symbol":{"type":"string"}}}'
                      aria-invalid={schemaInvalid}
                      readOnly={readOnly}
                      className="h-64 resize-none overflow-y-auto font-mono text-xs [field-sizing:fixed]"
                      onChange={(event) => field("propsSchema")(event.target.value)}
                    />
                    <p className={cn("text-[11px]", schemaInvalid ? "text-destructive" : "text-muted-foreground")}>{t("fields.propsSchemaHint")}</p>
                  </TabsContent>
                </Tabs>
              ) : null}
              <div className="flex items-center justify-between">
                <p className="text-xs text-muted-foreground">{t("fields.enabled")}</p>
                <Switch size="sm" checked={form.enabled} disabled={saving || readOnly} onCheckedChange={(enabled) => field("enabled")(enabled)} />
              </div>
            </div>

            {!builtin ? (
              <div className="flex min-h-0 flex-col gap-2">
                <div className="flex items-center justify-between">
                  <p className="text-xs text-muted-foreground">{t("preview.title")}</p>
                  <Button type="button" size="sm" variant="outline" className="h-7 px-2 text-xs shadow-none" onClick={() => setPreviewSource(form.rendererSource)}>
                    {t("preview.apply")}
                  </Button>
                </div>
                <Textarea
                  value={previewPropsText}
                  spellCheck={false}
                  aria-invalid={!previewProps.ok}
                  className="h-24 resize-none font-mono text-xs [field-sizing:fixed]"
                  onChange={(event) => setPreviewPropsText(event.target.value)}
                />
                <UIBlockFrame className="min-h-24 flex-1">
                  {open && previewProps.ok ? (
                    <SandboxComponent id="preview" props={previewProps.value} definition={previewDefinition} />
                  ) : (
                    <p className="p-4 text-xs text-muted-foreground">{t("preview.invalidProps")}</p>
                  )}
                </UIBlockFrame>
              </div>
            ) : null}
          </div>

          <DialogFooter className="shrink-0 px-5 py-3">
            <Button variant="ghost" disabled={saving} onClick={() => onOpenChange(false)}>
              {t("cancel")}
            </Button>
            {readOnly ? null : (
              <Button disabled={saving} onClick={onSave}>
                {saving ? t("saving") : t("save")}
              </Button>
            )}
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}
