// Per-point formulas of catalog components inside a live block.
//
// A template that reads `x`, `i` or `row` (and the block does not define that
// name) is left unevaluated by resolveBlock; the component's data is expanded
// here, once per point, before the component validates its props:
//
//   chart       series[].data: "{formula of x}"   one value per entry of `x`
//   data-table  where: "{row.price < budget}"     rows kept when true
//   card-grid   where: "{contains(row.tag, q)}"   items kept when true
//
// A point that fails or is not a finite number becomes null (a gap in the
// line), so one bad point never shifts the others.

import { asUIValue, type BlockIssue, isPointTemplate } from "./block.ts";
import type { UIValue } from "./expression.ts";

// Which list prop `where` filters, per component.
export const WHERE_LIST_PROP: Readonly<Record<string, string>> = {
  "data-table": "rows",
  "card-grid": "items",
};

export type RowPredicate = (row: unknown, index: number) => boolean;

// Expands chart series formulas. Returns props unchanged for other components.
export function expandPointProps(component: string, props: Record<string, unknown>, issues: BlockIssue[]): Record<string, unknown> {
  if (component !== "chart" || !Array.isArray(props.series)) {
    return props;
  }
  const xs = Array.isArray(props.x) ? props.x : undefined;
  const series = props.series.map((entry, index) => {
    if (typeof entry !== "object" || entry === null || !("data" in entry) || !isPointTemplate(entry.data)) {
      return entry;
    }
    const template = entry.data;
    if (!xs) {
      issues.push({ path: `series[${index}].data`, message: "a per-point formula needs x" });
      return { ...entry, data: [] };
    }
    const data = xs.map((x, pointIndex) => {
      const value = template.evaluate({ x: asUIValue(x) ?? null, i: pointIndex });
      return typeof value === "number" && Number.isFinite(value) ? value : null;
    });
    return { ...entry, data };
  });
  return { ...props, series };
}

// Splits `where` off the props. The predicate is applied after validation, to
// the list prop named in WHERE_LIST_PROP; a literal true/false keeps all/none.
export function takeWhere(component: string, props: Record<string, unknown>, issues: BlockIssue[]): { props: Record<string, unknown>; where?: RowPredicate } {
  if (!Object.hasOwn(props, "where")) {
    return { props };
  }
  const { where, ...rest } = props;
  if (!Object.hasOwn(WHERE_LIST_PROP, component)) {
    issues.push({ path: "where", message: `${component} does not filter` });
    return { props: rest };
  }
  if (typeof where === "boolean") {
    return { props: rest, where: () => where };
  }
  if (!isPointTemplate(where)) {
    issues.push({ path: "where", message: "where must be a formula of row" });
    return { props: rest };
  }
  return {
    props: rest,
    where: (row, index) => {
      const point: Record<string, UIValue> = { row: asUIValue(row) ?? null, i: index };
      return where.evaluate(point) === true;
    },
  };
}
