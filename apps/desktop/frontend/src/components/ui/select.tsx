import type * as React from "react";
import { cn } from "@/lib/utils";

// Select is a shadcn-styled *native* select: the settings form is driven by
// change events on real <select> elements (and jsdom tests depend on that),
// so this deliberately skips the Radix listbox in favor of the OS picker.
function Select({ className, children, ...props }: React.ComponentProps<"select">) {
  return (
    <span className="relative inline-flex" data-slot="select">
      <select
        className={cn(
          "h-8 cursor-pointer appearance-none rounded-lg border border-white/15 bg-white/8 pl-3 pr-8 text-sm text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md transition-colors outline-none focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring/40 disabled:opacity-40 [&>option]:bg-[#101527] [&>option]:text-foreground",
          className,
        )}
        {...props}
      >
        {children}
      </select>
      <svg
        aria-hidden="true"
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

export { Select };
