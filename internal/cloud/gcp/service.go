// Package gcp — service.go: GCP Compute Engine API integration (Phase 8).
// Implements OtherBootInstance management for GCP: list/create/delete
// instances with cloud-init, supporting the grab engine's multi-cloud
// design. Uses Application Default Credentials.
//
// Parity with Java GCP compute integration.
package gcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BootTask represents a GCP boot task (OtherBootInstance).
type BootTask struct {
	ID              string    `json:"id"`
	TenantID        int64     `json:"tenantId"`
	ProjectID       string    `json:"projectId"`
	Zone            string    `json:"zone"`
	MachineType     string    `json:"machineType"`
	SourceImage     string    `json:"sourceImage"`
	DiskSizeGb      int64     `json:"diskSizeGb"`
	Preemptible     bool      `json:"preemptible"`
	InstanceCount   int64     `json:"instanceCount"`
	Architecture    string    `json:"architecture"` // ARM64 or X86_64
	CloudInit       string    `json:"cloudInit"`
	Status          int       `json:"status"` // 0=stopped, 1=active, 2=success
	SuccessCount    int64     `json:"successCount"`
	FailCount       int64     `json:"failCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// InstanceInfo represents basic info about a running GCP instance.
type InstanceInfo struct {
	Name          string `json:"name"`
	Zone          string `json:"zone"`
	Status        string `json:"status"` // RUNNING, TERMINATED, etc.
	MachineType   string `json:"machineType"`
	PublicIP      string `json:"publicIp"`
	PrivateIP     string `json:"privateIp"`
	Architecture  string `json:"architecture"`
	CPUPlatform   string `json:"cpuPlatform"`
	CreationTime  string `json:"creationTime"`
}

// GcpClient authenticates with GCP and manages Compute Engine resources.
type GcpClient struct {
	credentialsJSON string // raw service account JSON
	projectID       string
	auth            *GcpAuth
}

// NewGcpClient creates a GCP client from a service account JSON.
func NewGcpClient(credentialsJSON, projectID string) (*GcpClient, error) {
	credsJSON := strings.TrimSpace(credentialsJSON)
	if credsJSON == "" {
		return nil, fmt.Errorf("GCP: credentials JSON is required")
	}
	auth, err := NewGcpAuth(credsJSON)
	if err != nil {
		return nil, fmt.Errorf("GCP: %w", err)
	}
	return &GcpClient{
		credentialsJSON: credsJSON,
		projectID:       projectID,
		auth:            auth,
	}, nil
}

// GcpService manages GCP OtherBootInstance records and Compute Engine operations.
type GcpService struct {
	client *GcpClient
	// store  for persisting boot tasks — injected via SetStore
	store BootTaskStore
}

// BootTaskStore is the persistence interface for GCP boot tasks.
type BootTaskStore interface {
	ListBootTasks(ctx context.Context) ([]BootTask, error)
	CreateBootTask(ctx context.Context, task BootTask) error
	UpdateBootTask(ctx context.Context, task BootTask) error
	DeleteBootTask(ctx context.Context, id string) error
	GetBootTask(ctx context.Context, id string) (*BootTask, error)
}

// InMemoryStore is a simple in-memory BootTaskStore for standalone use.
type InMemoryStore struct {
	tasks map[string]BootTask
}

// NewInMemoryStore creates an in-memory boot task store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{tasks: make(map[string]BootTask)}
}

// ListBootTasks returns all boot tasks.
func (s *InMemoryStore) ListBootTasks(ctx context.Context) ([]BootTask, error) {
	var tasks []BootTask
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// CreateBootTask saves a boot task.
func (s *InMemoryStore) CreateBootTask(ctx context.Context, task BootTask) error {
	s.tasks[task.ID] = task
	return nil
}

// UpdateBootTask updates a boot task.
func (s *InMemoryStore) UpdateBootTask(ctx context.Context, task BootTask) error {
	s.tasks[task.ID] = task
	return nil
}

// DeleteBootTask removes a boot task.
func (s *InMemoryStore) DeleteBootTask(ctx context.Context, id string) error {
	delete(s.tasks, id)
	return nil
}

// GetBootTask gets a boot task by ID.
func (s *InMemoryStore) GetBootTask(ctx context.Context, id string) (*BootTask, error) {
	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("boot task %s not found", id)
	}
	return &t, nil
}

// NewGcpService creates a GCP boot task service.
func NewGcpService(client *GcpClient, store BootTaskStore) *GcpService {
	return &GcpService{client: client, store: store}
}

// NewGcpServiceWithClient creates a service with an existing GCP client and
// an in-memory store as default.
func NewGcpServiceWithClient(client *GcpClient) *GcpService {
	return &GcpService{client: client, store: NewInMemoryStore()}
}

// SetStore injects a persistence layer for boot tasks.
func (s *GcpService) SetStore(store BootTaskStore) {
	s.store = store
}

// ListBootTasks returns all GCP boot tasks from the store.
func (s *GcpService) ListBootTasks(ctx context.Context) ([]map[string]any, error) {
	tasks, err := s.store.ListBootTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list GCP boot tasks: %w", err)
	}
	var result []map[string]any
	for _, t := range tasks {
		result = append(result, map[string]any{
			"id":            t.ID,
			"tenantId":      t.TenantID,
			"projectId":     t.ProjectID,
			"zone":          t.Zone,
			"machineType":   t.MachineType,
			"sourceImage":   t.SourceImage,
			"diskSizeGb":    t.DiskSizeGb,
			"preemptible":   t.Preemptible,
			"instanceCount": t.InstanceCount,
			"architecture":  t.Architecture,
			"status":        t.Status,
			"successCount":  t.SuccessCount,
			"failCount":     t.FailCount,
			"createdAt":     t.CreatedAt.Format(time.RFC3339),
			"updatedAt":     t.UpdatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

// CreateBootTask creates a GCP boot task.
func (s *GcpService) CreateBootTask(ctx context.Context, input map[string]any) error {
	task := BootTask{
		ID:        fmt.Sprintf("gcp-boot-%d", time.Now().UnixNano()),
		Status:    1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if v, ok := input["tenantId"]; ok {
		task.TenantID = toInt64(v)
	}
	if v, ok := input["projectId"]; ok {
		task.ProjectID = fmt.Sprint(v)
	}
	if v, ok := input["zone"]; ok {
		task.Zone = fmt.Sprint(v)
	}
	if v, ok := input["machineType"]; ok {
		task.MachineType = fmt.Sprint(v)
	}
	if v, ok := input["sourceImage"]; ok {
		task.SourceImage = fmt.Sprint(v)
	}
	if v, ok := input["diskSizeGb"]; ok {
		task.DiskSizeGb = toInt64(v)
	}
	if v, ok := input["preemptible"]; ok {
		task.Preemptible = toBool(v)
	}
	if v, ok := input["instanceCount"]; ok {
		task.InstanceCount = toInt64(v)
	}
	if v, ok := input["architecture"]; ok {
		task.Architecture = fmt.Sprint(v)
	}
	if v, ok := input["cloudInit"]; ok {
		task.CloudInit = fmt.Sprint(v)
	}

	if s.client != nil {
		s.client.projectID = task.ProjectID
	}

	return s.store.CreateBootTask(ctx, task)
}

// DeleteBootTask deletes a GCP boot task.
func (s *GcpService) DeleteBootTask(ctx context.Context, id string) error {
	return s.store.DeleteBootTask(ctx, id)
}

// GetBootTask gets a single boot task.
func (s *GcpService) GetBootTask(ctx context.Context, id string) (*BootTask, error) {
	return s.store.GetBootTask(ctx, id)
}

// LaunchGcpInstance creates a GCP Compute Engine instance via the REST API.
// Authenticates using the service account's RSA key (JWT bearer flow) and
// posts to the Compute Engine instances.insert endpoint.
//
// API: POST https://compute.googleapis.com/compute/v1/projects/{project}/zones/{zone}/instances
func (s *GcpService) LaunchGcpInstance(ctx context.Context, task BootTask) (*InstanceInfo, error) {
	if s.client == nil || s.client.auth == nil {
		return nil, fmt.Errorf("GCP client not configured — set gcp.serviceAccountJson and gcp.projectId in system config")
	}

	projectID := s.client.projectID
	if projectID == "" {
		projectID = task.ProjectID
	}
	if projectID == "" || task.Zone == "" {
		return nil, fmt.Errorf("GCP: projectID and zone are required")
	}

	name := fmt.Sprintf("gcp-%s-%d", strings.ToLower(task.Architecture), time.Now().Unix())

	// Build instance insert request body per GCP Compute Engine API.
	body := map[string]interface{}{
		"name": name,
		"machineType": fmt.Sprintf("zones/%s/machineTypes/%s", task.Zone, task.MachineType),
		"disks": []map[string]interface{}{
			{
				"boot":       true,
				"autoDelete": true,
				"initializeParams": map[string]interface{}{
					"sourceImage": task.SourceImage,
					"diskSizeGb":  strconv.FormatInt(task.DiskSizeGb, 10),
				},
			},
		},
		"networkInterfaces": []map[string]interface{}{
			{
				"network": "global/networks/default",
				"accessConfigs": []map[string]interface{}{
					{
						"name": "External NAT",
						"type": "ONE_TO_ONE_NAT",
					},
				},
			},
		},
		"scheduling": map[string]interface{}{
			"preemptible": task.Preemptible,
		},
	}

	if task.CloudInit != "" {
		body["metadata"] = map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"key":   "user-data",
					"value": base64.StdEncoding.EncodeToString([]byte(task.CloudInit)),
				},
			},
		}
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("GCP: marshal instance body: %w", err)
	}

	// Obtain an OAuth2-authenticated HTTP client.
	httpClient, err := s.client.auth.HTTPClient()
	if err != nil {
		return nil, fmt.Errorf("GCP: auth client: %w", err)
	}

	apiURL := fmt.Sprintf(
		"https://compute.googleapis.com/compute/v1/projects/%s/zones/%s/instances",
		projectID, task.Zone,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("GCP: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GCP: instances.insert: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GCP: instances.insert returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the operation response.
	var op gcpOperation
	if err := json.Unmarshal(respBody, &op); err != nil {
		return &InstanceInfo{
			Name:        name,
			Zone:        task.Zone,
			Status:      "PROVISIONING",
			MachineType: task.MachineType,
		}, nil // body parse failed but request succeeded — instance is provisioning
	}

	info := &InstanceInfo{
		Name:        name,
		Zone:        task.Zone,
		Status:      op.Status,
		MachineType: task.MachineType,
	}
	if op.Status == "" {
		info.Status = "PROVISIONING"
	}

	return info, nil
}

// gcpOperation is the response from a GCP Compute Engine zonal operation.
type gcpOperation struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // PENDING, RUNNING, DONE
	Zone   string `json:"zone"`
}

// IsConfigured returns true if GCP credentials are available and valid.
func (s *GcpService) IsConfigured() bool {
	return s.client != nil && s.client.auth != nil
}

// --- helpers ---

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		i, _ := strconv.ParseInt(val, 10, 64)
		return i
	case json.Number:
		i, _ := val.Int64()
		return i
	}
	return 0
}

func toBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	case int64:
		return val != 0
	case float64:
		return val != 0
	}
	return false
}
