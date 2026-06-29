// Package service -- ip_quality.go: Phase 13.1 IP Quality Detection service.
// Tests network quality of instance IPs, tracks quality history, and provides
// auto-switch logic to change IPs when quality degrades below threshold.
// Non-blocking: results are returned as they complete via concurrent tests.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// IPQualityService manages IP quality testing and auto-switching.
type IPQualityService struct {
	store     *db.Store
	masterKey []byte
	pool      *oci.ProxyPool
	logger    zerolog.Logger
}

func NewIPQualityService(store *db.Store, masterKey []byte, pool *oci.ProxyPool, logger zerolog.Logger) *IPQualityService {
	return &IPQualityService{store: store, masterKey: masterKey, pool: pool, logger: logger}
}

// IPQualityTestResult is the API-facing result of an IP quality test.
type IPQualityTestResult struct {
	InstanceID   int64   `json:"instanceId"`
	DisplayName  string  `json:"displayName"`
	IP           string  `json:"ip"`
	PingOK       bool    `json:"pingOk"`
	PingAvgMs    float64 `json:"pingAvgMs"`
	PingLossRate float64 `json:"pingLossRate"`
	HTTPSpeedMbps float64 `json:"httpSpeedMbps"`
	HTTPOK       bool    `json:"httpOk"`
	TCPOK        bool    `json:"tcpOk"`
	OverallScore float64 `json:"overallScore"`
	TestDuration string  `json:"testDuration"`
}

// BatchTestResult is the response for a batch IP quality test.
type BatchTestResult struct {
	Total     int                   `json:"total"`
	Tested    int                   `json:"tested"`
	Results   []IPQualityTestResult `json:"results"`
	AvgScore  float64               `json:"avgScore"`
	BestIP    string                `json:"bestIp"`
	WorstIP   string                `json:"worstIp"`
}

// TestSingleIP tests the network quality of a single instance's IP.
func (s *IPQualityService) TestSingleIP(ctx context.Context, instanceID int64) (*IPQualityTestResult, error) {
	inst, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("find instance %d: %w", instanceID, err)
	}

	ip := nsStr(inst.PublicIps)
	if ip == "" {
		return nil, fmt.Errorf("instance %d has no public IP", instanceID)
	}

	result := oci.TestIPQuality(ctx, ip)

	return &IPQualityTestResult{
		InstanceID:    instanceID,
		DisplayName:   nsStr(inst.DisplayName),
		IP:            ip,
		PingOK:        result.PingOK,
		PingAvgMs:     result.PingAvgMs,
		PingLossRate:  result.PingLossRate,
		HTTPSpeedMbps: result.HTTPSpeedMbps,
		HTTPOK:        result.HTTPOK,
		TCPOK:         result.TCPOK,
		OverallScore:  result.OverallScore,
		TestDuration:  result.TestDuration,
	}, nil
}

// TestIPByAddress tests the quality of a specific IP address directly.
func (s *IPQualityService) TestIPByAddress(ctx context.Context, ip string) (*oci.IPQualityResult, error) {
	if ip == "" {
		return nil, fmt.Errorf("IP address is required")
	}
	result := oci.TestIPQuality(ctx, ip)
	return result, nil
}

