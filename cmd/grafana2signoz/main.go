package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"grafana2signoz/internal/compare"
	"grafana2signoz/internal/mapper"
	"grafana2signoz/internal/output"
	"grafana2signoz/internal/parser"
)

var (
	inputPath  string
	outputPath string
	rulesPath  string
	dryRun     bool
)

func main() {
	root := &cobra.Command{
		Use:   "grafana2signoz",
		Short: "Convert Grafana dashboard JSON to SigNoz-compatible JSON",
	}

	convertCmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert Grafana JSON to SigNoz JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return fmt.Errorf("--input is required (file or directory)")
			}
			// Load mapping rules (optional)
			rules, err := mapper.LoadRules(rulesPath)
			if err != nil {
				return err
			}

			info, err := os.Stat(inputPath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return convertDir(inputPath, outputPath, rules)
			}

			// Single file
			gDash, err := parser.ParseGrafanaDashboardFile(inputPath)
			if err != nil {
				return err
			}
			sDash := mapper.GrafanaToSigNoz(gDash, rules)
			if errs := output.ValidateSigNozDashboard(sDash); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "validation: %v\n", e)
				}
			}
			if dryRun {
				return output.WriteSigNozDashboard(os.Stdout, sDash)
			}
			if outputPath == "" {
				return fmt.Errorf("--output is required when not using --dry-run")
			}
			f, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer f.Close()
			return output.WriteSigNozDashboard(f, sDash)
		},
	}
	convertCmd.Flags().StringVar(&inputPath, "input", "", "Path to Grafana dashboard JSON")
	convertCmd.Flags().StringVar(&outputPath, "output", "", "Path to write SigNoz JSON")
	convertCmd.Flags().StringVar(&rulesPath, "rules", "", "Optional path to custom mapping rules JSON")
	convertCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print SigNoz JSON to stdout without writing a file")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a SigNoz dashboard JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputPath == "" {
				return fmt.Errorf("--input is required")
			}
			sDash, err := output.ReadSigNozDashboardFile(inputPath)
			if err != nil {
				return err
			}
			errs := output.ValidateSigNozDashboard(sDash)
			if len(errs) == 0 {
				fmt.Fprintln(os.Stdout, "OK: looks like a valid SigNoz dashboard structure")
				return nil
			}
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "validation: %v\n", e)
			}
			return fmt.Errorf("validation failed with %d issue(s)", len(errs))
		},
	}
	validateCmd.Flags().StringVar(&inputPath, "input", "", "Path to SigNoz dashboard JSON")

	var grafanaFile, signozFile string
	compareCmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare Grafana dashboard vs. converted SigNoz dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			if grafanaFile == "" || signozFile == "" {
				return fmt.Errorf("--grafana and --signoz are required")
			}
			rules, err := mapper.LoadRules(rulesPath)
			if err != nil {
				return err
			}
			mismatches, err := compare.CompareDashboards(os.Stdout, grafanaFile, signozFile, rules)
			if err != nil {
				return err
			}
			if mismatches > 0 {
				return fmt.Errorf("found %d mismatch(es)", mismatches)
			}
			return nil
		},
	}
	compareCmd.Flags().StringVar(&grafanaFile, "grafana", "", "Path to Grafana dashboard JSON")
	compareCmd.Flags().StringVar(&signozFile, "signoz", "", "Path to SigNoz dashboard JSON (converted)")
	compareCmd.Flags().StringVar(&rulesPath, "rules", "", "Optional path to custom mapping rules JSON")

	root.AddCommand(convertCmd)
	root.AddCommand(validateCmd)
	root.AddCommand(compareCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func convertDir(inDir, outPath string, rules *mapper.Rules) error {
	// Determine output directory: if --output is file or dir
	outDir := outPath
	if outDir == "" {
		outDir = filepath.Join(inDir, "..", "converted-signoz")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(inDir)
	if err != nil {
		return err
	}
	var lastErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		inFile := filepath.Join(inDir, e.Name())
		gDash, err := parser.ParseGrafanaDashboardFile(inFile)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", e.Name(), err)
			continue
		}
		sDash := mapper.GrafanaToSigNoz(gDash, rules)
		if errs := output.ValidateSigNozDashboard(sDash); len(errs) > 0 {
			for _, ve := range errs {
				fmt.Fprintf(os.Stderr, "%s: validation: %v\n", e.Name(), ve)
			}
		}
		outFile := filepath.Join(outDir, fmt.Sprintf("converted-%s", e.Name()))
		f, err := os.Create(outFile)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outFile, err)
			continue
		}
		if err := output.WriteSigNozDashboard(f, sDash); err != nil {
			lastErr = err
		}
		f.Close()
	}
	return lastErr
}
