// Package service -- resourcemgr.go: Phase 13.3 Resource Manager service.
// Orchestrates OCI Resource Manager stack and job operations.
package service

import (
	"context"
	"fmt"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
	"github.com/oracle/oci-go-sdk/v65/resourcemanager"
)

// ResourceMgrService manages OCI Resource Manager operations per tenant.
type ResourceMgrService struct {
	store     *db.Store
	masterKey []byte
}

// NewResourceMgrService constructs a ResourceMgrService.
func NewResourceMgrService(store *db.Store, masterKey []byte) *ResourceMgrService {
	return &ResourceMgrService{store: store, masterKey: masterKey}
}

// ListStacks lists all Resource Manager stacks in a compartment.
func (s *ResourceMgrService) ListStacks(ctx context.Context, tenantID int64, compartmentID, displayName string, limit int, page string) ([]resourcemanager.StackSummary, *string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.ListStacks(ctx, client, compartmentID, displayName, limit, page)
}

// GetStack retrieves details of a Resource Manager stack.
func (s *ResourceMgrService) GetStack(ctx context.Context, tenantID int64, stackID string) (*resourcemanager.Stack, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.GetStack(ctx, client, stackID)
}

// CreateStack creates a new Resource Manager stack.
func (s *ResourceMgrService) CreateStack(ctx context.Context, tenantID int64, compartmentID, displayName, description string, configSource resourcemanager.CreateConfigSourceDetails) (*resourcemanager.Stack, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.CreateStack(ctx, client, compartmentID, displayName, description, configSource)
}

// DeleteStack deletes a Resource Manager stack.
func (s *ResourceMgrService) DeleteStack(ctx context.Context, tenantID int64, stackID string) error {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.DeleteStack(ctx, client, stackID)
}

// CreatePlanJob creates a plan job for a stack.
func (s *ResourceMgrService) CreatePlanJob(ctx context.Context, tenantID int64, stackID string) (*resourcemanager.Job, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.CreatePlanJob(ctx, client, stackID)
}

// CreateApplyJob creates an apply job for a stack.
func (s *ResourceMgrService) CreateApplyJob(ctx context.Context, tenantID int64, stackID string, planJobID *string) (*resourcemanager.Job, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.CreateApplyJob(ctx, client, stackID, planJobID)
}

// CreateDestroyJob creates a destroy job for a stack.
func (s *ResourceMgrService) CreateDestroyJob(ctx context.Context, tenantID int64, stackID string) (*resourcemanager.Job, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.CreateDestroyJob(ctx, client, stackID)
}

// GetJob retrieves details of a Resource Manager job.
func (s *ResourceMgrService) GetJob(ctx context.Context, tenantID int64, jobID string) (*resourcemanager.Job, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.GetJob(ctx, client, jobID)
}

// ListJobs lists all Resource Manager jobs in a compartment.
func (s *ResourceMgrService) ListJobs(ctx context.Context, tenantID int64, compartmentID, stackID string, limit int, page string) ([]resourcemanager.JobSummary, *string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.ListJobs(ctx, client, compartmentID, stackID, limit, page)
}

// GetJobLogs retrieves the logs for a Resource Manager job.
func (s *ResourceMgrService) GetJobLogs(ctx context.Context, tenantID int64, jobID string, limit int, page string) ([]resourcemanager.LogEntry, *string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.GetJobLogs(ctx, client, jobID, limit, page)
}

// GetJobTfState retrieves the Terraform state for a job.
func (s *ResourceMgrService) GetJobTfState(ctx context.Context, tenantID int64, jobID string) (string, error) {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return "", err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.GetJobTfState(ctx, client, jobID)
}

// CancelJob cancels a running Resource Manager job.
func (s *ResourceMgrService) CancelJob(ctx context.Context, tenantID int64, jobID string) error {
	client, err := s.newClient(ctx, tenantID)
	if err != nil {
		return err
	}
	ops := &oci.ResourceMgrOps{}
	return ops.CancelJob(ctx, client, jobID)
}

// newClient creates a ResourceManagerClient from tenant credentials.
func (s *ResourceMgrService) newClient(ctx context.Context, tenantID int64) (*resourcemanager.ResourceManagerClient, error) {
	t, err := repo.New(s.store.Read).FindTenantByID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("tenant %d not found: %w", tenantID, err)
	}
	creds := oci.Credentials{
		Tenancy:     nsStr(t.Tenancy),
		UserID:      nsStr(t.TenantID),
		Fingerprint: nsStr(t.Fingerprint),
		Region:      nsStr(t.Region),
		KeyFileBlob: nsStr(t.KeyFileBlob),
		KeyFile:     nsStr(t.KeyFile),
	}
	provider, err := oci.NewProvider(creds, s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("oci provider: %w", err)
	}
	client, err := resourcemanager.NewResourceManagerClientWithConfigurationProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("resourcemanager client: %w", err)
	}
	return &client, nil
}
