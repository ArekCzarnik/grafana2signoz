package compare

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"grafana2signoz/internal/mapper"
	"grafana2signoz/internal/output"
	"grafana2signoz/internal/parser"
)

// CompareDashboards compares a Grafana dashboard file vs. a SigNoz dashboard file
// using the provided rules to derive expected panel type mappings.
// It writes human-readable differences to w and returns the mismatch count.
func CompareDashboards(w io.Writer, grafanaPath, signozPath string, rules *mapper.Rules) (int, error) {
	gd, err := parser.ParseGrafanaDashboardFile(grafanaPath)
	if err != nil {
		return 0, err
	}
	sd, err := output.ReadSigNozDashboardFile(signozPath)
	if err != nil {
		return 0, err
	}
	if rules == nil {
		r := mapper.DefaultRules()
		rules = &r
	}

	// Build widget index by numeric panel id from "w_<id>"
	wid := map[int]mapper.SigNozWidget{}
	for _, wdg := range sd.Widgets {
		id := strings.TrimPrefix(wdg.ID, "w_")
		if n, err := strconv.Atoi(id); err == nil {
			wid[n] = wdg
		}
	}

	mismatches := 0
	for _, p := range gd.Panels {
		wdg, ok := wid[p.ID]
		if !ok {
			fmt.Fprintf(w, "missing: grafana panel id %d title=%q type=%q not found in SigNoz widgets\n", p.ID, p.Title, p.Type)
			mismatches++
			continue
		}
		// expected mapped panel type
		pt := strings.ToLower(p.Type)
		expect := rules.PanelTypeMap[pt]
		if expect == "" {
			expect = rules.DefaultPanel
		}
		if wdg.PanelType != expect {
			fmt.Fprintf(w, "type mismatch: id=%d title=%q grafana=%q expected_signoz=%q got=%q\n", p.ID, p.Title, p.Type, expect, wdg.PanelType)
			mismatches++
		}
		// title equality
		if strings.TrimSpace(wdg.Title) != strings.TrimSpace(p.Title) {
			fmt.Fprintf(w, "title mismatch: id=%d grafana=%q signoz=%q\n", p.ID, p.Title, wdg.Title)
			mismatches++
		}
	}

	// Also emit summary with short file names
	if mismatches > 0 {
		fmt.Fprintf(w, "\nCompared %s vs %s: %d mismatch(es).\n", filepath.Base(grafanaPath), filepath.Base(signozPath), mismatches)
	} else {
		fmt.Fprintf(w, "OK: Grafana panels match expected SigNoz widgets by id/title/type.\n")
	}
	return mismatches, nil
}
