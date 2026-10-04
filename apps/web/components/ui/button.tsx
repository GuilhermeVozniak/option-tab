import { cva, type VariantProps } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/lib/utils";

// buttonVariants is exported so plain <a> links can wear the button styles
// without pulling in a Radix Slot dependency.
const buttonVariants = cva(
  "inline-flex cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium no-underline transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-40",
  {
    variants: {
      variant: {
        default: "border border-primary bg-primary text-primary-foreground hover:opacity-85",
        glass: "border border-border bg-background text-foreground hover:bg-muted",
        ghost: "text-muted-foreground hover:bg-muted hover:text-foreground",
      },
      size: {
        default: "h-10 px-5 text-[15px]",
        sm: "h-8 rounded-lg px-3.5 text-sm",
        lg: "h-11 px-5 text-[15px]",
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
