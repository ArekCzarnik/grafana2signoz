package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Minimal Grafana dashboard structures covering common fields used in mapping.
// Many fields are intentionally simplified; unknown fields are ignored.

type GrafanaDashboard struct {
	Title      string          `json:"title"`
	UID        string          `json:"uid"`
	Templating GrafanaTemplate `json:"templating"`
	Panels     []GrafanaPanel  `json:"panels"`
}

type GrafanaTemplate struct {
	List []GrafanaVariable `json:"list"`
}

type GrafanaVariable struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Query      interface{} `json:"query"`
	Current    interface{} `json:"current"`
	Label      string      `json:"label"`
	IncludeAll bool        `json:"includeAll"`
	Multi      bool        `json:"multi"`
}

type GrafanaPanel struct {
	ID         int             `json:"id"`
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Datasource interface{}     `json:"datasource"`
	Targets    []GrafanaTarget `json:"targets"`
	GridPos    *GrafanaGridPos `json:"gridPos"`
	Options    json.RawMessage `json:"options"`
	FieldCfg   json.RawMessage `json:"fieldConfig"`
	// Some Grafana dashboards nest rows; for simplicity we flatten if present.
	Panels []GrafanaPanel `json:"panels"`
}

type GrafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

type GrafanaTarget struct {
	RefID        string          `json:"refId"`
	Expr         string          `json:"expr"`      // PromQL/Expr
	QueryType    string          `json:"queryType"` // e.g. instant
	Datasource   interface{}     `json:"datasource"`
	LegendFormat string          `json:"legendFormat"`
	Format       string          `json:"format"`
	Raw          json.RawMessage `json:"-"` // any other fields
}

func ParseGrafanaDashboardFile(path string) (*GrafanaDashboard, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseGrafanaDashboard(f)
}

func ParseGrafanaDashboard(r io.Reader) (*GrafanaDashboard, error) {
	dec := json.NewDecoder(r)
	var dash GrafanaDashboard
	if err := dec.Decode(&dash); err != nil {
		return nil, fmt.Errorf("decode grafana json: %w", err)
	}
	// Flatten nested row panels, if any.
	dash.Panels = flattenPanels(dash.Panels)
	return &dash, nil
}

func flattenPanels(panels []GrafanaPanel) []GrafanaPanel {
	out := make([]GrafanaPanel, 0, len(panels))
	for _, p := range panels {
		if len(p.Panels) > 0 {
			out = append(out, flattenPanels(p.Panels)...)
			continue
		}
		out = append(out, p)
	}
	return out
}
