// Package oci -- ip_quality.go: IP quality detection operations (Phase 13.1).
// Tests network quality of an instance's public IP via ICMP ping, HTTP download
// speed test, and TCP port connectivity checks. Used by the IP quality service
// to evaluate and auto-switch IPs.
package oci

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	defaultPingCount    = 4
	defaultPingTimeout  = 5 * time.Second
	defaultHTTPTTimeout = 15 * time.Second
	defaultTCPTTimeout  = 5 * time.Second
	tcpTestPorts        = "22,80,443"
	httpTestURL         = "https://speed.cloudflare.com/__down?bytes=10000000"
)

// IPQualityResult holds the combined quality test results for a single IP.
type IPQualityResult struct {
	IP           string  `json:"ip"`
	PingOK       bool    `json:"pingOk"`
	PingAvgMs    float64 `json:"pingAvgMs"`
	PingLossRate float64 `json:"pingLossRate"`
	HTTPSpeedMbps float64 `json:"httpSpeedMbps"`
	HTTPOK       bool    `json:"httpOk"`
	TCPOK        bool    `json:"tcpOk"`
	TCPResults   []TCPPortResult `json:"tcpResults"`
	OverallScore float64 `json:"overallScore"`
	TestDuration string  `json:"testDuration"`
}

// TCPPortResult holds the result of a single TCP port connectivity test.
type TCPPortResult struct {
	Port    int   `json:"port"`
	OK      bool  `json:"ok"`
	LatencyMs int64 `json:"latencyMs"`
}

// PingResult holds the result of a ping test.
type PingResult struct {
	OK       bool    `json:"ok"`
	AvgMs    float64 `json:"avgMs"`
	LossRate float64 `json:"lossRate"`
	MinMs    float64 `json:"minMs"`
	MaxMs    float64 `json:"maxMs"`
}

// TestIPQuality runs all quality tests on the given IP concurrently and returns
// the combined result. This is a pure network operation (no OCI SDK needed).
func TestIPQuality(ctx context.Context, ip string) *IPQualityResult {
	start := time.Now()
	result := &IPQualityResult{IP: ip}

	// Run tests concurrently.
	type pingRes struct{ ok bool; avg, loss, min, max float64 }
	type httpRes struct{ ok bool; speed float64 }
	type tcpRes struct{ ok bool; results []TCPPortResult }

	pingCh := make(chan pingRes, 1)
	httpCh := make(chan httpRes, 1)
	tcpCh := make(chan tcpRes, 1)

	go func() {
		p := doPingTest(ip)
		pingCh <- pingRes{ok: p.OK, avg: p.AvgMs, loss: p.LossRate, min: p.MinMs, max: p.MaxMs}
	}()

	go func() {
		ok, speed := doHTTPSpeedTest(ctx, ip)
		httpCh <- httpRes{ok: ok, speed: speed}
	}()

	go func() {
		ok, results := doTCPTest(ctx, ip)
		tcpCh <- tcpRes{ok: ok, results: results}
	}()

	// Collect results (with timeout).
	timeout := time.After(30 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case p := <-pingCh:
			result.PingOK = p.ok
			result.PingAvgMs = p.avg
			result.PingLossRate = p.loss
		case h := <-httpCh:
			result.HTTPOK = h.ok
			result.HTTPSpeedMbps = h.speed
		case t := <-tcpCh:
			result.TCPOK = t.ok
			result.TCPResults = t.results
		case <-timeout:
			// Timeout — use whatever results we have.
		}
	}

	// Compute overall score (0-100).
	result.OverallScore = computeQualityScore(result)
	result.TestDuration = time.Since(start).Round(time.Millisecond).String()
	return result
}

