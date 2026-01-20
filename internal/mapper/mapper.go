package mapper

import (
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"grafana2signoz/internal/parser"
)

// Rules defines custom mapping behavior loaded from a JSON file.
// All fields are optional; sensible defaults are applied if empty.
type Rules struct {
	PanelTypeMap map[string]string `json:"panelTypeMap"`
	DefaultPanel string            `json:"defaultPanel"`
	// QueryReplacements are applied to target expressions in order.
	QueryReplacements []Replacement `json:"queryReplacements"`
	// Default grid sizes for widgets (SigNoz uses 24 cols). Defaults 6x6.
	DefaultWidth  int `json:"defaultWidth"`
	DefaultHeight int `json:"defaultHeight"`
}

type Replacement struct {
	Match       string `json:"match"`
	Replacement string `json:"replacement"`
}

func LoadRules(path string) (*Rules, error) {
	if strings.TrimSpace(path) == "" {
		// Return default rules
		r := DefaultRules()
		return &r, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Rules
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}
	// Fill defaults where empty
	def := DefaultRules()
	if r.PanelTypeMap == nil {
		r.PanelTypeMap = def.PanelTypeMap
	}
	if r.DefaultPanel == "" {
		r.DefaultPanel = def.DefaultPanel
	}
	if r.DefaultWidth == 0 {
		r.DefaultWidth = def.DefaultWidth
	}
	if r.DefaultHeight == 0 {
		r.DefaultHeight = def.DefaultHeight
	}
	return &r, nil
}

func DefaultRules() Rules {
	return Rules{
		PanelTypeMap: map[string]string{
			// Grafana -> SigNoz panel type mapping
			"graph":      "graph",
			"timeseries": "graph",
			"barchart":   "bar",
			"bar-gauge":  "bar",
			"gauge":      "value",
			"stat":       "value",
			"singlestat": "value",
			"table":      "table",
			"piechart":   "pie",
			"pie-chart":  "pie",
			"heatmap":    "histogram",
			"histogram":  "histogram",
			// Fallbacks
			"text": "value",
			"logs": "list",
		},
		DefaultPanel:  "graph",
		DefaultWidth:  6,
		DefaultHeight: 6,
	}
}

// Internal normalized SigNoz structures (subset needed for import)
type SigNozDashboard struct {
	Title           string                 `json:"title"`
	UUID            string                 `json:"uuid,omitempty"`
	Version         string                 `json:"version"`
	Tags            []string               `json:"tags"`
	Layout          []SigNozLayout         `json:"layout"`
	Widgets         []SigNozWidget         `json:"widgets"`
	Variables       map[string]interface{} `json:"variables"`
	PanelMap        map[string]interface{} `json:"panelMap,omitempty"`
	UploadedGrafana bool                   `json:"uploadedGrafana,omitempty"`
	Description     string                 `json:"description,omitempty"`
}

