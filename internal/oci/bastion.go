// Package oci -- bastion.go: Phase 14.1 OCI Bastion SDK wrapper.
// Parity with Java BastionSshSessionUtils: list bastions, get bastion,
// create/get/list/delete sessions (port-forwarding and managed SSH).
package oci

import (
	"context"
	"fmt"
	"net/http"

	"github.com/oracle/oci-go-sdk/v65/bastion"
	"github.com/oracle/oci-go-sdk/v65/common"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// BastionSummaryVO is a simplified bastion summary for the API response.
type BastionSummaryVO struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	BastionType          string `json:"bastionType"`
	LifecycleState       string `json:"lifecycleState"`
	CompartmentID        string `json:"compartmentId"`
	TargetVcnID          string `json:"targetVcnId"`
	MaxSessionTTL        int    `json:"maxSessionTtlInSeconds"`
	TimeCreated          string `json:"timeCreated,omitempty"`
	TimeUpdated          string `json:"timeUpdated,omitempty"`
}

// SessionVO is a simplified session for the API response.
type SessionVO struct {
	ID                 string            `json:"id"`
	BastionID          string            `json:"bastionId"`
	BastionName        string            `json:"bastionName"`
	SessionType        string            `json:"sessionType"`
	DisplayName        string            `json:"displayName"`
	LifecycleState     string            `json:"lifecycleState"`
	SessionTTL         int               `json:"sessionTtlInSeconds"`
	SshMetadata        map[string]string `json:"sshMetadata,omitempty"`
	BastionUserName    string            `json:"bastionUserName,omitempty"`
	TimeCreated        string            `json:"timeCreated,omitempty"`
	TimeUpdated        string            `json:"timeUpdated,omitempty"`
	LifecycleDetails   string            `json:"lifecycleDetails,omitempty"`
}

// CreateSessionInput is the service-layer input for creating a bastion session.
type CreateSessionInput struct {
	BastionID                         string `json:"bastionId"`
	SessionType                       string `json:"sessionType"` // PORT_FORWARDING or MANAGED_SSH
	DisplayName                       string `json:"displayName"`
	TargetResourcePrivateIPAddress    string `json:"targetResourcePrivateIpAddress"`
	TargetResourcePort                int    `json:"targetResourcePort"`
	TargetResourceID                  string `json:"targetResourceId"`
	TargetResourceOSUserName          string `json:"targetResourceOperatingSystemUserName"`
	SessionTTLInSeconds               int    `json:"sessionTtlInSeconds"`
	PublicKeyContent                  string `json:"publicKeyContent"`
}

// ---------------------------------------------------------------------------
// SDK wrappers
// ---------------------------------------------------------------------------

