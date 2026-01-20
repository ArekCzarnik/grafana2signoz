package parser

import (
	"os"
	"testing"
)

func TestParseGrafanaDashboard(t *testing.T) {
	f, err := os.Open("../../testdata/sample-grafana.json")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	dash, err := ParseGrafanaDashboard(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if dash.Title != "Sample Grafana Dashboard" {
		t.Fatalf("got title %q", dash.Title)
	}
	if len(dash.Panels) != 4 {
		t.Fatalf("expected 4 panels, got %d", len(dash.Panels))
	}
}
