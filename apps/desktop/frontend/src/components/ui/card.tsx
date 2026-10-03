import { cn } from "cn";
import { Slot } from "radix-ui";
import type * as React from "react";

function Card({
  className,
  appearance = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"div"> & { appearance?: "default" | "unstyled"; asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "div";
  return (
    <Comp
      data-slot="card"
      data-appearance={appearance}
      className={cn(
        appearance === "unstyled"
          ? undefined
          : "rounded-2xl border border-white/12 bg-white/7 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.12),0_12px_40px_-16px_rgba(0,0,0,0.6)] backdrop-blur-xl",
        className,
      )}
      {...props}
    />
  );
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn("flex flex-col gap-1 px-5 pt-4", className)}
      {...props}
    />
  );
}

function CardTitle({ className, ...props }: React.ComponentProps<"h2">) {
  return (
    <h2
      data-slot="card-title"
      className={cn("m-0 text-[15px] font-semibold tracking-tight text-foreground", className)}
      {...props}
    />
  );
}

function CardDescription({ className, ...props }: React.ComponentProps<"p">) {
  return (
    <p
      data-slot="card-description"
      className={cn("m-0 text-[13px] text-muted-foreground", className)}
      {...props}
    />
  );
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="card-content" className={cn("px-5 pb-4 pt-3", className)} {...props} />;
}

function CardAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-action"
      className={cn("self-start justify-self-end", className)}
      {...props}
    />
  );
}
function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-footer"
      className={cn("flex items-center px-5 pb-4", className)}
      {...props}
    />
  );
}

export { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle };
