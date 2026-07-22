import { Check } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

export type CheckboxProps = Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  "type"
>;

/** Native checkbox semantics with one shared Dream appearance contract. */
export const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  function Checkbox({ className, ...props }, ref) {
    return (
      <span
        className={cn("app-dream-checkbox", className)}
        data-app-checkbox="true"
        data-menu-indicator="true"
      >
        <input
          {...props}
          ref={ref}
          className="app-dream-checkbox__control"
          type="checkbox"
        />
        <span aria-hidden="true" className="app-dream-checkbox__visual">
          <Check />
        </span>
      </span>
    );
  },
);
