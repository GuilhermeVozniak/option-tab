import { Collapsible as CollapsiblePrimitive } from "radix-ui";
import { useFormDisabled } from "./form-disabled";

function Collapsible({
  disabled,
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.Root>) {
  const isDisabled = useFormDisabled(disabled);
  return <CollapsiblePrimitive.Root data-slot="collapsible" disabled={isDisabled} {...props} />;
}

function CollapsibleTrigger({
  disabled,
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.CollapsibleTrigger>) {
  const isDisabled = useFormDisabled(disabled);
  return (
    <CollapsiblePrimitive.CollapsibleTrigger
      data-slot="collapsible-trigger"
      {...(isDisabled ? { disabled: true } : {})}
      {...props}
    />
  );
}

function CollapsibleContent({
  ...props
}: React.ComponentProps<typeof CollapsiblePrimitive.CollapsibleContent>) {
  return <CollapsiblePrimitive.CollapsibleContent data-slot="collapsible-content" {...props} />;
}

export { Collapsible, CollapsibleContent, CollapsibleTrigger };
