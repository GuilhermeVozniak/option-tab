import type * as React from "react";
import { cn } from "@/lib/utils";

function Input({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      data-slot="input"
      className={cn(
        "h-8 min-w-0 rounded-lg border border-white/15 bg-white/8 px-3 py-1 text-sm text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md transition-colors outline-none placeholder:text-foreground/35 focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-ring/40 disabled:opacity-40",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