type SigNozLayout struct {
	H      int    `json:"h"`
	W      int    `json:"w"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	I      string `json:"i"`
	Moved  bool   `json:"moved"`
	Static bool   `json:"static"`
}

type SigNozWidget struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	PanelType      string                 `json:"panelTypes"`
	TimePreference string                 `json:"timePreferance"`
	Description    string                 `json:"description,omitempty"`
	Query          map[string]interface{} `json:"query"`
}

// GrafanaToSigNoz converts a parsed Grafana dashboard to a SigNoz dashboard
// using provided rules.
func GrafanaToSigNoz(g *parser.GrafanaDashboard, rules *Rules) SigNozDashboard {
	if rules == nil {
		r := DefaultRules()
		rules = &r
	}

	s := SigNozDashboard{
		Title:           nonEmpty(g.Title, "Migrated Grafana Dashboard"),
		Version:         "v4",
		Tags:            []string{"migrated", "grafana"},
		Layout:          []SigNozLayout{},
		Widgets:         []SigNozWidget{},
		Variables:       map[string]interface{}{},
		PanelMap:        map[string]interface{}{},
		UploadedGrafana: false,
		Description:     "Converted from Grafana dashboard JSON",
	}

	// Variables mapping (best effort)
	s.Variables = buildVariables(g)

	// Panels -> Widgets with simple grid packing (24 cols)
	const cols = 24
	curX, curY := 0, 0
	rowH := 0

	// Sort panels by grid position when available to keep visual order.
	panels := append([]parser.GrafanaPanel(nil), g.Panels...)
	sort.SliceStable(panels, func(i, j int) bool {
		pi, pj := panels[i].GridPos, panels[j].GridPos
		if pi == nil && pj == nil {
			return panels[i].ID < panels[j].ID
		}
		if pi == nil {
			return false
		}
		if pj == nil {
			return true
		}
		if pi.Y == pj.Y {
			return pi.X < pj.X
		}
		return pi.Y < pj.Y
	})

	for _, p := range panels {
		pt := strings.ToLower(p.Type)
		mapped, ok := rules.PanelTypeMap[pt]
		if !ok || mapped == "" {
			mapped = rules.DefaultPanel
		}

		// Compose a basic widget query: preserve original targets as a note
		q := makeSigNozQueryFromTargets(p.Targets, rules)

		id := fmt.Sprintf("w_%d", p.ID)
		widget := SigNozWidget{
			ID:             id,
			Title:          nonEmpty(p.Title, strings.Title(mapped)),
			PanelType:      mapped,
			TimePreference: "GLOBAL_TIME",
			Description:    fmt.Sprintf("Migrated from Grafana (type: %s); original queries preserved in _grafanaExprs.", pt),
			Query:          q,
		}

		w := rules.DefaultWidth
		h := rules.DefaultHeight
		if p.GridPos != nil {
			if p.GridPos.W > 0 {
				w = p.GridPos.W
			}
			if p.GridPos.H > 0 {
				h = p.GridPos.H
			}
		}

		if curX+w > cols { // wrap to next row
			curX = 0
			curY += rowH
			rowH = 0
		}
		layout := SigNozLayout{H: h, W: w, X: curX, Y: curY, I: id, Moved: false, Static: false}
		curX += w
		if h > rowH {
			rowH = h
		}

		s.Widgets = append(s.Widgets, widget)
		s.Layout = append(s.Layout, layout)
	}

	return s
}

func collectExprs(ts []parser.GrafanaTarget, reps []Replacement) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		expr := t.Expr
		if expr == "" {
			continue
		}
		for _, r := range reps {
			if r.Match == "" {
				continue
			}
			expr = regexReplace(expr, r.Match, r.Replacement)
		}
		out = append(out, expr)
	}
	return out
}

func regexReplace(in, pattern, repl string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return in
	}
	return re.ReplaceAllString(in, repl)
}

func nonEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// buildVariables converts Grafana variables into SigNoz-like variable objects (best-effort).
func buildVariables(g *parser.GrafanaDashboard) map[string]interface{} {
	out := map[string]interface{}{}
	for i, v := range g.Templating.List {
		id := newUUID()
		typ := strings.ToUpper(v.Type)
		if typ == "" {
			typ = "TEXT"
		}
		name := v.Name
		if name == "" {
			name = fmt.Sprintf("var_%d", i)
		}
		out[id] = map[string]interface{}{
			"id":               id,
			"name":             name,
			"type":             typ,
			"modificationUUID": newUUID(),
			"queryValue":       v.Query,
			"multiSelect":      v.Multi,
			"showALLOption":    v.IncludeAll,
			"order":            i,
			"description":      v.Label,
			"sort":             "DISABLED",
			"customValue":      "",
			"textboxValue":     "",
			"allSelected":      false,
		}
	}
	return out
}

// newUUID returns a pseudo-random UUID v4 string.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		// very unlikely; fallback to zeros which still produce a string
	}
	// Set version and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hex := func(x byte) string {
		const hexdigits = "0123456789abcdef"
		return string([]byte{hexdigits[x>>4], hexdigits[x&0x0f]})
	}
	sb := strings.Builder{}
	for i, v := range b {
		sb.WriteString(hex(v))
		switch i {
		case 3, 5, 7, 9:
			sb.WriteByte('-')
		}
	}
	return sb.String()
}

// ---------- PromQL -> SigNoz Builder mapping ----------

type labelMatcher struct {
	Key   string
	Op    string // =, !=, regex, nregex
	Value string
}

type promQL struct {
	Metric    string
	Labels    []labelMatcher
	Func      string   // rate, irate, increase
	InnerFunc string   // inner function when Func is a wrapper like histogram_quantile
	Range     string   // [5m]
	Agg       string   // sum, avg, min, max, count
	By        []string // by(...) labels
	// histogram_quantile support
	Quantile string // e.g., "0.95" when Func == "histogram_quantile"
	// offset support
	Offset string // e.g., "1m"
	// comparison support
	CmpOp     string // >, <, >=, <=, ==, !=
	CmpRight  string // number as string
	CmpIsBool bool   // whether 'bool' modifier was used
}

func makeSigNozQueryFromTargets(ts []parser.GrafanaTarget, rules *Rules) map[string]interface{} {
	// Defaults
	queryID := newUUID()

	// Build queryData slice, and promql entries from grafana targets
	qd := make([]interface{}, 0, len(ts))
	promql := make([]map[string]interface{}, 0, int(math.Max(1, float64(len(ts)))))
	for _, t := range ts {
		expr := strings.TrimSpace(t.Expr)
		if expr == "" {
			continue
		}
		p := parsePromQL(expr)
		// Convert to builder query item
		qitem := map[string]interface{}{
			"aggregateAttribute": map[string]interface{}{
				"dataType": "float64",
				"id":       fmt.Sprintf("%s--float64--%s--true", p.Metric, guessMetricType(p)),
				"isColumn": true,
				"isJSON":   false,
				"key":      p.Metric,
				"type":     guessMetricType(p),
			},
			"aggregateOperator": pickAggOperator(p),
			"dataSource":        "metrics",
			"disabled":          false,
			"expression":        nonEmpty(t.RefID, "A"),
			"filters": map[string]interface{}{
				"items": buildFilterItems(p.Labels),
				"op":    "AND",
			},
			"functions":        buildFunctions(p),
			"groupBy":          buildGroupBy(p, t.LegendFormat),
			"having":           []interface{}{},
			"legend":           nonEmpty(t.LegendFormat, ""),
			"limit":            nil,
			"orderBy":          []interface{}{},
			"queryName":        nonEmpty(t.RefID, "A"),
			"reduceTo":         "avg",
			"spaceAggregation": "sum",
			"stepInterval":     60,
			"timeAggregation":  pickTimeAggregation(p),
		}
		// Add comparison as HAVING when possible
		if p.CmpOp != "" && p.CmpRight != "" {
			qitem["having"] = []interface{}{
				map[string]interface{}{
					"columnName": "#SIGNOZ_VALUE",
					"op":         p.CmpOp,
					"value":      p.CmpRight,
				},
			}
		}
		qd = append(qd, qitem)
		promql = append(promql, map[string]interface{}{
			"disabled": false,
			"legend":   nonEmpty(t.LegendFormat, ""),
			"name":     nonEmpty(t.RefID, "A"),
			"query":    expr,
		})
	}

	return map[string]interface{}{
		"queryType": "builder",
		"builder": map[string]interface{}{
			"queryData":     qd,
			"queryFormulas": []interface{}{},
		},
		"promql": promql,
		"clickhouse_sql": []map[string]interface{}{{
			"disabled": false, "legend": "", "name": "A", "query": "",
		}},
		"id":            queryID,
		"_grafanaExprs": collectExprs(ts, rules.QueryReplacements),
	}
}

func buildFilterItems(ms []labelMatcher) []interface{} {
	out := make([]interface{}, 0, len(ms))
	for _, m := range ms {
		op := m.Op
		switch op {
		case "=~":
			op = "regex"
		case "!~":
			op = "nregex"
		default:
			// keep = or !=
		}
		out = append(out, map[string]interface{}{
			"id": fmt.Sprintf("f_%s", m.Key),
			"key": map[string]interface{}{
				"dataType": "string",
				"id":       fmt.Sprintf("%s--string--tag--true", m.Key),
				"isColumn": true,
				"isJSON":   false,
				"key":      m.Key,
				"type":     "tag",
			},
			"op":    op,
			"value": toSigNozTmpl(m.Value),
		})
	}
	return out
}

func buildGroupBy(p promQL, legend string) []interface{} {
	// Prefer explicit by() labels; otherwise infer from legend placeholders
	labels := map[string]bool{}
	for _, b := range p.By {
		labels[b] = true
	}
	for _, ph := range legendPlaceholders(legend) {
		labels[ph] = true
	}
	// If none collected but we have label matchers with template values, group by those labels
	if len(labels) == 0 {
		for _, m := range p.Labels {
			if looksLikeTemplate(m.Value) {
				labels[m.Key] = true
			}
		}
	}
	out := make([]interface{}, 0, len(labels))
	for k := range labels {
		out = append(out, map[string]interface{}{
			"dataType": "string",
			"id":       fmt.Sprintf("%s--string--tag--true", k),
			"isColumn": true,
			"isJSON":   false,
			"key":      k,
			"type":     "tag",
		})
	}
	return out
}

func pickAggOperator(p promQL) string {
	if p.Agg != "" {
		return p.Agg
	}
	// default to avg across time buckets
	return "avg"
}

func pickTimeAggregation(p promQL) string {
	if p.Func == "rate" || p.Func == "irate" || p.Func == "increase" {
		return "rate"
	}
	if p.Func == "histogram_quantile" {
		return pickTimeAggregation(promQL{Func: innerFuncForHist(p)})
	}
	return "avg"
}

func guessMetricType(p promQL) string {
	if p.Func == "rate" || p.Func == "irate" || p.Func == "increase" {
		return "Counter"
	}
	if p.Func == "histogram_quantile" {
		if p.InnerFunc == "rate" || p.InnerFunc == "irate" || p.InnerFunc == "increase" {
			return "Counter"
		}
	}
	return "Gauge"
}

func parsePromQL(expr string) promQL {
	e := strings.TrimSpace(expr)
	e = stripOuterParens(e)
	e = stripArithmetic(e)
	// handle comparisons with optional 'bool': <expr> <op> [bool] <number>
	if m := regexp.MustCompile(`^(.*)\s*(>=|<=|==|!=|>|<)\s*(?:bool\s+)?([0-9]+(?:\.[0-9]+)?)$`).FindStringSubmatch(e); len(m) == 4 {
		left := strings.TrimSpace(m[1])
		inner := parsePromQL(left)
		inner.CmpOp = m[2]
		inner.CmpRight = m[3]
		inner.CmpIsBool = strings.Contains(e, " bool ")
		return inner
	}
	// handle trailing offset: <expr> offset <duration>
	if i := strings.LastIndex(e, " offset "); i > 0 {
		off := strings.TrimSpace(e[i+8:])
		base := strings.TrimSpace(e[:i])
		inner := parsePromQL(base)
		inner.Offset = off
		return inner
	}
	// histogram_quantile(q, expr)
	if m := regexp.MustCompile(`^histogram_quantile\s*\(\s*([0-9]*\.?[0-9]+)\s*,\s*(.+)\)$`).FindStringSubmatch(e); len(m) == 3 {
		inner := parsePromQL(m[2])
		// wrap into histogram_quantile while preserving inner
		inner.InnerFunc = inner.Func
		inner.Func = "histogram_quantile"
		inner.Quantile = m[1]
		// Ensure groupBy includes 'le' (typical for histogram buckets)
		if !contains(inner.By, "le") {
			inner.By = append(inner.By, "le")
		}
		return inner
	}
	// aggregator with by(...) before or after
	reAggA := regexp.MustCompile(`^(sum|avg|min|max|count)\s*(?:by\s*\(\s*([^\)]*)\s*\))?\s*\(\s*(.+)\s*\)$`)
	if m := reAggA.FindStringSubmatch(e); len(m) == 4 {
		inner := parsePromQL(m[3])
		inner.Agg = m[1]
		if m[2] != "" {
			inner.By = splitCSV(m[2])
		}
		return inner
	}
	reAggB := regexp.MustCompile(`^(sum|avg|min|max|count)\s*\(\s*(.+)\s*\)\s*by\s*\(\s*([^\)]*)\s*\)\s*$`)
	if m := reAggB.FindStringSubmatch(e); len(m) == 4 {
		inner := parsePromQL(m[2])
		inner.Agg = m[1]
		if m[3] != "" {
			inner.By = splitCSV(m[3])
		}
		return inner
	}
	// rate/irate/increase
	reFn := regexp.MustCompile(`^(rate|irate|increase)\s*\(\s*([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(\{[^}]*\})?\s*(\[[^\]]+\])?\s*\)\s*$`)
	if m := reFn.FindStringSubmatch(e); len(m) == 5 {
		return promQL{
			Metric: nonEmpty(m[2], ""),
			Labels: parseLabelSet(m[3]),
			Func:   m[1],
			Range:  strings.TrimSpace(m[4]),
		}
	}
	// plain metric with optional selectors and range
	reSimple := regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(\{[^}]*\})?\s*(\[[^\]]+\])?\s*$`)
	if m := reSimple.FindStringSubmatch(e); len(m) >= 2 {
		return promQL{Metric: m[1], Labels: parseLabelSet(m[2]), Range: strings.TrimSpace(m[3])}
	}
	// Fallback: unknown, return as-is metric name best-effort
	return promQL{Metric: e}
}