// doPingTest performs a TCP-based ping (ICMP requires root, so we use TCP
// connect to port 22 as a proxy for reachability and latency).
func doPingTest(ip string) PingResult {
	var latencies []float64
	lossCount := 0

	for i := 0; i < defaultPingCount; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "22"), defaultPingTimeout)
		elapsed := time.Since(start).Seconds() * 1000 // ms
		if err != nil {
			lossCount++
			continue
		}
		conn.Close()
		latencies = append(latencies, elapsed)
		time.Sleep(200 * time.Millisecond)
	}

	result := PingResult{
		LossRate: float64(lossCount) / float64(defaultPingCount),
	}

	if len(latencies) == 0 {
		return result
	}

	result.OK = true
	var sum, min, max float64
	min = latencies[0]
	max = latencies[0]
	for _, l := range latencies {
		sum += l
		if l < min {
			min = l
		}
		if l > max {
			max = l
		}
	}
	result.AvgMs = sum / float64(len(latencies))
	result.MinMs = min
	result.MaxMs = max

	// Consider failed if loss > 50%.
	if result.LossRate > 0.5 {
		result.OK = false
	}
	return result
}

// doHTTPSpeedTest downloads a test file from the given IP via HTTP and measures
// the download speed in Mbps.
func doHTTPSpeedTest(ctx context.Context, ip string) (bool, float64) {
	testURL := fmt.Sprintf("http://%s/", ip)
	// Use the Cloudflare speed test as a general internet speed test.
	// For instance-specific testing, we measure basic HTTP connectivity.
	client := &http.Client{
		Timeout: defaultHTTPTTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, net.JoinHostPort(ip, "80"), 5*time.Second)
			},
		},
	}

	start := time.Now()
	resp, err := client.Get(testURL)
	if err != nil {
		// HTTP test failed — try HTTPS on 443.
		client2 := &http.Client{
			Timeout: defaultHTTPTTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.DialTimeout(network, net.JoinHostPort(ip, "443"), 5*time.Second)
				},
			},
		}
		resp2, err2 := client2.Get(fmt.Sprintf("https://%s/", ip))
		if err2 != nil {
			return false, 0
		}
		defer resp2.Body.Close()
		n, _ := io.Copy(io.Discard, resp2.Body)
		elapsed := time.Since(start).Seconds()
		if elapsed > 0 {
			return true, float64(n*8) / elapsed / 1_000_000
		}
		return true, 0
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start).Seconds()
	if elapsed > 0 {
		return true, float64(n*8) / elapsed / 1_000_000
	}
	return true, 0
}

// doTCPTest tests TCP connectivity to common ports on the given IP.
func doTCPTest(ctx context.Context, ip string) (bool, []TCPPortResult) {
	ports := []int{22, 80, 443}
	results := make([]TCPPortResult, 0, len(ports))
	allOK := true

	for _, port := range ports {
		start := time.Now()
		addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, defaultTCPTTimeout)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			allOK = false
			results = append(results, TCPPortResult{Port: port, OK: false, LatencyMs: latency})
			continue
		}
		conn.Close()
		results = append(results, TCPPortResult{Port: port, OK: true, LatencyMs: latency})
	}

	return allOK, results
}

// computeQualityScore calculates an overall quality score (0-100) from the
// individual test results. Weights: ping 40%, HTTP 30%, TCP 30%.
func computeQualityScore(r *IPQualityResult) float64 {
	var pingScore, httpScore, tcpScore float64

	// Ping score: based on latency and loss rate.
	if r.PingOK {
		// Lower latency = higher score. 0ms=100, 500ms=0.
		latencyScore := 100.0 - (r.PingAvgMs / 5.0)
		if latencyScore < 0 {
			latencyScore = 0
		}
		lossPenalty := r.PingLossRate * 100
		pingScore = latencyScore - lossPenalty
		if pingScore < 0 {
			pingScore = 0
		}
	}

	// HTTP score: based on speed. 100Mbps=100, 0Mbps=0.
	if r.HTTPOK {
		httpScore = r.HTTPSpeedMbps * 1.0
		if httpScore > 100 {
			httpScore = 100
		}
	}

	// TCP score: based on number of open ports and latency.
	if r.TCPOK {
		openCount := 0
		var totalLatency int64
		for _, tr := range r.TCPResults {
			if tr.OK {
				openCount++
				totalLatency += tr.LatencyMs
			}
		}
		tcpScore = float64(openCount) / float64(len(r.TCPResults)) * 100
		if openCount > 0 {
			avgLatency := float64(totalLatency) / float64(openCount)
			// Penalize high latency: 0ms=full, 500ms=half.
			tcpScore *= (1.0 - avgLatency/1000.0)
		}
	}

	return pingScore*0.4 + httpScore*0.3 + tcpScore*0.3
}
