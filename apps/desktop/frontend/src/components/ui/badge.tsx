import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold backdrop-blur-md",
  {
    variants: {
      variant: {
        outline: "border-white/20 bg-white/8 text-foreground/80",
        success: "border-emerald-300/30 bg-emerald-400/15 text-emerald-200",
        destructive: "border-red-300/30 bg-red-400/15 text-red-200",
        warning: "border-amber-300/30 bg-amber-400/15 text-amber-200",
      },
    },
    defaultVariants: { variant: "outline" },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span data-slot="badge" className={cn(badgeVariants({ variant, className }))} {...props} />
  );
}

export { Badge, badgeVariants };
