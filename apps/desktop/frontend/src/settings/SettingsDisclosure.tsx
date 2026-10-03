import { type ReactNode, useState } from "react";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";

export function SettingsDisclosure({
  title,
  className,
  children,
}: {
  title: string;
  className: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Collapsible className={className} open={open} onOpenChange={setOpen}>
      <CollapsibleTrigger>{title}</CollapsibleTrigger>
      <CollapsibleContent forceMount hidden={!open}>
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}
