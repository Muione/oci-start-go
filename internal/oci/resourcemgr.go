// Package oci -- resourcemgr.go: OCI Resource Manager SDK operations (Phase 13.3).
// Wraps the OCI Resource Manager service client for stack CRUD, job management,
// and Terraform state/plan operations. Parity with Java OciResourceManagerUtil.
package oci

import (
	"context"
	"fmt"
	"io"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/resourcemanager"
)

// ResourceMgrOps groups all OCI Resource Manager SDK operations.
type ResourceMgrOps struct{}

// CreateStack creates a new Resource Manager stack from Terraform configuration.
func (r *ResourceMgrOps) CreateStack(ctx context.Context, client *resourcemanager.ResourceManagerClient, compartmentID, displayName, description string, configSource resourcemanager.CreateConfigSourceDetails) (*resourcemanager.Stack, error) {
	req := resourcemanager.CreateStackRequest{
		CreateStackDetails: resourcemanager.CreateStackDetails{
			CompartmentId: &compartmentID,
			DisplayName:   &displayName,
			Description:   &description,
			ConfigSource:  configSource,
		},
	}
	resp, err := client.CreateStack(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}
	return &resp.Stack, nil
}

// GetStack retrieves details of a Resource Manager stack.
func (r *ResourceMgrOps) GetStack(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID string) (*resourcemanager.Stack, error) {
	req := resourcemanager.GetStackRequest{
		StackId: &stackID,
	}
	resp, err := client.GetStack(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get stack %s: %w", stackID, err)
	}
	return &resp.Stack, nil
}

// ListStacks lists all Resource Manager stacks in a compartment.
func (r *ResourceMgrOps) ListStacks(ctx context.Context, client *resourcemanager.ResourceManagerClient, compartmentID string, displayName string, limit int, page string) ([]resourcemanager.StackSummary, *string, error) {
	req := resourcemanager.ListStacksRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if displayName != "" {
		req.DisplayName = &displayName
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListStacks(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list stacks: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// UpdateStack updates a Resource Manager stack.
func (r *ResourceMgrOps) UpdateStack(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID, displayName, description string) (*resourcemanager.Stack, error) {
	req := resourcemanager.UpdateStackRequest{
		StackId: &stackID,
		UpdateStackDetails: resourcemanager.UpdateStackDetails{
			DisplayName: &displayName,
			Description: &description,
		},
	}
	resp, err := client.UpdateStack(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("update stack %s: %w", stackID, err)
	}
	return &resp.Stack, nil
}

// DeleteStack deletes a Resource Manager stack.
func (r *ResourceMgrOps) DeleteStack(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID string) error {
	req := resourcemanager.DeleteStackRequest{
		StackId: &stackID,
	}
	_, err := client.DeleteStack(ctx, req)
	if err != nil {
		return fmt.Errorf("delete stack %s: %w", stackID, err)
	}
	return nil
}

// --- Job Operations ---

// CreatePlanJob creates a plan job for a stack.
func (r *ResourceMgrOps) CreatePlanJob(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID string) (*resourcemanager.Job, error) {
	req := resourcemanager.CreateJobRequest{
		CreateJobDetails: resourcemanager.CreateJobDetails{
			StackId:   &stackID,
			Operation: resourcemanager.JobOperationPlan,
		},
	}
	resp, err := client.CreateJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create plan job: %w", err)
	}
	return &resp.Job, nil
}

// CreateApplyJob creates an apply job for a stack.
func (r *ResourceMgrOps) CreateApplyJob(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID string, planJobID *string) (*resourcemanager.Job, error) {
	details := resourcemanager.CreateJobDetails{
		StackId:   &stackID,
		Operation: resourcemanager.JobOperationApply,
	}
	if planJobID != nil {
		details.JobOperationDetails = resourcemanager.CreateApplyJobOperationDetails{
			ExecutionPlanJobId:    planJobID,
			ExecutionPlanStrategy: resourcemanager.ApplyJobOperationDetailsExecutionPlanStrategyFromPlanJobId,
		}
	}
	req := resourcemanager.CreateJobRequest{
		CreateJobDetails: details,
	}
	resp, err := client.CreateJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create apply job: %w", err)
	}
	return &resp.Job, nil
}

// CreateDestroyJob creates a destroy job for a stack.
func (r *ResourceMgrOps) CreateDestroyJob(ctx context.Context, client *resourcemanager.ResourceManagerClient, stackID string) (*resourcemanager.Job, error) {
	req := resourcemanager.CreateJobRequest{
		CreateJobDetails: resourcemanager.CreateJobDetails{
			StackId:   &stackID,
			Operation: resourcemanager.JobOperationDestroy,
		},
	}
	resp, err := client.CreateJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create destroy job: %w", err)
	}
	return &resp.Job, nil
}

// GetJob retrieves details of a Resource Manager job.
func (r *ResourceMgrOps) GetJob(ctx context.Context, client *resourcemanager.ResourceManagerClient, jobID string) (*resourcemanager.Job, error) {
	req := resourcemanager.GetJobRequest{
		JobId: &jobID,
	}
	resp, err := client.GetJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", jobID, err)
	}
	return &resp.Job, nil
}

// ListJobs lists all Resource Manager jobs in a compartment.
func (r *ResourceMgrOps) ListJobs(ctx context.Context, client *resourcemanager.ResourceManagerClient, compartmentID, stackID string, limit int, page string) ([]resourcemanager.JobSummary, *string, error) {
	req := resourcemanager.ListJobsRequest{
		CompartmentId: &compartmentID,
		Limit:         common.Int(limit),
	}
	if stackID != "" {
		req.StackId = &stackID
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.ListJobs(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("list jobs: %w", err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// GetJobLogs retrieves the logs for a Resource Manager job.
func (r *ResourceMgrOps) GetJobLogs(ctx context.Context, client *resourcemanager.ResourceManagerClient, jobID string, limit int, page string) ([]resourcemanager.LogEntry, *string, error) {
	req := resourcemanager.GetJobLogsRequest{
		JobId: &jobID,
		Limit: common.Int(limit),
	}
	if page != "" {
		req.Page = &page
	}
	resp, err := client.GetJobLogs(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("get job logs %s: %w", jobID, err)
	}
	return resp.Items, resp.OpcNextPage, nil
}

// GetJobTfState retrieves the Terraform state for a job.
func (r *ResourceMgrOps) GetJobTfState(ctx context.Context, client *resourcemanager.ResourceManagerClient, jobID string) (string, error) {
	req := resourcemanager.GetJobTfStateRequest{
		JobId: &jobID,
	}
	resp, err := client.GetJobTfState(ctx, req)
	if err != nil {
		return "", fmt.Errorf("get job tf state %s: %w", jobID, err)
	}
	// Content is io.ReadCloser; read it into a string.
	if resp.Content == nil {
		return "", nil
	}
	defer resp.Content.Close()
	buf, err := io.ReadAll(resp.Content)
	if err != nil {
		return "", fmt.Errorf("read tf state %s: %w", jobID, err)
	}
	return string(buf), nil
}

// CancelJob cancels a running Resource Manager job.
func (r *ResourceMgrOps) CancelJob(ctx context.Context, client *resourcemanager.ResourceManagerClient, jobID string) error {
	req := resourcemanager.CancelJobRequest{
		JobId: &jobID,
	}
	_, err := client.CancelJob(ctx, req)
	if err != nil {
		return fmt.Errorf("cancel job %s: %w", jobID, err)
	}
	return nil
}