func innerFuncForHist(p promQL) string {
	// Try to guess inner function's time aggregation
	if p.Func == "histogram_quantile" {
		return "rate"
	}
	return p.Func
}

func buildFunctions(p promQL) []interface{} {
	funcs := []interface{}{}
	if p.Func == "histogram_quantile" {
		funcs = append(funcs, map[string]interface{}{
			"name": "histogram_quantile",
			"args": map[string]interface{}{"q": p.Quantile, "leLabel": "le"},
		})
	}
	if p.Offset != "" {
		funcs = append(funcs, map[string]interface{}{
			"name": "offset",
			"args": map[string]interface{}{"duration": p.Offset},
		})
	}
	return funcs
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func stripOuterParens(s string) string {
	for {
		if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
			return s
		}
		// check balance
		depth := 0
		balanced := true
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth < 0 {
					balanced = false
				}
			}
		}
		if !balanced || depth != 0 {
			return s
		}
		// safe to strip one level
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
}

func stripArithmetic(s string) string {
	// Remove simple scalar arithmetic at top-level, e.g.:
	//  irate(metric{...}[5m]) * 100  -> irate(metric{...}[5m])
	//  1024 / rate(x[1m])          -> rate(x[1m])
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '*', '/', '+', '-':
			if depth == 0 {
				left := strings.TrimSpace(s[:i])
				right := strings.TrimSpace(s[i+1:])
				if isNumber(right) {
					return stripOuterParens(left)
				}
				if isNumber(left) {
					return stripOuterParens(right)
				}
			}
		}
	}
	return s
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '.' {
			return false
		}
	}
	return true
}

