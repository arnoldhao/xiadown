import * as React from "react";

import { cn } from "@/lib/utils";

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
  ({ className, type, ...props }, ref) => {
    const normalizedProps =
      type !== "file" &&
      Object.prototype.hasOwnProperty.call(props, "value") &&
      props.value == null
        ? { ...props, value: "" }
        : props;
    return (
      <input
        type={type}
        className={cn("app-base-input", className)}
        ref={ref}
        {...normalizedProps}
      />
    );
  },
);
Input.displayName = "Input";

export { Input };
