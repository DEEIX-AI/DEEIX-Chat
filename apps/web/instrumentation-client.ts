// Runs once in the browser before hydration (Next.js client instrumentation).

// Dev only: React's RSC performance tracks call performance.measure() with
// ranges WebKit rejects (TypeError), which trips the Next error overlay.
if (process.env.NODE_ENV === "development" && typeof performance !== "undefined") {
  const measure = performance.measure.bind(performance);
  performance.measure = ((...args: Parameters<typeof measure>) => {
    try {
      return measure(...args);
    } catch (error) {
      if (error instanceof TypeError) {
        return undefined as unknown as PerformanceMeasure;
      }
      throw error;
    }
  }) as typeof performance.measure;
}
