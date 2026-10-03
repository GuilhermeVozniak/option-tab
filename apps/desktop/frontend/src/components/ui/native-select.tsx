import { cn } from "cn";
import type * as React from "react";
import { useFormDisabled } from "./form-disabled";

function NativeSelect({
  className,
  size = "default",
  disabled,
  ...props
}: Omit<React.ComponentProps<"select">, "size"> & { size?: "sm" | "default" }) {
  const isDisabled = useFormDisabled(disabled);
  return (
    <span className="relative inline-flex" data-slot="native-select-wrapper">
      <select
        data-slot="native-select"
        data-size={size}
        disabled={isDisabled}
        className={cn(
          "h-8 cursor-pointer appearance-none rounded-lg border border-white/15 bg-white/8 pl-3 pr-8 text-sm text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md transition-colors outline-none focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring/40 disabled:opacity-40 [&>option]:bg-[#101527] [&>option]:text-foreground",
          className,
        )}
        {...props}
      />
      <svg
        aria-hidden="true"
        data-slot="native-select-icon"
        viewBox="0 0 16 16"
        className="pointer-events-none absolute right-2.5 top-1/2 size-3 -translate-y-1/2 text-foreground/50"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="m4 6 4 4 4-4" />
      </svg>
    </span>
  );
}

function NativeSelectOption({ className, ...props }: React.ComponentProps<"option">) {
  return (
    <option
      data-slot="native-select-option"
      className={cn("bg-[Canvas] text-[CanvasText]", className)}
      {...props}
    />
  );
}

function NativeSelectOptGroup({ className, ...props }: React.ComponentProps<"optgroup">) {
  return (
    <optgroup
      data-slot="native-select-optgroup"
      className={cn("bg-[Canvas] text-[CanvasText]", className)}
      {...props}
    />
  );
}

export { NativeSelect, NativeSelectOptGroup, NativeSelectOption };
