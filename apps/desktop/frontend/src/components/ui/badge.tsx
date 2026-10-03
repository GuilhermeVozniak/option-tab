import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "cn";
import { Slot } from "radix-ui";
import type * as React from "react";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold backdrop-blur-md",
  {
    variants: {
      variant: {
        unstyled: "",
        default: "bg-primary text-primary-foreground",
        ghost: "text-foreground",
        secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/80",
        link: "text-primary underline-offset-4 hover:underline",
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
  variant = "outline",
  asChild = false,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "span";
  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(variant === "unstyled" ? undefined : badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
