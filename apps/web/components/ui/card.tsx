import type * as React from "react";
import { cn } from "@/lib/utils";

function Card({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card"
      className={cn(
        "rounded-2xl border border-white/12 bg-white/7 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.12),0_16px_48px_-20px_rgba(0,0,0,0.7)] backdrop-blur-xl",
        className,
      )}
      {...props}
    />
  );
}

function CardTitle({ className, ...props }: React.ComponentProps<"h3">) {
  return (
    <h3
      data-slot="card-title"
      className={cn("m-0 text-lg font-semibold tracking-tight", className)}
      {...props}
    />
  );
}

function CardDescription({ className, ...props }: React.ComponentProps<"p">) {
  return (
    <p
      data-slot="card-description"
      className={cn("m-0 text-[15px] leading-relaxed text-muted-foreground", className)}
      {...props}
    />
  );
}

export { Card, CardDescription, CardTitle };
