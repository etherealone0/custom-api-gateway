// Command chaos-report queries the gateway's Prometheus instance after a
// chaos-demo run and renders the circuit-breaker/backend-health state
// transitions and request outcome counts observed during the run as a
// markdown timeline. See scripts/chaos-demo.sh for the orchestration this
// is meant to be run after.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

type queryRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string       `json:"resultType"`
		Result     []seriesData `json:"result"`
	} `json:"data"`
}

type seriesData struct {
	Metric map[string]string `json:"metric"`
	Values [][2]interface{}  `json:"values"`
}

type sample struct {
	ts  time.Time
	val float64
}

func queryRange(promURL, query string, start, end time.Time, step time.Duration) ([]seriesData, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", strconv.FormatFloat(step.Seconds(), 'f', -1, 64))

	reqURL := promURL + "/api/v1/query_range?" + q.Encode()
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed queryRangeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding prometheus response: %w (body: %s)", err, body)
	}
	if parsed.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", body)
	}
	return parsed.Data.Result, nil
}

func samplesOf(s seriesData) []sample {
	out := make([]sample, 0, len(s.Values))
	for _, v := range s.Values {
		tsFloat, ok := v[0].(float64)
		if !ok {
			continue
		}
		valStr, ok := v[1].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		out = append(out, sample{ts: time.Unix(int64(tsFloat), 0), val: val})
	}
	return out
}

func circuitStateName(v float64) string {
	switch int(v) {
	case 0:
		return "closed"
	case 1:
		return "open"
	case 2:
		return "half-open"
	default:
		return fmt.Sprintf("unknown(%v)", v)
	}
}

func healthStateName(v float64) string {
	if v == 1 {
		return "healthy"
	}
	return "unhealthy"
}

type transition struct {
	offset  time.Duration
	backend string
	kind    string
	from    string
	to      string
}

func findTransitions(series []seriesData, kind string, nameFn func(float64) string, windowStart time.Time) []transition {
	var out []transition
	for _, s := range series {
		backend := s.Metric["backend"]
		samples := samplesOf(s)
		if len(samples) == 0 {
			continue
		}
		prev := samples[0].val
		for _, sm := range samples[1:] {
			if sm.val != prev {
				out = append(out, transition{
					offset:  sm.ts.Sub(windowStart).Round(time.Second),
					backend: backend,
					kind:    kind,
					from:    nameFn(prev),
					to:      nameFn(sm.val),
				})
				prev = sm.val
			}
		}
	}
	return out
}

func main() {
	promURL := flag.String("prom", "http://localhost:9090", "Prometheus base URL")
	minutes := flag.Float64("minutes", 5, "how many minutes to look back from now")
	step := flag.Duration("step", 2*time.Second, "query resolution step")
	out := flag.String("out", "docs/chaos-demo-report.md", "output markdown file")
	flag.Parse()

	end := time.Now()
	start := end.Add(-time.Duration(*minutes * float64(time.Minute)))

	cbSeries, err := queryRange(*promURL, "gateway_circuit_breaker_state", start, end, *step)
	if err != nil {
		log.Fatalf("circuit breaker query: %v", err)
	}
	healthSeries, err := queryRange(*promURL, "gateway_backend_health", start, end, *step)
	if err != nil {
		log.Fatalf("backend health query: %v", err)
	}
	reqSeries, err := queryRange(*promURL, `gateway_requests_total{path="/api/orders"}`, start, end, *step)
	if err != nil {
		log.Fatalf("requests query: %v", err)
	}

	var transitions []transition
	transitions = append(transitions, findTransitions(cbSeries, "circuit breaker", circuitStateName, start)...)
	transitions = append(transitions, findTransitions(healthSeries, "health check", healthStateName, start)...)
	sort.Slice(transitions, func(i, j int) bool { return transitions[i].offset < transitions[j].offset })

	statusTotals := map[string]float64{}
	for _, s := range reqSeries {
		samples := samplesOf(s)
		if len(samples) == 0 {
			continue
		}
		status := s.Metric["status"]
		delta := samples[len(samples)-1].val - samples[0].val
		statusTotals[status] += delta
	}
	var statuses []string
	for st := range statusTotals {
		statuses = append(statuses, st)
	}
	sort.Strings(statuses)

	f, err := os.Create(*out)
	if err != nil {
		log.Fatalf("creating output file: %v", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "# Chaos Demo Report\n\n")
	fmt.Fprintf(f, "Generated %s, covering the window %s to %s (%.0f minutes).\n\n",
		end.Format(time.RFC3339), start.Format(time.RFC3339), end.Format(time.RFC3339), *minutes)

	fmt.Fprintf(f, "## State Transition Timeline\n\n")
	if len(transitions) == 0 {
		fmt.Fprintf(f, "No state transitions observed in this window. Re-run with a wider `-minutes` window or check that the chaos-demo script actually killed a backend.\n\n")
	} else {
		fmt.Fprintf(f, "| t+ (s) | Component | Backend | From | To |\n")
		fmt.Fprintf(f, "|--------|-----------|---------|------|----|\n")
		for _, t := range transitions {
			fmt.Fprintf(f, "| %.0f | %s | %s | %s | %s |\n", t.offset.Seconds(), t.kind, t.backend, t.from, t.to)
		}
		fmt.Fprintf(f, "\n")
	}

	fmt.Fprintf(f, "## Request Outcomes on /api/orders\n\n")
	fmt.Fprintf(f, "| Status | Count |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	for _, st := range statuses {
		fmt.Fprintf(f, "| %s | %.0f |\n", st, statusTotals[st])
	}

	fmt.Printf("Report written to %s\n", *out)
}
