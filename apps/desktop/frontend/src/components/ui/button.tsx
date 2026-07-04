import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-all outline-none focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-40",
  {
    variants: {
      variant: {
        default:
          "border border-white/20 bg-primary/80 text-primary-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.25),0_6px_20px_-8px_rgba(0,0,0,0.6)] backdrop-blur-md hover:bg-primary/95",
        glass:
          "border border-white/15 bg-white/10 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.18)] backdrop-blur-md hover:bg-white/15",
        outline:
          "border border-white/20 bg-transparent text-foreground/90 backdrop-blur-md hover:bg-white/10 hover:text-foreground",
        ghost: "text-foreground/75 hover:bg-white/10 hover:text-foreground",
        destructive:
          "border border-red-400/30 bg-red-500/15 text-red-200 shadow-[inset_0_1px_0_rgba(255,255,255,0.12)] backdrop-blur-md hover:bg-red-500/25",
        dashed:
          "border border-dashed border-white/25 bg-transparent text-foreground/75 hover:bg-white/8 hover:text-foreground",
      },
      size: {
        default: "h-8 px-3.5",
        sm: "h-7 rounded-md px-2.5 text-xs",
        lg: "h-10 rounded-xl px-6",
        icon: "size-7 shrink-0 rounded-md p-0",
      },
    },
    defaultVariants: { variant: "glass", size: "default" },
  },
);

function Button({
  className,
  variant,
  size,
  ...props
}: React.ComponentProps<"button"> & VariantProps<typeof buttonVariants>) {
  return (
    <button
      type="button"
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
