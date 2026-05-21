import * as React from 'react';

interface CommonControlledStateProps<T> {
  value?: T;
  defaultValue?: T;
}

export function useControlledState<T>(
  props: CommonControlledStateProps<T> & {
    onChange?: (value: T) => void;
  },
): readonly [T, (next: T) => void] {
  const { value, defaultValue, onChange } = props;

  const [state, setInternalState] = React.useState<T>(
    value !== undefined ? value : (defaultValue as T),
  );

  React.useEffect(() => {
    if (value !== undefined) setInternalState(value);
  }, [value]);

  const setState = React.useCallback(
    (next: T) => {
      setInternalState(next);
      onChange?.(next);
    },
    [onChange],
  );

  return [state, setState] as const;
}
