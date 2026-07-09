import { Slider as SliderPrimitive } from "@base-ui/react/slider";
import * as React from "react";
import { cn } from "../lib/utils";

// Wrapper keeps the Radix-style array value API (`number[]`) so existing
// consumers that destructure the value in `onValueChange`/`onValueCommitted`
// keep working; Base UI's widened `number | number[]` union is narrowed here.
type SliderProps = SliderPrimitive.Root.Props<number[]> & {
  rangeClassName?: string;
  rangeStyle?: React.CSSProperties;
};

const Slider = React.forwardRef<HTMLDivElement, SliderProps>(
  (
    {
      className,
      rangeClassName,
      rangeStyle,
      value,
      defaultValue,
      onValueChange,
      onValueCommitted,
      ...props
    },
    ref,
  ) => {
    const resolved = value ?? defaultValue ?? [0];
    const thumbCount = Array.isArray(resolved) ? resolved.length : 1;
    // Base UI emits a bare `number` for single-thumb sliders; the wrapper's
    // contract is `number[]`, so normalize back to an array for consumers.
    const toArray = (v: number | number[]): number[] => (Array.isArray(v) ? v : [v]);
    return (
      <SliderPrimitive.Root
        ref={ref}
        value={value}
        defaultValue={defaultValue}
        onValueChange={
          onValueChange ? (v, details) => onValueChange(toArray(v), details) : undefined
        }
        onValueCommitted={
          onValueCommitted ? (v, details) => onValueCommitted(toArray(v), details) : undefined
        }
        thumbAlignment="edge"
        className={cn("relative w-full", className)}
        {...props}
      >
        <SliderPrimitive.Control className="relative flex w-full touch-none select-none items-center">
          <SliderPrimitive.Track className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-grayA-3">
            <SliderPrimitive.Indicator
              className={cn("absolute h-full bg-accent-12", rangeClassName)}
              style={rangeStyle}
            />
            {Array.from({ length: thumbCount }).map((_, i) => (
              <SliderPrimitive.Thumb
                // biome-ignore lint/suspicious/noArrayIndexKey: <explanation>
                key={i}
                index={i}
                className="block h-4 w-4 rounded-full border border-grayA-6 bg-gray-2 shadow transition-colors duration-300 hover:border-grayA-8 focus:ring focus:ring-gray-5 focus-visible:outline-none data-disabled:pointer-events-none data-disabled:cursor-not-allowed data-disabled:opacity-50"
              />
            ))}
          </SliderPrimitive.Track>
        </SliderPrimitive.Control>
      </SliderPrimitive.Root>
    );
  },
);
Slider.displayName = "Slider";

export { Slider };
