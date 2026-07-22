import * as React from "react";

import { cn } from "@/lib/utils";

export type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, ...props }, ref) => (
    <select
      {...props}
      ref={ref}
      className={cn(
        "app-dream-select app-motion-color app-control-compact",
        className,
      )}
      data-size="compact"
    />
  ),
);
Select.displayName = "Select";

export { Select };
