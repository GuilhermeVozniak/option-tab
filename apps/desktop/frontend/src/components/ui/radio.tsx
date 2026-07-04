import type * as React from "react";
import { cn } from "@/lib/utils";

// Radio is a shadcn-styled *native* radio input (see checkbox.tsx for why).
function Radio({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type="radio"
      data-slot="radio"
      className={cn(
        "size-4 shrink-0 cursor-pointer appearance-none rounded-full border border-white/25 bg-white/10 shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)] backdrop-blur-md transition-all outline-none checked:border-[5px] checked:border-primary checked:bg-white focus-visible:ring-2 focus-visible:ring-ring/50",
        className,
      )}
      {...props}
    />
  );
}

export { Radio };
