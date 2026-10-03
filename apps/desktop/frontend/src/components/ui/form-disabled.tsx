import { createContext, type ReactNode, useContext } from "react";

// Native controls inherit <fieldset disabled>; div-based Radix controls do not.
// Carry the same canonical loading/import state through every shared control.
const FormDisabledContext = createContext(false);

export function FormDisabledProvider({
  disabled,
  children,
}: {
  disabled: boolean;
  children: ReactNode;
}) {
  const inherited = useContext(FormDisabledContext);
  return (
    <FormDisabledContext.Provider value={inherited || disabled}>
      {children}
    </FormDisabledContext.Provider>
  );
}

export function useFormDisabled(disabled?: boolean) {
  return useContext(FormDisabledContext) || disabled || false;
}
