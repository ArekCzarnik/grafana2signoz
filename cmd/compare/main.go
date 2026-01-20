package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"grafana2signoz/internal/mapper"
	"grafana2signoz/internal/output"
	"grafana2signoz/internal/parser"
)

func main() {
	var grafanaPath, signozPath, rulesPath string
	flag.StringVar(&grafanaPath, "grafana", "", "Path to Grafana dashboard JSON")
	flag.StringVar(&signozPath, "signoz", "", "Path to SigNoz dashboard JSON (converted)")
	flag.StringVar(&rulesPath, "rules", "", "Optional path to mapping rules JSON")
	flag.Parse()

	if grafanaPath == "" || signozPath == "" {
		fmt.Fprintln(os.Stderr, "usage: compare --grafana <grafana.json> --signoz <signoz.json> [--rules mapping.json]")
		os.Exit(2)
	}
	gd, err := parser.ParseGrafanaDashboardFile(grafanaPath)
	if err != nil {
		fatal(err)
	}
	sd, err := output.ReadSigNozDashboardFile(signozPath)
	if err != nil {
		fatal(err)
	}
	rules, err := mapper.LoadRules(rulesPath)
	if err != nil {
		fatal(err)
	}

	// Build widget index by numeric ID (strip leading 'w_')
	wid := map[int]mapper.SigNozWidget{}
	for _, w := range sd.Widgets {
		id := strings.TrimPrefix(w.ID, "w_")
		if n, err := strconv.Atoi(id); err == nil {
			wid[n] = w
		}
	}

	mismatches := 0
	for _, p := range gd.Panels {
		w, ok := wid[p.ID]
		if !ok {
			fmt.Printf("missing: grafana panel id %d title=%q type=%q not found in SigNoz widgets\n", p.ID, p.Title, p.Type)
			mismatches++
			continue
		}
		// Expected mapped type
		pt := strings.ToLower(p.Type)
		expect := rules.PanelTypeMap[pt]
		if expect == "" {
			expect = rules.DefaultPanel
		}
		if w.PanelType != expect {
			fmt.Printf("type mismatch: id=%d title=%q grafana=%q expected_signoz=%q got=%q\n", p.ID, p.Title, p.Type, expect, w.PanelType)
			mismatches++
		}
		// Title equality
		if strings.TrimSpace(w.Title) != strings.TrimSpace(p.Title) {
			fmt.Printf("title mismatch: id=%d grafana=%q signoz=%q\n", p.ID, p.Title, w.Title)
			mismatches++
		}
	}
	if mismatches > 0 {
		fmt.Printf("\nFound %d mismatch(es).\n", mismatches)
		os.Exit(1)
	}
	fmt.Println("OK: Grafana panels match expected SigNoz widgets by id/title/type")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
