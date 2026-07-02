// Package oci — audit.go: OCI Audit Log SDK wrapper (Phase 11.4).
// Parity with Java AuditLogUtils: query audit events by recent days or
// date range, extract user/IP/event info, paginated via pageToken.
package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/audit"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/rs/zerolog/log"
)

const (
	maxAuditDays     = 90
	defaultAuditDays = 1
	maxUserNameLen   = 35
	timeLayout       = "2006-01-02 15:04:05"
	dateLayout       = "2006-01-02"
)

// AuditEventDTO is a simplified audit event for the API response.
type AuditEventDTO struct {
	EventType      string `json:"eventType"`
	UserName       string `json:"userName"`
	UserType       string `json:"userType"`
	IPAddress      string `json:"ipAddress"`
	ClientEnv      string `json:"clientEnv"`
	EventTime      string `json:"eventTime"`
	ResponseStatus string `json:"responseStatus"`
}

// AuditLogPage is a paginated response of audit events.
type AuditLogPage struct {
	Data          []AuditEventDTO `json:"data"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
}

// ListAuditEvents queries audit events for a time range.
// Parity with AuditLogUtils.listAuditEvents.
//
// For each AuditEvent, extracts:
//   - data.identity.principalName -> userName (truncated to 35 chars + "...")
//   - data.identity.authType -> userType
//   - data.identity.ipAddress -> ipAddress (raw IP for v1)
//   - data.identity.userAgent -> clientEnv
//   - eventType
//   - eventTime -> formatted as "yyyy-MM-dd HH:mm:ss"
//   - data.response.status -> responseStatus
//
// Skips events where data or data.identity is null.
func ListAuditEvents(
	ctx context.Context,
	c Clients,
	compartmentID string,
	startTime, endTime time.Time,
	pageToken string,
) (*AuditLogPage, error) {
	req := audit.ListEventsRequest{
		CompartmentId:      &compartmentID,
		StartTime:          &common.SDKTime{Time: startTime},
		EndTime:            &common.SDKTime{Time: endTime},
	}
	if pageToken != "" {
		req.Page = &pageToken
	}

	resp, err := c.Audit.ListEvents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}

	events := make([]AuditEventDTO, 0, len(resp.Items))
	for _, ev := range resp.Items {
		dto := extractAuditEvent(ev)
		if dto != nil {
			events = append(events, *dto)
		}
	}

	page := &AuditLogPage{
		Data: events,
	}
	if resp.OpcNextPage != nil {
		page.NextPageToken = *resp.OpcNextPage
	}
	return page, nil
}

// ListRecentAuditEvents queries the past N days (1-90, clamped).
// Parity with AuditLogUtils.listRecentAuditEvents.
// OCI Audit API requires seconds/milliseconds to be 0 (minute granularity).
func ListRecentAuditEvents(
	ctx context.Context,
	c Clients,
	compartmentID string,
	days int,
	pageToken string,
) (*AuditLogPage, error) {
	if days < 1 {
		days = defaultAuditDays
	}
	if days > maxAuditDays {
		days = maxAuditDays
	}
	now := time.Now().UTC()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.UTC)
	startTime := endTime.AddDate(0, 0, -days)
	return ListAuditEvents(ctx, c, compartmentID, startTime, endTime, pageToken)
}

// ListAuditEventsByDateRange queries a specific date range (max 90 days).
// Parity with AuditLogUtils.listAuditEventsByDateRange.
// startDate/endDate format: "yyyy-MM-dd" (converted to UTC start/end of day).
func ListAuditEventsByDateRange(
	ctx context.Context,
	c Clients,
	compartmentID string,
	startDate, endDate string,
	pageToken string,
) (*AuditLogPage, error) {
	start, err := time.Parse(dateLayout, startDate)
	if err != nil {
		log.Warn().Err(err).Str("startDate", startDate).Msg("invalid audit start date")
		return &AuditLogPage{Data: []AuditEventDTO{}}, nil
	}
	start = start.UTC()

	end := start
	if endDate != "" {
		parsed, err := time.Parse(dateLayout, endDate)
		if err != nil {
			log.Warn().Err(err).Str("endDate", endDate).Msg("invalid audit end date")
			return &AuditLogPage{Data: []AuditEventDTO{}}, nil
		}
		end = parsed.UTC()
	}
	// End of day.
	end = end.Add(24*time.Hour - time.Second)

	if end.Before(start) {
		log.Warn().Str("start", startDate).Str("end", endDate).Msg("invalid date range: end before start")
		return &AuditLogPage{Data: []AuditEventDTO{}}, nil
	}
	if int(end.Sub(start).Hours()/24) > maxAuditDays {
		log.Warn().Str("start", startDate).Str("end", endDate).Msg("date range exceeds 90 days")
		return &AuditLogPage{Data: []AuditEventDTO{}}, nil
	}

	return ListAuditEvents(ctx, c, compartmentID, start, end, pageToken)
}

// extractAuditEvent extracts the DTO fields from an OCI AuditEvent.
// Returns nil if the event data or identity is missing.
func extractAuditEvent(ev audit.AuditEvent) *AuditEventDTO {
	if ev.Data == nil {
		return nil
	}
	data := ev.Data
	if data.Identity == nil {
		return nil
	}

	dto := &AuditEventDTO{
		EventType: nsAudit(data.EventName),
	}

	identity := data.Identity
	dto.UserName = truncateUserName(nsAudit(identity.PrincipalName))
	dto.UserType = nsAudit(identity.AuthType)
	dto.IPAddress = nsAudit(identity.IpAddress)
	dto.ClientEnv = nsAudit(identity.UserAgent)

	if ev.EventTime != nil {
		dto.EventTime = ev.EventTime.Format(timeLayout)
	}

	if data.Response != nil && data.Response.Status != nil {
		dto.ResponseStatus = *data.Response.Status
	}

	return dto
}

// nsAudit safely dereferences a *string, returning "" if nil.
func nsAudit(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// truncateUserName truncates the userName to 35 characters + "..." if longer.
func truncateUserName(name string) string {
	if len(name) > maxUserNameLen {
		return name[:maxUserNameLen] + "..."
	}
	return name
}
