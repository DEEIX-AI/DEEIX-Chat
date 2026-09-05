import * as React from "react";

const DEFAULT_DEBOUNCE_DELAY = 250;

export function useDebouncedValue<T>(value: T, delay = DEFAULT_DEBOUNCE_DELAY): T {
  const [debouncedValue, setDebouncedValue] = React.useState(value);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedValue(value);
    }, delay);

    return () => window.clearTimeout(timer);
  }, [delay, value]);

  return debouncedValue;
}
