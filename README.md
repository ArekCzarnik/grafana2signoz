**grafana2signoz**
- Convert Grafana dashboard JSON to SigNoz-style JSON.

**Install**
- Go 1.21+
- In repo root: `go build ./cmd/grafana2signoz`

**Usage**
- Convert: `./grafana2signoz convert --input testdata/sample-grafana.json --output out-signoz.json`
- Dry-run: `./grafana2signoz convert --input testdata/sample-grafana.json --dry-run`
- Custom rules: `./grafana2signoz convert --input in.json --output out.json --rules mapping-example.json`
- Validate: `./grafana2signoz validate --input out-signoz.json`
- Directory → Directory: `./grafana2signoz convert --input grafana-dasboards --output converted-signoz`
- Compare (Grafana vs. converted SigNoz): `./grafana2signoz compare --grafana grafana-dasboards/node-application.json --signoz converted-signoz/converted-node-application.json`

**How it works**
- Parses Grafana JSON (title, variables, panels, targets).
- Maps Grafana panel types to SigNoz panel types (Graph, Bar, Pie, Table, Value, Histogram, List). Unsupported types fall back to `graph`.
- Translates simple PromQL selectors to SigNoz Metrics Builder: extracts metric, label filters (`=`, `!=`, `=~`, `!~`→`regex/nregex`), infers `groupBy` from `by(...)` or legend placeholders `{{label}}`, sets `timeAggregation=rate` for `rate|irate|increase`, otherwise `avg`. Simple scalar math like `* 100` or `/ 1024` is ignored for detection.
  - Also supported: `sum|avg|min|max|count by(...) (rate(...))` Kombinationen, `histogram_quantile(q, ...)` (als Function mit `q` + `le`), Offsets (`offset 1m` → Function), einfache bool‑Vergleiche (`> bool 0`) → Having‑Klausel.
- Preserves original Grafana query expressions under `query._grafanaExprs` for manual follow-up in SigNoz.
- Generates a SigNoz dashboard JSON with `title`, `widgets`, `layout`, `variables`.

**Schema note**
- SigNoz JSON here follows public examples/templates and validates against a lightweight checker. It may require manual touch-ups after import in SigNoz UI.

**Limitations**
- Query translation is best‑effort. Provide `--rules` to apply regex replacements or extend mapping.
- Complex Grafana options/field configs are not automatically translated.

**References**
- Migration guide: signoz.io/docs/migration/migrate-from-grafana/dashboards/
- Panel types: signoz.io/docs/dashboards/panel-types/