func parseLabelSet(s string) []labelMatcher {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		s = s[1 : len(s)-1]
	}
	parts := splitCSV(s)
	out := make([]labelMatcher, 0, len(parts))
	re := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_\.:]*)\s*(=~|!~|=|!=)\s*"(.*)"\s*$`)
	for _, p := range parts {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if m := re.FindStringSubmatch(p); len(m) == 4 {
			out = append(out, labelMatcher{Key: m[1], Op: m[2], Value: m[3]})
		}
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	cur := strings.Builder{}
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuote = !inQuote
			cur.WriteByte(c)
			continue
		}
		if c == ',' && !inQuote {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		out = append(out, strings.TrimSpace(cur.String()))
	}
	return out
}

func legendPlaceholders(legend string) []string {
	// extract {{label}} placeholders
	var out []string
	re := regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_\.:]*)\s*\}\}`)
	for _, m := range re.FindAllStringSubmatch(legend, -1) {
		out = append(out, m[1])
	}
	return out
}

func looksLikeTemplate(v string) bool {
	return strings.Contains(v, "$") || strings.Contains(v, "{{")
}

func toSigNozTmpl(v string) string {
	// $var or ${var} -> {{.var}}
	// keep raw literals otherwise
	re := regexp.MustCompile(`\$\{?([a-zA-Z0-9_\.]+)\}?`)
	return re.ReplaceAllString(v, `{{.$1}}`)
}
