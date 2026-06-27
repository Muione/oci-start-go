// Package service — boot_instance.go provides CRUD operations for the
// boot_instance table (Phase 4). Boot tasks define the target specs for the
// grab engine: region, architecture, OCPU/memory/disk, loop interval, etc.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// BootService manages boot_instance records.
type BootService struct {
	store *db.Store
}

// NewBootService creates a new BootService.
func NewBootService(store *db.Store) *BootService {
	return &BootService{store: store}
}

// BootTask is the API-facing representation of a boot_instance row.
type BootTask struct {
	ID                     int64  `json:"id"`
	BootID                 string `json:"bootId"`
	TenantID               int64  `json:"tenantId"`
	Ocpu                   int64  `json:"ocpu"`
	Memory                 int64  `json:"memory"`
	Disk                   int64  `json:"disk"`
	LoopTime               int64  `json:"loopTime"`
	InstanceCount          int64  `json:"instanceCount"`
	Status                 int64  `json:"status"`
	Architecture           string `json:"architecture"`
	RootPassword           string `json:"rootPassword,omitempty"`
	PublicIP               string `json:"publicIp"`
	ImageID                string `json:"imageId"`
	OperatingSystem        string `json:"operatingSystem"`
	OperatingSystemVersion string `json:"operatingSystemVersion"`
	DataGap                string `json:"dataGap"`
	NotifyFlag             string `json:"notifyFlag"`
	NextExecutionTime      string `json:"nextExecutionTime"`
	FailCount              int64  `json:"failCount"`
	SuccessCount           int64  `json:"successCount"`
	TotalCount             int64  `json:"totalCount"`
	Remark                 string `json:"remark"`
	CloudType              int64  `json:"cloudType"`
	CreatedAt              string `json:"createdAt"`
	UpdatedAt              string `json:"updatedAt"`
}

// List returns all boot tasks ordered by creation time desc.
func (s *BootService) List(ctx context.Context) ([]BootTask, error) {
	rows, err := repo.New(s.store.Read).ListBootInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("list boot instances: %w", err)
	}
	out := make([]BootTask, 0, len(rows))
	for _, r := range rows {
		out = append(out, toBootTask(r))
	}
	return out, nil
}

// BootSaveInput carries the fields for adding or updating a boot task.
type BootSaveInput struct {
	BootID                 string `json:"bootId"`                 // empty → create
	TenantID               int64  `json:"tenantId"`
	Ocpu                   int64  `json:"ocpu"`
	Memory                 int64  `json:"memory"`
	Disk                   int64  `json:"disk"`
	LoopTime               int64  `json:"loopTime"`
	InstanceCount          int64  `json:"instanceCount"`
	Architecture           string `json:"architecture"`
	RootPassword           string `json:"rootPassword"`
	ImageID                string `json:"imageId"`
	OperatingSystem        string `json:"operatingSystem"`
	OperatingSystemVersion string `json:"operatingSystemVersion"`
	DataGap                string `json:"dataGap"`
	NotifyFlag             string `json:"notifyFlag"`
	Remark                 string `json:"remark"`
	CloudType              int64  `json:"cloudType"`
}