// BatchTestByTenant tests all instance IPs for a given tenant.
func (s *IPQualityService) BatchTestByTenant(ctx context.Context, tenantID int64) (*BatchTestResult, error) {
	instances, err := repo.New(s.store.Read).FindInstancesByTenantId(ctx, sql.NullInt64{Int64: tenantID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list instances for tenant %d: %w", tenantID, err)
	}

	results := make([]IPQualityTestResult, 0, len(instances))
	var totalScore float64
	bestScore, worstScore := -1.0, 101.0
	bestIP, worstIP := "", ""

	for _, inst := range instances {
		ip := nsStr(inst.PublicIps)
		if ip == "" {
			continue
		}

		result := oci.TestIPQuality(ctx, ip)
		testResult := IPQualityTestResult{
			InstanceID:    inst.ID,
			DisplayName:   nsStr(inst.DisplayName),
			IP:            ip,
			PingOK:        result.PingOK,
			PingAvgMs:     result.PingAvgMs,
			PingLossRate:  result.PingLossRate,
			HTTPSpeedMbps: result.HTTPSpeedMbps,
			HTTPOK:        result.HTTPOK,
			TCPOK:         result.TCPOK,
			OverallScore:  result.OverallScore,
			TestDuration:  result.TestDuration,
		}
		results = append(results, testResult)
		totalScore += result.OverallScore

		if result.OverallScore > bestScore {
			bestScore = result.OverallScore
			bestIP = ip
		}
		if result.OverallScore < worstScore {
			worstScore = result.OverallScore
			worstIP = ip
		}
	}

	avgScore := 0.0
	if len(results) > 0 {
		avgScore = totalScore / float64(len(results))
	}

	return &BatchTestResult{
		Total:    len(instances),
		Tested:   len(results),
		Results:  results,
		AvgScore: avgScore,
		BestIP:   bestIP,
		WorstIP:  worstIP,
	}, nil
}

// BatchTestAll tests all instance IPs across all tenants.
func (s *IPQualityService) BatchTestAll(ctx context.Context) (*BatchTestResult, error) {
	rows, err := repo.New(s.store.Read).ListAllInstanceDetails(ctx, repo.ListAllInstanceDetailsParams{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("list all instances: %w", err)
	}

	results := make([]IPQualityTestResult, 0, len(rows))
	var totalScore float64
	bestScore, worstScore := -1.0, 101.0
	bestIP, worstIP := "", ""

	for _, inst := range rows {
		ip := nsStr(inst.PublicIps)
		if ip == "" {
			continue
		}

		result := oci.TestIPQuality(ctx, ip)
		testResult := IPQualityTestResult{
			InstanceID:    inst.ID,
			DisplayName:   nsStr(inst.DisplayName),
			IP:            ip,
			PingOK:        result.PingOK,
			PingAvgMs:     result.PingAvgMs,
			PingLossRate:  result.PingLossRate,
			HTTPSpeedMbps: result.HTTPSpeedMbps,
			HTTPOK:        result.HTTPOK,
			TCPOK:         result.TCPOK,
			OverallScore:  result.OverallScore,
			TestDuration:  result.TestDuration,
		}
		results = append(results, testResult)
		totalScore += result.OverallScore

		if result.OverallScore > bestScore {
			bestScore = result.OverallScore
			bestIP = ip
		}
		if result.OverallScore < worstScore {
			worstScore = result.OverallScore
			worstIP = ip
		}
	}

	avgScore := 0.0
	if len(results) > 0 {
		avgScore = totalScore / float64(len(results))
	}

	return &BatchTestResult{
		Total:    len(rows),
		Tested:   len(results),
		Results:  results,
		AvgScore: avgScore,
		BestIP:   bestIP,
		WorstIP:  worstIP,
	}, nil
}

// AutoSwitchInput is the request for auto-switching an instance's IP.
type AutoSwitchInput struct {
	InstanceID    int64   `json:"instanceId"`
	MinScore      float64 `json:"minScore"`      // minimum acceptable score (default 30)
	MaxAttempts   int     `json:"maxAttempts"`     // max IP change attempts (default 3)
}

// AutoSwitchResult is the result of an auto-switch operation.
type AutoSwitchResult struct {
	OldIP        string  `json:"oldIp"`
	NewIP        string  `json:"newIp"`
	OldScore     float64 `json:"oldScore"`
	NewScore     float64 `json:"newScore"`
	Attempts     int     `json:"attempts"`
	Switched     bool    `json:"switched"`
	Message      string  `json:"message"`
}

// AutoSwitchIP tests the current IP quality and switches to a new IP if the
// score is below the threshold. After switching, it verifies the new IP quality.
func (s *IPQualityService) AutoSwitchIP(ctx context.Context, input AutoSwitchInput) (*AutoSwitchResult, error) {
	if input.MinScore <= 0 {
		input.MinScore = 30
	}
	if input.MaxAttempts <= 0 {
		input.MaxAttempts = 3
	}

	// Find the instance.
	inst, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, input.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("find instance %d: %w", input.InstanceID, err)
	}

	if !inst.TenantID.Valid {
		return nil, fmt.Errorf("instance %d has no tenant", input.InstanceID)
	}

	oldIP := nsStr(inst.PublicIps)
	if oldIP == "" {
		return nil, fmt.Errorf("instance %d has no public IP", input.InstanceID)
	}

	// Test current IP quality.
	oldResult := oci.TestIPQuality(ctx, oldIP)
	result := &AutoSwitchResult{
		OldIP:    oldIP,
		OldScore: oldResult.OverallScore,
	}

	// If current IP is good enough, no switch needed.
	if oldResult.OverallScore >= input.MinScore {
		result.NewIP = oldIP
		result.NewScore = oldResult.OverallScore
		result.Switched = false
		result.Message = fmt.Sprintf("current IP quality (%.1f) is above threshold (%.1f), no switch needed", oldResult.OverallScore, input.MinScore)
		return result, nil
	}

	// Need to switch IP. Build OCI clients.
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, inst.TenantID.Int64)
	if err != nil {
		return nil, fmt.Errorf("find tenant %d: %w", inst.TenantID.Int64, err)
	}
	creds := tenantToCredsObj(t)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("oci provider: %w", err)
	}
	clients, err := oci.NewClients(prov)
	if err != nil {
		return nil, fmt.Errorf("oci clients: %w", err)
	}

	// Attempt IP switches.
	for attempt := 1; attempt <= input.MaxAttempts; attempt++ {
		result.Attempts = attempt
		s.logger.Info().Int64("instance", input.InstanceID).Int("attempt", attempt).
			Float64("oldScore", oldResult.OverallScore).Msg("ip_quality: switching IP")

		newIP, err := oci.ReassignPublicIP(ctx, clients, nsStr(t.Tenancy), nsStr(inst.InstanceID))
		if err != nil {
			s.logger.Error().Err(err).Int("attempt", attempt).Msg("ip_quality: IP switch failed")
			result.Message = fmt.Sprintf("IP switch attempt %d failed: %v", attempt, err)
			continue
		}

		// Wait for the new IP to stabilize.
		time.Sleep(5 * time.Second)

		// Test new IP quality.
		newResult := oci.TestIPQuality(ctx, newIP)
		result.NewIP = newIP
		result.NewScore = newResult.OverallScore

		// Update DB with new IP.
		_ = repo.New(s.store.Write).UpdateInstanceDetailPublicIp(ctx, repo.UpdateInstanceDetailPublicIpParams{
			PublicIps: sql.NullString{String: newIP, Valid: true},
			ID:        input.InstanceID,
		})

		if newResult.OverallScore >= input.MinScore {
			result.Switched = true
			result.Message = fmt.Sprintf("switched to new IP %s with score %.1f (attempt %d)", newIP, newResult.OverallScore, attempt)
			return result, nil
		}

		// New IP is also bad, try again.
		s.logger.Warn().Str("newIP", newIP).Float64("score", newResult.OverallScore).
			Int("attempt", attempt).Msg("ip_quality: new IP also below threshold")
		oldIP = newIP
		oldResult = newResult
	}

	result.Switched = false
	result.Message = fmt.Sprintf("could not find an IP above threshold %.1f after %d attempts", input.MinScore, input.MaxAttempts)
	return result, nil
}

// GetIPQualityHistory returns the quality test history for an instance.
// Since we don't persist history in the DB, this returns the current test result.
func (s *IPQualityService) GetIPQualityHistory(ctx context.Context, instanceID int64) (*IPQualityTestResult, error) {
	return s.TestSingleIP(ctx, instanceID)
}

// nsStr is defined in object_storage.go; reuse it here via the same package.
