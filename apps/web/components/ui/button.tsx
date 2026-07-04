import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

// buttonVariants is exported so plain <a> links can wear the button styles
// without pulling in a Radix Slot dependency.
const buttonVariants = cva(
  "inline-flex cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-xl font-semibold no-underline transition-all outline-none focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-40",
  {
    variants: {
      variant: {
        default:
          "border border-white/25 bg-gradient-to-r from-indigo-500/85 to-cyan-500/75 text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.3),0_10px_30px_-10px_rgba(99,102,241,0.7)] backdrop-blur-md hover:brightness-110",
        glass:
          "border border-white/15 bg-white/10 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.18)] backdrop-blur-md hover:bg-white/15",
        ghost: "text-foreground/75 hover:bg-white/10 hover:text-foreground",
      },
      size: {
        default: "h-10 px-5 text-[15px]",
        sm: "h-8 rounded-lg px-3.5 text-sm",
        lg: "h-12 px-7 text-[17px]",
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
