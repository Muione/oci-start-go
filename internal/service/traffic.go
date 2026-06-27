// Package service — traffic.go: instance traffic monitoring service (Phase 5).
// Port of InstanceTrafficTask.java. Queries OCI Monitoring API in real-time
// for tenant traffic stats, checks thresholds, and sends alerts.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// TrafficSvc monitors instance traffic via OCI Monitoring API.
type TrafficSvc struct {
	store     *db.Store
	masterKey []byte
	logger    zerolog.Logger
	notifier  notify.Notifier
	running   atomic.Bool
}

func NewTrafficSvc(store *db.Store, masterKey []byte, logger zerolog.Logger, notifier notify.Notifier) *TrafficSvc {
	return &TrafficSvc{store: store, masterKey: masterKey, logger: logger, notifier: notifier}
}

// TenantTrafficStats holds the monthly traffic summary for one tenant.
type TenantTrafficStats struct {
	TenantID          int64             `json:"tenantId"`
	Tenancy           string            `json:"tenancy"`
	TenancyName       string            `json:"tenancyName"`
	Region            string            `json:"region"`
	TotalEgressGB     float64           `json:"totalEgressGB"`
	ThresholdGB       float64           `json:"thresholdGB"`
	StatisticsEnabled bool              `json:"statisticsEnabled"`
	AutoShutdown      bool              `json:"autoShutdown"`
	Success           bool              `json:"success"`
	Message           string            `json:"message,omitempty"`
	Instances         []InstanceTraffic `json:"instances,omitempty"`
}

// InstanceTraffic holds per-instance traffic data.
type InstanceTraffic struct {
	InstanceID   string  `json:"instanceId"`
	InstanceName string  `json:"instanceName"`
	PublicIP     string  `json:"publicIp"`
	VnicCount    int     `json:"vnicCount"`
	EgressGB     float64 `json:"egressGB"`
}

// CheckAllTenantsTraffic is the main entry point for the traffic monitoring
// cron job (every 30 min). Single-flight guard prevents overlap.
func (s *TrafficSvc) CheckAllTenantsTraffic(ctx context.Context) {
	if !s.running.CompareAndSwap(false, true) {
		s.logger.Debug().Msg("traffic: previous check still running, skipping")
		return
	}
	defer s.running.Store(false)

	start := time.Now()
	deadline := start.Add(25 * time.Minute)

	alerts, err := repo.New(s.store.Read).ListTrafficAlerts(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("traffic: list alerts failed")
		return
	}

	for _, alert := range alerts {
		if time.Now().After(deadline) {
			s.logger.Warn().Msg("traffic: time budget exceeded, stopping")
			break
		}
		if alert.StatisticsEnabled == 0 {
			continue
		}
		tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, alert.TenantID)
		if err != nil {
			continue
		}
		s.checkTenantTraffic(ctx, tenant, alert)
	}

	s.logger.Debug().Dur("elapsed", time.Since(start)).Int("checks", len(alerts)).Msg("traffic: check complete")
}

func (s *TrafficSvc) checkTenantTraffic(ctx context.Context, tenant repo.Tenant, alert repo.TrafficAlert) {
	stats := s.QueryTenantTraffic(ctx, tenant)
	if !stats.Success || len(stats.Instances) == 0 {
		return
	}

	threshold := alert.Threshold
	if threshold <= 0 {
		return // no threshold configured
	}

	if stats.TotalEgressGB > threshold {
		s.logger.Warn().
			Str("tenancy", stats.Tenancy).
			Float64("totalGB", stats.TotalEgressGB).
			Float64("threshold", threshold).
			Msg("traffic: threshold exceeded")

		first := stats.Instances[0]
		s.logger.Warn().
			Str("instanceName", first.InstanceName).
			Str("publicIp", first.PublicIP).
			Float64("totalGB", stats.TotalEgressGB).
			Float64("threshold", threshold).
			Msg("traffic: threshold exceeded")

		msg := fmt.Sprintf("[流量超限]\n租户: %s (%s)\n用量: %.2f GB\n阈值: %.2f GB\n实例: %s",
			stats.TenancyName, stats.Tenancy, stats.TotalEgressGB, threshold, first.InstanceName)
		if s.notifier != nil {
			if sendErr := s.notifier.Send(ctx, msg); sendErr != nil {
				s.logger.Warn().Err(sendErr).Msg("traffic: notify failed")
			}
		}

		if alert.AutoShutdown > 0 && first.InstanceID != "" {
			s.logger.Warn().
				Str("instanceId", first.InstanceID).
				Msg("traffic: auto-shutdown would be triggered (requires OCI instance terminate API)")
		}
	}
}

