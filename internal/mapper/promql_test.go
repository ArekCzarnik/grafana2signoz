package mapper

import "testing"

func TestParsePromQL_Simple(t *testing.T) {
	p := parsePromQL(`nodejs_eventloop_lag_seconds{instance=~"$instance"}`)
	if p.Metric != "nodejs_eventloop_lag_seconds" {
		t.Fatalf("metric: %v", p.Metric)
	}
	if len(p.Labels) != 1 || p.Labels[0].Key != "instance" || p.Labels[0].Op != "=~" {
		t.Fatalf("labels: %+v", p.Labels)
	}
}

func TestParsePromQL_Rate(t *testing.T) {
	p := parsePromQL(`rate(http_requests_total{code="200"}[5m])`)
	if p.Func != "rate" || p.Metric != "http_requests_total" {
		t.Fatalf("parse: %+v", p)
	}
}

func TestParsePromQL_SumByRate(t *testing.T) {
	p := parsePromQL(`sum by (method,status) (rate(http_requests_total{service=~"$s"}[5m]))`)
	if p.Agg != "sum" {
		t.Fatalf("agg=%s", p.Agg)
	}
	if len(p.By) != 2 {
		t.Fatalf("by=%v", p.By)
	}
	if p.Func != "rate" {
		t.Fatalf("func=%s", p.Func)
	}
}

func TestParsePromQL_HistogramQuantile(t *testing.T) {
	p := parsePromQL(`histogram_quantile(0.95, sum by (le) (rate(http_server_duration_seconds_bucket[5m])) )`)
	if p.Func != "histogram_quantile" || p.Quantile != "0.95" {
		t.Fatalf("p=%+v", p)
	}
	if p.Metric != "http_server_duration_seconds_bucket" {
		t.Fatalf("metric=%s", p.Metric)
	}
}

func TestParsePromQL_OffsetAndCmp(t *testing.T) {
	p := parsePromQL(`rate(foo_total[5m]) offset 1m > bool 0`)
	if p.Offset != "1m" || p.CmpOp != ">" || p.CmpRight != "0" || !p.CmpIsBool {
		t.Fatalf("p=%+v", p)
	}
}
