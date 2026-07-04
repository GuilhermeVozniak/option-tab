import type * as React from "react";
import { cn } from "@/lib/utils";

// Checkbox is a shadcn-styled *native* checkbox rendered as a glass toggle
// switch. The real <input> stays on top (transparent) so labels, clicks,
// keyboard focus, and the .checked property keep their native semantics.
function Checkbox({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <span
      className={cn("relative inline-flex h-[18px] w-8 shrink-0", className)}
      data-slot="checkbox"
    >
      <input
        type="checkbox"
        className="peer absolute inset-0 z-10 size-full cursor-pointer appearance-none rounded-full opacity-0 outline-none"
        {...props}
      />
      <span
        aria-hidden="true"
        className="absolute inset-0 rounded-full border border-white/20 bg-white/10 shadow-[inset_0_1px_2px_rgba(0,0,0,0.35)] backdrop-blur-md transition-colors peer-checked:border-primary/60 peer-checked:bg-primary/75 peer-focus-visible:ring-2 peer-focus-visible:ring-ring/50"
      />
      <span
        aria-hidden="true"
        className="absolute left-[3px] top-1/2 size-3 -translate-y-1/2 rounded-full bg-white/95 shadow-[0_1px_3px_rgba(0,0,0,0.4)] transition-all peer-checked:left-[17px]"
      />
    </span>
  );
}

export { Checkbox };
