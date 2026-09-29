// Runs once in the browser before hydration (Next.js client instrumentation).

// Dev only: React's RSC performance tracks call performance.measure() with
// ranges WebKit rejects (TypeError), which trips the Next error overlay.
if (process.env.NODE_ENV === "development" && typeof performance !== "undefined") {
  const measure = performance.measure.bind(performance);
  // Swallowing the WebKit TypeError leaves no real entry to return; an empty
  // measure keeps the declared return type without asserting `undefined` into it.
  const emptyMeasure = (name: string): PerformanceMeasure => {
    performance.mark(name);
    const entry = measure(name, name);
    performance.clearMarks(name);
    performance.clearMeasures(name);
    return entry;
  };
  performance.measure = (...args: Parameters<typeof measure>): PerformanceMeasure => {
    try {
      return measure(...args);
    } catch (error) {
      if (error instanceof TypeError) {
        return emptyMeasure(`${args[0]}:rejected`);
      }
      throw error;
    }
  };
}
