import { type VariantProps } from "class-variance-authority";
import { cn } from "cn";
import { ToggleGroup as ToggleGroupPrimitive } from "radix-ui";
import * as React from "react";
import { toggleVariants } from "@/components/ui/toggle";
import { useFormDisabled } from "./form-disabled";

const ToggleGroupContext = React.createContext<
  VariantProps<typeof toggleVariants> & {
    appearance?: "default" | "unstyled";
    spacing?: number;
  }
>({
  size: "default",
  variant: "default",
  spacing: 0,
});

function ToggleGroup({
  className,
  appearance = "default",
  variant,
  size,
  spacing = 0,
  children,
  disabled,
  ...props
}: React.ComponentProps<typeof ToggleGroupPrimitive.Root> &
  VariantProps<typeof toggleVariants> & {
    appearance?: "default" | "unstyled";
    spacing?: number;
  }) {
  const isDisabled = useFormDisabled(disabled);
  return (
    <ToggleGroupPrimitive.Root
      data-slot="toggle-group"
      data-variant={variant}
      data-size={size}
      data-spacing={spacing}
      style={{ "--gap": spacing } as React.CSSProperties}
      className={
        appearance === "unstyled"
          ? className
          : cn(
              "group/toggle-group flex w-fit items-center gap-[--spacing(var(--gap))] rounded-md data-[spacing=default]:data-[variant=outline]:shadow-xs",
              className,
            )
      }
      disabled={isDisabled}
      {...props}
    >
      <ToggleGroupContext.Provider value={{ variant, size, spacing, appearance }}>
        {children}
      </ToggleGroupContext.Provider>
    </ToggleGroupPrimitive.Root>
  );
}

function ToggleGroupItem({
  className,
  appearance,
  children,
  variant,
  size,
  disabled,
  ...props
}: React.ComponentProps<typeof ToggleGroupPrimitive.Item> &
  VariantProps<typeof toggleVariants> & { appearance?: "default" | "unstyled" }) {
  const context = React.useContext(ToggleGroupContext);

  const isDisabled = useFormDisabled(disabled);
  return (
    <ToggleGroupPrimitive.Item
      data-slot="toggle-group-item"
      data-variant={context.variant || variant}
      data-size={context.size || size}
      data-spacing={context.spacing}
      className={
        (appearance ?? context.appearance) === "unstyled"
          ? className
          : cn(
              toggleVariants({
                variant: context.variant || variant,
                size: context.size || size,
              }),
              "w-auto min-w-0 shrink-0 px-3 focus:z-10 focus-visible:z-10",
              "data-[spacing=0]:rounded-none data-[spacing=0]:shadow-none data-[spacing=0]:first:rounded-l-md data-[spacing=0]:last:rounded-r-md data-[spacing=0]:data-[variant=outline]:border-l-0 data-[spacing=0]:data-[variant=outline]:first:border-l",
              className,
            )
      }
      disabled={isDisabled}
      {...props}
    >
      {children}
    </ToggleGroupPrimitive.Item>
  );
}

export { ToggleGroup, ToggleGroupItem };
