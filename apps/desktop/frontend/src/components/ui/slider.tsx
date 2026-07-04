import type * as React from "react";
import { cn } from "@/lib/utils";

// Slider is a shadcn-styled *native* range input; the glass track/thumb
// styling lives in styles.css (webkit pseudo-elements can't be done inline).
function Slider({ className, ...props }: React.ComponentProps<"input">) {
  return <input type="range" data-slot="slider" className={cn("ot-range", className)} {...props} />;
}

export { Slider };
