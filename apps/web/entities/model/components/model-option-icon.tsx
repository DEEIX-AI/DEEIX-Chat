import { ModelIcon } from "@/entities/model/components/model-icon";
import { cn } from "@/lib/utils";

export function ModelOptionIcon({
  iconUrl,
  label,
  size = 16,
  className,
}: {
  iconUrl?: string | null;
  label: string;
  size?: number;
  className?: string;
}) {
  return (
    <ModelIcon iconUrl={iconUrl} label={label} size={size} className={cn("self-center", className)} />
  );
}
