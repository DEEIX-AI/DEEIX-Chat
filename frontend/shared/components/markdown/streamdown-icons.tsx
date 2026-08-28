import * as React from "react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type StreamdownIconProps = React.SVGProps<SVGSVGElement> & {
  size?: number;
};

type StreamdownIconComponent = React.ComponentType<StreamdownIconProps>;

export function createStreamdownTooltipIcon(
  Icon: StreamdownIconComponent,
  label: string,
): StreamdownIconComponent {
  function StreamdownTooltipIcon({ className, ...props }: StreamdownIconProps) {
    const triggerRef = React.useRef<HTMLSpanElement>(null);
    const [open, setOpen] = React.useState(false);

    React.useLayoutEffect(() => {
      const button = triggerRef.current?.closest<HTMLButtonElement>("button");
      if (!button) {
        return;
      }

      const originalTitle = button.getAttribute("title");
      const originalAriaLabel = button.getAttribute("aria-label");
      const fallbackLabel = originalTitle?.trim() || label;
      const suppliedAriaLabel = !originalAriaLabel && Boolean(fallbackLabel);

      // Streamdown exposes icon overrides but owns the surrounding button.
      // Preserve its semantics while replacing the native title with the
      // shared visual tooltip, then restore the DOM when the icon changes.
      if (suppliedAriaLabel) {
        button.setAttribute("aria-label", fallbackLabel);
      }
      button.removeAttribute("title");

      const showTooltip = () => setOpen(true);
      const hideTooltip = () => setOpen(false);
      button.addEventListener("focus", showTooltip);
      button.addEventListener("blur", hideTooltip);
      button.addEventListener("pointerenter", showTooltip);
      button.addEventListener("pointerleave", hideTooltip);

      return () => {
        button.removeEventListener("focus", showTooltip);
        button.removeEventListener("blur", hideTooltip);
        button.removeEventListener("pointerenter", showTooltip);
        button.removeEventListener("pointerleave", hideTooltip);
        if (originalTitle !== null && !button.hasAttribute("title")) {
          button.setAttribute("title", originalTitle);
        }
        if (suppliedAriaLabel && button.getAttribute("aria-label") === fallbackLabel) {
          button.removeAttribute("aria-label");
        }
      };
    }, []);

    return (
      <Tooltip open={open} onOpenChange={setOpen}>
        <TooltipTrigger asChild>
          <span ref={triggerRef} className="-m-1 inline-flex size-5 items-center justify-center">
            <Icon {...props} className={cn("size-3", className)} />
          </span>
        </TooltipTrigger>
        <TooltipContent className="z-[60]" side="top" sideOffset={6}>
          {label}
        </TooltipContent>
      </Tooltip>
    );
  }

  StreamdownTooltipIcon.displayName = `StreamdownTooltipIcon(${Icon.name || "Icon"})`;
  return StreamdownTooltipIcon;
}

