import type * as React from "react";
import { cn } from "@/lib/utils";

function Separator({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="separator"
      role="separator"
      className={cn("my-3 h-px w-full shrink-0 bg-white/10", className)}
      {...props}
    />
  );
}

export { Separator };