// ListBastions returns all bastions in a compartment, paginated.
func ListBastions(ctx context.Context, c *bastion.BastionClient, compartmentID string) ([]BastionSummaryVO, error) {
	var all []BastionSummaryVO
	var page *string

	for {
		resp, err := c.ListBastions(ctx, bastion.ListBastionsRequest{
			CompartmentId: common.String(compartmentID),
			Page:          page,
		})
		if err != nil {
			return nil, fmt.Errorf("bastion: list: %w", err)
		}
		for _, b := range resp.Items {
			all = append(all, bastionSummaryToVO(b))
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return all, nil
}

// GetBastion returns a single bastion by ID.
func GetBastion(ctx context.Context, c *bastion.BastionClient, bastionID string) (*BastionSummaryVO, error) {
	resp, err := c.GetBastion(ctx, bastion.GetBastionRequest{
		BastionId: common.String(bastionID),
	})
	if err != nil {
		return nil, fmt.Errorf("bastion: get: %w", err)
	}
	vo := bastionSummaryToVO(bastion.BastionSummary{
		Id:                   resp.Id,
		Name:                 resp.Name,
		BastionType:          resp.BastionType,
		LifecycleState:       resp.LifecycleState,
		CompartmentId:        resp.CompartmentId,
		TargetVcnId:          resp.TargetVcnId,
		MaxSessionTtlInSeconds: resp.MaxSessionTtlInSeconds,
		TimeCreated:          resp.TimeCreated,
		TimeUpdated:          resp.TimeUpdated,
	})
	return &vo, nil
}

// CreateSession creates a new bastion session (port-forwarding or managed SSH).
func CreateSession(ctx context.Context, c *bastion.BastionClient, in CreateSessionInput) (*SessionVO, error) {
	if in.PublicKeyContent == "" {
		return nil, fmt.Errorf("bastion: create session: publicKeyContent required")
	}

	details := bastion.CreateSessionDetails{
		BastionId: common.String(in.BastionID),
		KeyDetails: &bastion.PublicKeyDetails{
			PublicKeyContent: common.String(in.PublicKeyContent),
		},
	}
	if in.DisplayName != "" {
		details.DisplayName = common.String(in.DisplayName)
	}
	if in.SessionTTLInSeconds > 0 {
		details.SessionTtlInSeconds = common.Int(in.SessionTTLInSeconds)
	}

	switch in.SessionType {
	case "MANAGED_SSH":
		if in.TargetResourceID == "" {
			return nil, fmt.Errorf("bastion: create managed SSH session: targetResourceId required")
		}
		details.TargetResourceDetails = bastion.CreateManagedSshSessionTargetResourceDetails{
			TargetResourceId:                   common.String(in.TargetResourceID),
			TargetResourceOperatingSystemUserName: common.String(in.TargetResourceOSUserName),
		}
	default: // PORT_FORWARDING
		if in.TargetResourcePrivateIPAddress == "" {
			return nil, fmt.Errorf("bastion: create port-forwarding session: targetResourcePrivateIpAddress required")
		}
		pf := bastion.CreatePortForwardingSessionTargetResourceDetails{
			TargetResourcePrivateIpAddress: common.String(in.TargetResourcePrivateIPAddress),
		}
		if in.TargetResourcePort > 0 {
			pf.TargetResourcePort = common.Int(in.TargetResourcePort)
		}
		if in.TargetResourceID != "" {
			pf.TargetResourceId = common.String(in.TargetResourceID)
		}
		details.TargetResourceDetails = pf
	}

	resp, err := c.CreateSession(ctx, bastion.CreateSessionRequest{
		CreateSessionDetails: details,
	})
	if err != nil {
		return nil, fmt.Errorf("bastion: create session: %w", err)
	}
	vo := sessionToVO(resp.Session)
	return &vo, nil
}

// GetSession returns a single session by ID.
func GetSession(ctx context.Context, c *bastion.BastionClient, sessionID string) (*SessionVO, error) {
	resp, err := c.GetSession(ctx, bastion.GetSessionRequest{
		SessionId: common.String(sessionID),
	})
	if err != nil {
		return nil, fmt.Errorf("bastion: get session: %w", err)
	}
	vo := sessionToVO(resp.Session)
	return &vo, nil
}

// ListSessions returns all sessions for a bastion, paginated.
func ListSessions(ctx context.Context, c *bastion.BastionClient, bastionID string) ([]SessionVO, error) {
	var all []SessionVO
	var page *string

	for {
		resp, err := c.ListSessions(ctx, bastion.ListSessionsRequest{
			BastionId: common.String(bastionID),
			Page:      page,
		})
		if err != nil {
			return nil, fmt.Errorf("bastion: list sessions: %w", err)
		}
		for _, s := range resp.Items {
			all = append(all, sessionSummaryToVO(s))
		}
		if resp.OpcNextPage == nil {
			break
		}
		page = resp.OpcNextPage
	}
	return all, nil
}

// DeleteSession deletes a bastion session.
func DeleteSession(ctx context.Context, c *bastion.BastionClient, sessionID string) error {
	_, err := c.DeleteSession(ctx, bastion.DeleteSessionRequest{
		SessionId: common.String(sessionID),
	})
	if err != nil {
		return fmt.Errorf("bastion: delete session: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func bastionSummaryToVO(b bastion.BastionSummary) BastionSummaryVO {
	vo := BastionSummaryVO{
		ID:            derefStr(b.Id),
		Name:          derefStr(b.Name),
		BastionType:   derefStr(b.BastionType),
		LifecycleState: string(b.LifecycleState),
		CompartmentID: derefStr(b.CompartmentId),
		TargetVcnID:   derefStr(b.TargetVcnId),
	}
	if b.MaxSessionTtlInSeconds != nil {
		vo.MaxSessionTTL = *b.MaxSessionTtlInSeconds
	}
	if b.TimeCreated != nil {
		vo.TimeCreated = b.TimeCreated.Time.Format(timeLayout)
	}
	if b.TimeUpdated != nil {
		vo.TimeUpdated = b.TimeUpdated.Time.Format(timeLayout)
	}
	return vo
}

func sessionToVO(s bastion.Session) SessionVO {
	vo := SessionVO{
		ID:              derefStr(s.Id),
		BastionID:       derefStr(s.BastionId),
		BastionName:     derefStr(s.BastionName),
		DisplayName:     derefStr(s.DisplayName),
		LifecycleState:  string(s.LifecycleState),
		SshMetadata:     s.SshMetadata,
		BastionUserName: derefStr(s.BastionUserName),
		LifecycleDetails: derefStr(s.LifecycleDetails),
	}
	if s.SessionTtlInSeconds != nil {
		vo.SessionTTL = *s.SessionTtlInSeconds
	}
	if s.TimeCreated != nil {
		vo.TimeCreated = s.TimeCreated.Time.Format(timeLayout)
	}
	if s.TimeUpdated != nil {
		vo.TimeUpdated = s.TimeUpdated.Time.Format(timeLayout)
	}
	// Determine session type from TargetResourceDetails.
	vo.SessionType = inferSessionType(s.TargetResourceDetails)
	return vo
}

func sessionSummaryToVO(s bastion.SessionSummary) SessionVO {
	vo := SessionVO{
		ID:             derefStr(s.Id),
		BastionID:      derefStr(s.BastionId),
		BastionName:    derefStr(s.BastionName),
		DisplayName:    derefStr(s.DisplayName),
		LifecycleState: string(s.LifecycleState),
		LifecycleDetails: derefStr(s.LifecycleDetails),
	}
	if s.SessionTtlInSeconds != nil {
		vo.SessionTTL = *s.SessionTtlInSeconds
	}
	if s.TimeCreated != nil {
		vo.TimeCreated = s.TimeCreated.Time.Format(timeLayout)
	}
	if s.TimeUpdated != nil {
		vo.TimeUpdated = s.TimeUpdated.Time.Format(timeLayout)
	}
	vo.SessionType = inferSessionType(s.TargetResourceDetails)
	return vo
}

// inferSessionType determines the session type from the polymorphic target details.
func inferSessionType(d bastion.TargetResourceDetails) string {
	if d == nil {
		return "UNKNOWN"
	}
	switch d.(type) {
	case bastion.PortForwardingSessionTargetResourceDetails:
		return "PORT_FORWARDING"
	case bastion.ManagedSshSessionTargetResourceDetails:
		return "MANAGED_SSH"
	case bastion.DynamicPortForwardingSessionTargetResourceDetails:
		return "DYNAMIC_PORT_FORWARDING"
	default:
		return "UNKNOWN"
	}
}

// ensure unused import does not fail compilation.
var _ = http.StatusOK