// QueryTenantTraffic queries the current month's egress traffic for all
// instances belonging to a tenant.
func (s *TrafficSvc) QueryTenantTraffic(ctx context.Context, tenant repo.Tenant) TenantTrafficStats {
	stats := TenantTrafficStats{
		TenantID:          tenant.ID,
		Tenancy:           ns(tenant.Tenancy),
		TenancyName:       ns(tenant.TenancyName),
		Region:            ns(tenant.Region),
		StatisticsEnabled: true,
		Success:           true,
	}

	alert, err := repo.New(s.store.Read).FindTrafficAlertByTenantId(ctx, tenant.ID)
	if err == nil {
		stats.ThresholdGB = alert.Threshold
		stats.AutoShutdown = alert.AutoShutdown > 0
	}

	instances, err := repo.New(s.store.Read).FindInstancesByTenantId(ctx, sql.NullInt64{Int64: tenant.ID, Valid: true})
	if err != nil || len(instances) == 0 {
		stats.Message = "no instances found"
		return stats
	}

	creds := tenantToCreds(tenant)
	prov, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		stats.Success = false
		stats.Message = fmt.Sprintf("provider error: %v", err)
		return stats
	}

	monClient, err := oci.BuildMonitoringClient(prov)
	if err != nil {
		stats.Success = false
		stats.Message = fmt.Sprintf("monitoring client: %v", err)
		return stats
	}

	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var totalEgress float64
	for _, inst := range instances {
		if !inst.InstanceID.Valid || !inst.CompartmentID.Valid {
			continue
		}

		vnics := []oci.VnicInfo{
			{
				VnicID:       ns(inst.VnicIds),
				InstanceID:   ns(inst.InstanceID),
				InstanceName: ns(inst.DisplayName),
				PublicIP:     ns(inst.PublicIps),
			},
		}

		egress, err := oci.GetInstanceTrafficTotal(ctx, monClient, ns(inst.CompartmentID),
			vnics, false, startOfMonth, now, oci.TrafficPeriodOneDay)
		if err != nil {
			continue
		}

		it := InstanceTraffic{
			InstanceID:   ns(inst.InstanceID),
			InstanceName: ns(inst.DisplayName),
			PublicIP:     ns(inst.PublicIps),
			VnicCount:    1,
			EgressGB:     oci.BytesToGB(egress),
		}
		stats.Instances = append(stats.Instances, it)
		totalEgress += egress
	}

	stats.TotalEgressGB = oci.BytesToGB(totalEgress)
	return stats
}

// SaveTrafficAlert creates or updates a traffic alert config.
func (s *TrafficSvc) SaveTrafficAlert(ctx context.Context, tenantID int64, threshold float64, autoShutdown, enabled, statisticsEnabled bool) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	tenant, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("find tenant: %w", err)
	}
	return repo.New(s.store.Write).UpsertTrafficAlert(ctx, repo.UpsertTrafficAlertParams{
		TenantID:           tenantID,
		Tenancy:            ns(tenant.Tenancy),
		Threshold:          threshold,
		AutoShutdown:       boolToInt(autoShutdown),
		NotificationEmail:  sql.NullString{},
		Enabled:            boolToInt(enabled),
		CloudType:          tenant.CloudType,
		StatisticsEnabled:  boolToInt(statisticsEnabled),
		CreatedAt:          now,
		UpdatedAt:          sql.NullString{String: now, Valid: true},
	})
}

// GetTrafficAlert returns the traffic alert config for a tenant.
func (s *TrafficSvc) GetTrafficAlert(ctx context.Context, tenantID int64) (*repo.TrafficAlert, error) {
	r, err := repo.New(s.store.Read).FindTrafficAlertByTenantId(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListTrafficAlerts returns all traffic alert configs.
func (s *TrafficSvc) ListTrafficAlerts(ctx context.Context) ([]repo.TrafficAlert, error) {
	return repo.New(s.store.Read).ListTrafficAlerts(ctx)
}