export function StreamdownCheckIcon({ size = 16, ...props }: StreamdownIconProps) {
  return (
    <svg
      aria-hidden="true"
      color="currentColor"
      height={size}
      strokeLinejoin="round"
      viewBox="0 0 16 16"
      width={size}
      {...props}
    >
      <path
        clipRule="evenodd"
        d="M15.5607 3.99999L15.0303 4.53032L6.23744 13.3232C5.55403 14.0066 4.44599 14.0066 3.76257 13.3232L4.2929 12.7929L3.76257 13.3232L0.969676 10.5303L0.439346 9.99999L1.50001 8.93933L2.03034 9.46966L4.82323 12.2626C4.92086 12.3602 5.07915 12.3602 5.17678 12.2626L13.9697 3.46966L14.5 2.93933L15.5607 3.99999Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

export function StreamdownCopyIcon({ size = 16, ...props }: StreamdownIconProps) {
  return (
    <svg
      aria-hidden="true"
      color="currentColor"
      height={size}
      strokeLinejoin="round"
      viewBox="0 0 16 16"
      width={size}
      {...props}
    >
      <path
        clipRule="evenodd"
        d="M2.75 0.5C1.7835 0.5 1 1.2835 1 2.25V9.75C1 10.7165 1.7835 11.5 2.75 11.5H3.75H4.5V10H3.75H2.75C2.61193 10 2.5 9.88807 2.5 9.75V2.25C2.5 2.11193 2.61193 2 2.75 2H8.25C8.38807 2 8.5 2.11193 8.5 2.25V3H10V2.25C10 1.2835 9.2165 0.5 8.25 0.5H2.75ZM7.75 4.5C6.7835 4.5 6 5.2835 6 6.25V13.75C6 14.7165 6.7835 15.5 7.75 15.5H13.25C14.2165 15.5 15 14.7165 15 13.75V6.25C15 5.2835 14.2165 4.5 13.25 4.5H7.75ZM7.5 6.25C7.5 6.11193 7.61193 6 7.75 6H13.25C13.3881 6 13.5 6.11193 13.5 6.25V13.75C13.5 13.8881 13.3881 14 13.25 14H7.75C7.61193 14 7.5 13.8881 7.5 13.75V6.25Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

export function StreamdownDownloadIcon({ size = 16, ...props }: StreamdownIconProps) {
  return (
    <svg
      aria-hidden="true"
      color="currentColor"
      height={size}
      strokeLinejoin="round"
      viewBox="0 0 16 16"
      width={size}
      {...props}
    >
      <path
        clipRule="evenodd"
        d="M8.75 1V1.75V8.68934L10.7197 6.71967L11.25 6.18934L12.3107 7.25L11.7803 7.78033L8.70711 10.8536C8.31658 11.2441 7.68342 11.2441 7.29289 10.8536L4.21967 7.78033L3.68934 7.25L4.75 6.18934L5.28033 6.71967L7.25 8.68934V1.75V1H8.75ZM13.5 9.25V13.5H2.5V9.25V8.5H1V9.25V14C1 14.5523 1.44771 15 2 15H14C14.5523 15 15 14.5523 15 14V9.25V8.5H13.5V9.25Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

export function StreamdownMaximizeIcon({ size = 16, ...props }: StreamdownIconProps) {
  return (
    <svg
      aria-hidden="true"
      color="currentColor"
      height={size}
      strokeLinejoin="round"
      viewBox="0 0 16 16"
      width={size}
      {...props}
    >
      <path
        clipRule="evenodd"
        d="M1 5.25V6H2.5V5.25V2.5H5.25H6V1H5.25H2C1.44772 1 1 1.44772 1 2V5.25ZM5.25 14.9994H6V13.4994H5.25H2.5V10.7494V9.99939H1V10.7494V13.9994C1 14.5517 1.44772 14.9994 2 14.9994H5.25ZM15 10V10.75V14C15 14.5523 14.5523 15 14 15H10.75H10V13.5H10.75H13.5V10.75V10H15ZM10.75 1H10V2.5H10.75H13.5V5.25V6H15V5.25V2C15 1.44772 14.5523 1 14 1H10.75Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}

export function StreamdownCloseIcon({ size = 16, ...props }: StreamdownIconProps) {
  return (
    <svg
      aria-hidden="true"
      color="currentColor"
      height={size}
      strokeLinejoin="round"
      viewBox="0 0 16 16"
      width={size}
      {...props}
    >
      <path
        clipRule="evenodd"
        d="M12.4697 13.5303L13 14.0607L14.0607 13L13.5303 12.4697L9.06065 7.99999L13.5303 3.53032L14.0607 2.99999L13 1.93933L12.4697 2.46966L7.99999 6.93933L3.53032 2.46966L2.99999 1.93933L1.93933 2.99999L2.46966 3.53032L6.93933 7.99999L2.46966 12.4697L1.93933 13L2.99999 14.0607L3.53032 13.5303L7.99999 9.06065L12.4697 13.5303Z"
        fill="currentColor"
        fillRule="evenodd"
      />
    </svg>
  );
}
