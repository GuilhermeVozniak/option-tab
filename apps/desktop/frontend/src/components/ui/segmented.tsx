import { cn } from "@/lib/utils";

interface SegmentedProps<V extends string> {
  // ariaLabel prefixes each option's aria-label ("Size" -> "Size small"),
  // giving tests and screen readers stable English identifiers.
  ariaLabel: string;
  value: V;
  options: { value: V; label: string }[];
  onChange: (value: V) => void;
  className?: string;
}

// Segmented is a glass segmented control (AltTab uses these for Size, Theme,
// and release behavior). One button per option, aria-pressed marks the pick.
function Segmented<V extends string>({
  ariaLabel,
  value,
  options,
  onChange,
  className,
}: SegmentedProps<V>) {
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={cn(
        "inline-flex gap-0.5 rounded-lg border border-white/12 bg-white/6 p-0.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] backdrop-blur-md",
        className,
      )}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-label={`${ariaLabel} ${o.value}`}
          aria-pressed={value === o.value}
          className={cn(
            "cursor-pointer rounded-md px-3 py-1 text-xs font-medium text-foreground/60 transition-colors hover:text-foreground",
            value === o.value &&
              "bg-white/15 text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.2)]",
          )}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export { Segmented };
