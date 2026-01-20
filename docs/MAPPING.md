**Panel Mapping**

- timeseries/graph → Graph
- barchart, bar-gauge → Bar
- piechart, pie-chart → Pie
- table → Table
- stat, gauge → Value
- histogram, heatmap → Histogram
- logs → List
- others → Timeseries (fallback) with a warning-like note in widget description.

**Queries**
- Original Grafana target `expr` strings are preserved under `query._grafanaExprs`.
- Optional regex replacements (`rules.queryReplacements`) can adjust expressions during conversion.
- SigNoz query builder fields are left as an empty stub for manual refinement post-import.

**PromQL → Builder Übersetzung (neu)**
- Unterstützt: einfache Selektoren `metric{label=..., label=~...}` inkl. Range `[5m]`.
- Funktionen: `rate|irate|increase` → `timeAggregation=rate`, Metric‑Typ `Counter`.
- Aggregation: `sum|avg|min|max|count` mit `by(...)` (vor oder nach dem Ausdruck) → `aggregateOperator`, `groupBy`.
- `histogram_quantile(q, expr)` → fügt Function‑Eintrag `{name: histogram_quantile, args: {q, leLabel: "le"}}` hinzu; `groupBy` enthält `le`.
- Offsets: `<expr> offset 1m` → Function‑Eintrag `{name: offset, args: {duration}}`.
- Bool‑Vergleiche: `<expr> > bool 0` bzw. `<expr> > 0` → `having` mit `columnName: #SIGNOZ_VALUE` und Operator/Wert.
