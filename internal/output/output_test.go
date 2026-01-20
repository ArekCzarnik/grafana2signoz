package output

import (
	"bytes"
	"testing"

	"grafana2signoz/internal/mapper"
)

func TestValidateAndWrite(t *testing.T) {
	sd := mapper.SigNozDashboard{
		Title:     "Test",
		Version:   "v4",
		Tags:      []string{"t"},
		Variables: map[string]interface{}{},
		Widgets: []mapper.SigNozWidget{{
			ID:             "w_1",
			Title:          "A",
			PanelType:      "graph",
			TimePreference: "GLOBAL_TIME",
			Query:          map[string]interface{}{"queryType": "builder", "builder": map[string]interface{}{}},
		}},
		Layout: []mapper.SigNozLayout{{I: "w_1", W: 6, H: 6, X: 0, Y: 0}},
	}
	errs := ValidateSigNozDashboard(sd)
	if len(errs) > 0 {
		t.Fatalf("unexpected validation errors: %+v", errs)
	}
	var buf bytes.Buffer
	if err := WriteSigNozDashboard(&buf, sd); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("no output")
	}
}
