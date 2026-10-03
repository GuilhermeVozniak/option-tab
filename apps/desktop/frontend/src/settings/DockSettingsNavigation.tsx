import { Button } from "@/components/ui/button";
import "./dock-settings.css";

export function DockSettingsNavigation<T extends string>({
  label,
  prefix,
  value,
  sections,
  onChange,
  compact = false,
}: {
  label: string;
  prefix: string;
  value: T;
  sections: ReadonlyArray<readonly [T, string]>;
  onChange: (value: T) => void;
  compact?: boolean;
}) {
  return (
    <nav
      aria-label={label}
      className={`ot-dock-settings-navigation${compact ? " ot-dock-settings-navigation-compact" : ""}`}
    >
      {sections.map(([id, title]) => (
        <Button
          variant="unstyled"
          key={id}
          type="button"
          id={`${prefix}-${id}-nav`}
          aria-controls={`${prefix}-${id}`}
          aria-current={value === id ? "page" : undefined}
          onClick={() => onChange(id)}
        >
          {title}
        </Button>
      ))}
    </nav>
  );
}
