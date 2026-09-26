import * as React from "react";

import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

type SettingsPageProps = React.ComponentProps<"div">;

function SettingsPage({ className, ...props }: SettingsPageProps) {
  return <div className={cn("space-y-6 pb-8 md:space-y-7 xl:space-y-8 xl:pb-10", className)} {...props} />;
}

type SettingsSectionProps = Omit<React.ComponentProps<"section">, "title"> & {
  title?: React.ReactNode;
  actions?: React.ReactNode;
  headerClassName?: string;
};

function SettingsSection({
  title,
  actions,
  children,
  className,
  headerClassName,
  ...props
}: SettingsSectionProps) {
  return (
    <section className={cn("space-y-4 px-0.5 md:space-y-5 xl:space-y-6", className)} {...props}>
      {title || actions ? (
        <SettingsSectionHeader
          title={title}
          actions={actions}
          className={headerClassName}
        />
      ) : null}
      {children}
    </section>
  );
}

type SettingsSectionHeaderProps = Omit<React.ComponentProps<"div">, "title"> & {
  title?: React.ReactNode;
  actions?: React.ReactNode;
};

function SettingsSectionHeader({
  title,
  actions,
  className,
  ...props
}: SettingsSectionHeaderProps) {
  const hasActions = Boolean(actions);

  return (
    <div
      className={cn(
        hasActions ? "flex h-9 items-center justify-between gap-2 md:h-10 md:gap-3" : "flex h-9 items-center md:h-10",
        className,
      )}
      {...props}
    >
      <div className="min-w-0">
        {title ? <h3 className="text-sm font-semibold">{title}</h3> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center justify-end gap-2">{actions}</div> : null}
    </div>
  );
}

type SettingsFieldRowProps = React.ComponentProps<typeof Field> & {
  title: React.ReactNode;
  description?: React.ReactNode;
  controlClassName?: string;
};

function SettingsFieldRow({
  title,
  description,
  children,
  className,
  controlClassName,
  ...props
}: SettingsFieldRowProps) {
  return (
    <Field className={className} {...props}>
      <div className="flex flex-col gap-2 md:flex-row md:items-start md:gap-4 xl:gap-6">
        <div className="min-w-0 flex-1">
          <FieldLabel>{title}</FieldLabel>
          {description ? <FieldDescription className="text-[11px]">{description}</FieldDescription> : null}
        </div>
        <div className={cn("flex w-full min-w-0 justify-start md:w-44 md:shrink-0 md:justify-end xl:w-52", controlClassName)}>
          {children}
        </div>
      </div>
    </Field>
  );
}

function SettingsFieldList({ className, ...props }: React.ComponentProps<typeof FieldGroup>) {
  return <FieldGroup className={cn("gap-0", className)} {...props} />;
}

function SettingsFieldInset({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "border-l border-border/70 pl-3 md:pl-4",
        className,
      )}
      {...props}
    />
  );
}

type SettingsFieldItemProps = React.ComponentProps<"div"> & {
  index?: number;
};

function SettingsFieldItem({ index = 0, className, ...props }: SettingsFieldItemProps) {
  return <div className={cn(index === 0 ? "" : "pt-3 md:pt-4", className)} {...props} />;
}

function SettingsSectionSeparator({ className, ...props }: React.ComponentProps<typeof Separator>) {
  return <Separator className={cn("mx-0.5 my-7 md:my-8 xl:mx-1 xl:my-10", className)} {...props} />;
}

export {
  SettingsFieldInset,
  SettingsFieldItem,
  SettingsFieldList,
  SettingsFieldRow,
  SettingsPage,
  SettingsSection,
  SettingsSectionHeader,
  SettingsSectionSeparator,
};
