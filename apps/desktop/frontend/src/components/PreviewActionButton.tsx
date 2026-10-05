import { type ComponentProps, useState } from "react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

type PreviewActionButtonProps = Omit<
  ComponentProps<typeof Button>,
  "aria-label" | "title" | "ref" | "asChild" | "variant"
> & { label: string };

export function PreviewActionButton({ label, ...props }: PreviewActionButtonProps) {
  const [button, setButton] = useState<HTMLButtonElement | null>(null);

  return (
    <TooltipProvider delayDuration={2500} skipDelayDuration={0}>
      <Tooltip disableHoverableContent>
        <TooltipTrigger asChild>
          <Button ref={setButton} variant="unstyled" aria-label={label} {...props} />
        </TooltipTrigger>
        <TooltipContent
          // Inherit the switcher's theme outside its clipped panel and cards.
          container={button?.closest<HTMLElement>('[role="dialog"]')}
          side="bottom"
          sideOffset={6}
          collisionPadding={8}
          className="pointer-events-none max-w-[min(20rem,var(--radix-tooltip-content-available-width))] break-words"
        >
          {label}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