// Save creates or updates a boot task. If BootID is empty, a new task is created
// with a UUID boot_id; otherwise the existing task is updated.
func (s *BootService) Save(ctx context.Context, in BootSaveInput) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	q := repo.New(s.store.Write)

	if in.BootID == "" {
		// Create new task.
		bootID := uuid.New().String()
		loopTime := in.LoopTime
		if loopTime <= 0 {
			loopTime = 6
		}
		nextTime := time.Now().Add(time.Duration(loopTime) * time.Second).Format("2006-01-02 15:04:05")
		cloudType := in.CloudType
		if cloudType == 0 {
			cloudType = 1 // ORACLE_CLOUD
		}
		status := int64(1) // enabled
		notifyFlag := in.NotifyFlag
		if notifyFlag == "" {
			notifyFlag = "NO"
		}
		return q.InsertBootInstance(ctx, repo.InsertBootInstanceParams{
			BootID:                 sql.NullString{String: bootID, Valid: true},
			TenantID:               sql.NullInt64{Int64: in.TenantID, Valid: in.TenantID > 0},
			Ocpu:                   sql.NullInt64{Int64: in.Ocpu, Valid: true},
			Memory:                 sql.NullInt64{Int64: in.Memory, Valid: true},
			Disk:                   sql.NullInt64{Int64: in.Disk, Valid: true},
			LoopTime:               sql.NullInt64{Int64: loopTime, Valid: true},
			InstanceCount:          sql.NullInt64{Int64: in.InstanceCount, Valid: true},
			Status:                 sql.NullInt64{Int64: status, Valid: true},
			Architecture:           sql.NullString{String: in.Architecture, Valid: in.Architecture != ""},
			RootPassword:           sql.NullString{String: in.RootPassword, Valid: in.RootPassword != ""},
			ImageID:                sql.NullString{String: in.ImageID, Valid: in.ImageID != ""},
			OperatingSystem:        sql.NullString{String: in.OperatingSystem, Valid: in.OperatingSystem != ""},
			OperatingSystemVersion: sql.NullString{String: in.OperatingSystemVersion, Valid: in.OperatingSystemVersion != ""},
			NextExecutionTime:      sql.NullString{String: nextTime, Valid: true},
			CloudType:              sql.NullInt64{Int64: cloudType, Valid: true},
			DataGap:                sql.NullString{String: in.DataGap, Valid: in.DataGap != ""},
			NotifyFlag:             sql.NullString{String: notifyFlag, Valid: true},
			Remark:                 sql.NullString{String: in.Remark, Valid: in.Remark != ""},
			CreatedAt:              sql.NullString{String: now, Valid: true},
			UpdatedAt:              sql.NullString{String: now, Valid: true},
		})
	}

	// Update existing task.
	return q.UpdateBootInstance(ctx, repo.UpdateBootInstanceParams{
		Ocpu:                   sql.NullInt64{Int64: in.Ocpu, Valid: true},
		Memory:                 sql.NullInt64{Int64: in.Memory, Valid: true},
		Disk:                   sql.NullInt64{Int64: in.Disk, Valid: true},
		LoopTime:               sql.NullInt64{Int64: in.LoopTime, Valid: true},
		InstanceCount:          sql.NullInt64{Int64: in.InstanceCount, Valid: true},
		Architecture:           sql.NullString{String: in.Architecture, Valid: in.Architecture != ""},
		ImageID:                sql.NullString{String: in.ImageID, Valid: in.ImageID != ""},
		OperatingSystem:        sql.NullString{String: in.OperatingSystem, Valid: in.OperatingSystem != ""},
		OperatingSystemVersion: sql.NullString{String: in.OperatingSystemVersion, Valid: in.OperatingSystemVersion != ""},
		DataGap:                sql.NullString{String: in.DataGap, Valid: in.DataGap != ""},
		NotifyFlag:             sql.NullString{String: in.NotifyFlag, Valid: in.NotifyFlag != ""},
		Remark:                 sql.NullString{String: in.Remark, Valid: in.Remark != ""},
		UpdatedAt:              sql.NullString{String: now, Valid: true},
		BootID:                 sql.NullString{String: in.BootID, Valid: true},
	})
}

// Remove soft-deletes a boot task (status=0).
func (s *BootService) Remove(ctx context.Context, bootID string) error {
	if bootID == "" {
		return fmt.Errorf("bootId required")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	return repo.New(s.store.Write).DisableBootInstance(ctx, repo.DisableBootInstanceParams{
		UpdatedAt: sql.NullString{String: now, Valid: true},
		BootID:    sql.NullString{String: bootID, Valid: true},
	})
}

// Toggle enables or disables a boot task.
func (s *BootService) Toggle(ctx context.Context, bootID string, enable bool) error {
	if bootID == "" {
		return fmt.Errorf("bootId required")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	q := repo.New(s.store.Write)

	if enable {
		// Get the task to determine loop_time for next execution.
		task, err := q.FindBootInstanceByBootID(ctx, sql.NullString{String: bootID, Valid: true})
		if err != nil {
			return fmt.Errorf("find task: %w", err)
		}
		loopTime := ni(task.LoopTime)
		if loopTime <= 0 {
			loopTime = 6
		}
		nextTime := time.Now().Add(time.Duration(loopTime) * time.Second).Format("2006-01-02 15:04:05")
		return q.EnableBootInstance(ctx, repo.EnableBootInstanceParams{
			NextExecutionTime: sql.NullString{String: nextTime, Valid: true},
			UpdatedAt:         sql.NullString{String: now, Valid: true},
			BootID:            sql.NullString{String: bootID, Valid: true},
		})
	}

	return q.DisableBootInstance(ctx, repo.DisableBootInstanceParams{
		UpdatedAt: sql.NullString{String: now, Valid: true},
		BootID:    sql.NullString{String: bootID, Valid: true},
	})
}

// --- helpers ---

func toBootTask(r repo.BootInstance) BootTask {
	return BootTask{
		ID:                     r.ID,
		BootID:                 ns(r.BootID),
		TenantID:               ni(r.TenantID),
		Ocpu:                   ni(r.Ocpu),
		Memory:                 ni(r.Memory),
		Disk:                   ni(r.Disk),
		LoopTime:               ni(r.LoopTime),
		InstanceCount:          ni(r.InstanceCount),
		Status:                 ni(r.Status),
		Architecture:           ns(r.Architecture),
		RootPassword:           ns(r.RootPassword),
		PublicIP:               ns(r.PublicIp),
		ImageID:                ns(r.ImageID),
		OperatingSystem:        ns(r.OperatingSystem),
		OperatingSystemVersion: ns(r.OperatingSystemVersion),
		DataGap:                ns(r.DataGap),
		NotifyFlag:             ns(r.NotifyFlag),
		NextExecutionTime:      ns(r.NextExecutionTime),
		FailCount:              ni(r.FailCount),
		SuccessCount:           ni(r.SuccessCount),
		TotalCount:             ni(r.TotalCount),
		Remark:                 ns(r.Remark),
		CloudType:              ni(r.CloudType),
		CreatedAt:              ns(r.CreatedAt),
		UpdatedAt:              ns(r.UpdatedAt),
	}
}
