package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"grafana2signoz/internal/mapper"
)

// WriteSigNozDashboard writes pretty JSON to the given writer.
func WriteSigNozDashboard(w io.Writer, dash mapper.SigNozDashboard) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(dash)
}

// ReadSigNozDashboardFile reads a SigNoz dashboard JSON from disk.
func ReadSigNozDashboardFile(path string) (mapper.SigNozDashboard, error) {
	var dash mapper.SigNozDashboard
	b, err := os.ReadFile(path)
	if err != nil {
		return dash, err
	}
	if err := json.Unmarshal(b, &dash); err != nil {
		return dash, err
	}
	return dash, nil
}

// ValidateSigNozDashboard performs lightweight validation based on
// public examples of SigNoz dashboard JSON. It is not an official schema.
func ValidateSigNozDashboard(d mapper.SigNozDashboard) []error {
	var errs []error
	if d.Title == "" {
		errs = append(errs, fmt.Errorf("title is required"))
	}
	if len(d.Widgets) == 0 {
		errs = append(errs, fmt.Errorf("at least one widget is required"))
	}
	// Track widget ids
	ids := map[string]bool{}
	for i, w := range d.Widgets {
		if w.ID == "" {
			errs = append(errs, fmt.Errorf("widgets[%d]: id is required", i))
		}
		if w.Title == "" {
			errs = append(errs, fmt.Errorf("widgets[%d]: title is required", i))
		}
		if !isSupportedPanel(w.PanelType) {
			errs = append(errs, fmt.Errorf("widgets[%d]: unsupported panelTypes '%s'", i, w.PanelType))
		}
		if w.TimePreference == "" {
			errs = append(errs, fmt.Errorf("widgets[%d]: timePreferance is required", i))
		}
		if ids[w.ID] {
			errs = append(errs, fmt.Errorf("duplicate widget id '%s'", w.ID))
		}
		ids[w.ID] = true
	}
	// Validate layout references
	for i, l := range d.Layout {
		if l.I == "" {
			errs = append(errs, fmt.Errorf("layout[%d]: i (id) is required", i))
		}
		if !ids[l.I] {
			errs = append(errs, fmt.Errorf("layout[%d]: references unknown widget id '%s'", i, l.I))
		}
		if l.W <= 0 || l.H <= 0 {
			errs = append(errs, fmt.Errorf("layout[%d]: width/height must be > 0", i))
		}
	}
	return errs
}

func isSupportedPanel(p string) bool {
	switch p {
	case "graph", "timeseries", "bar", "histogram", "pie", "table", "value", "list", "row":
		return true
	default:
		return false
	}
}
