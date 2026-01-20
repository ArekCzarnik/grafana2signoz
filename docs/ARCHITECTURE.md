**Architecture Overview**

- `cmd/grafana2signoz/main.go`: Cobra CLI with `convert` and `validate`.
- `internal/parser`: Reads Grafana dashboard JSON into minimal structs.
- `internal/mapper`: Maps Grafana panels → SigNoz widgets, applies rules, packs grid.
- `internal/output`: Writes SigNoz JSON and performs lightweight validation.

**Key Data Structures**

- `parser.GrafanaDashboard`
  - `Title string`
  - `UID string`
  - `Templating { List []GrafanaVariable }`
  - `Panels []GrafanaPanel`

- `parser.GrafanaPanel`
  - `ID int`, `Type string`, `Title string`
  - `Datasource any`
  - `Targets []GrafanaTarget { RefID, Expr, ... }`
  - `GridPos { H,W,X,Y }`

- `mapper.Rules` (customizable via JSON file)
  - `PanelTypeMap map[string]string` (Grafana → SigNoz)
  - `DefaultPanel string`
  - `QueryReplacements []{ Match, Replacement }` (regex-based)
  - `DefaultWidth, DefaultHeight int`

- `mapper.SigNozDashboard`
  - `Title string`, `Version string` (e.g., `v4`), `Tags []string`
  - `Variables map[string]any` (best-effort preservation of Grafana variables)
  - `Widgets []SigNozWidget`
  - `Layout []SigNozLayout { H,W,X,Y,I }`

- `mapper.SigNozWidget`
  - `ID string`, `Title string`, `PanelType string` (one of: timeseries, bar, pie, table, value, histogram, list)
  - `TimePreference string` (e.g., `GLOBAL_TIME`)
  - `Description string`
  - `Query map[string]any` (builder stub + `_grafanaExprs` for manual follow-up)

**Mapping Notes**

- Supported mappings out-of-the-box:
  - Grafana `timeseries|graph` → SigNoz `timeseries`
  - `barchart|bar-gauge` → `bar`
  - `piechart|pie-chart` → `pie`
  - `table` → `table`
  - `stat|gauge` → `value`
  - `histogram|heatmap` → `histogram`
  - Fallback → `timeseries`

- Queries are preserved under `query._grafanaExprs` and can be massaged via `queryReplacements` rules.

**Validation**

- Ensures presence of `title`, at least one widget, supported `panelTypes`, non-empty `timePreferance`, unique IDs, and layout-to-widget references.
