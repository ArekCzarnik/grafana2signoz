package mapper

import (
	"os"
	"testing"

	"grafana2signoz/internal/parser"
)

func TestDefaultMapping(t *testing.T) {
	f, err := os.Open("../../testdata/sample-grafana.json")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	gd, err := parser.ParseGrafanaDashboard(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	rules := DefaultRules()
	sd := GrafanaToSigNoz(gd, &rules)

	if sd.Title == "" || len(sd.Widgets) == 0 {
		t.Fatalf("expected title and widgets")
	}
	if sd.Version != "v4" {
		t.Fatalf("expected version v4, got %q", sd.Version)
	}
	if len(sd.Widgets) != len(gd.Panels) {
		t.Fatalf("widgets != panels: %d vs %d", len(sd.Widgets), len(gd.Panels))
	}
	if got := sd.Widgets[0].PanelType; got != "graph" {
		t.Fatalf("panel 0 type = %s", got)
	}
}

func TestPromQLSelectorToBuilder(t *testing.T) {
	gd := &parser.GrafanaDashboard{
		Title: "T",
		Panels: []parser.GrafanaPanel{{
			ID:    100,
			Type:  "graph",
			Title: "Lag",
			Targets: []parser.GrafanaTarget{{
				RefID:        "A",
				Expr:         `nodejs_eventloop_lag_seconds{instance=~"$instance"}`,
				LegendFormat: "{{instance}}",
			}},
		}},
	}
	rules := DefaultRules()
	sd := GrafanaToSigNoz(gd, &rules)
	if len(sd.Widgets) != 1 {
		t.Fatalf("widgets=%d", len(sd.Widgets))
	}
	q := sd.Widgets[0].Query
	b, ok := q["builder"].(map[string]interface{})
	if !ok {
		t.Fatalf("no builder")
	}
	qd, ok := b["queryData"].([]interface{})
	if !ok || len(qd) != 1 {
		t.Fatalf("queryData len")
	}
	item := qd[0].(map[string]interface{})
	aggAttr := item["aggregateAttribute"].(map[string]interface{})
	if aggAttr["key"].(string) != "nodejs_eventloop_lag_seconds" {
		t.Fatalf("metric=%v", aggAttr["key"])
	}
	filters := item["filters"].(map[string]interface{})
	items := filters["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("filters=%v", len(items))
	}
}
