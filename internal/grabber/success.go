// Package grabber — success.go handles the post-launch success event chain
// (SPEC S8.6). Port of OpenSuccessServiceImpl.doSuccess:
//  1. Update DB: status=2 (COMPLETED), public_ip
//  2. Mark notification as sent (idempotent gate)
//  3. Trigger notification (Telegram) — stubbed, ready for Phase 7
//  4. Schedule 3-min delayed boot-volume backup (time.AfterFunc)
package grabber

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Muione/oci-start-go/internal/repo"
)

// onGrabSuccess runs after a successful OCI instance launch. It updates the
// boot task status, triggers the idempotent notification gate, and schedules
// a delayed backup. Runs synchronously in the API pool callback; heavy work
// is dispatched to goroutines with their own context.
func (e *Engine) onGrabSuccess(ctx context.Context, task repo.BootInstance, result *GrabResult) {
	bootID := ns(task.BootID)
	e.deps.Logger.Info().
		Str("bootId", bootID).
		Str("instanceId", result.InstanceID).
		Str("publicIp", result.PublicIP).
		Msg("grabber: GRAB SUCCESS")

	q := repo.New(e.deps.Store.Write)
	now := time.Now().Format("2006-01-02 15:04:05")

	// 1. Update DB: status=2 (COMPLETED), public_ip.
	statusComplete := int64(2) // parity: status 2 = SUCCESS/COMPLETED
	if err := q.UpdateBootInstanceStatus(ctx, repo.UpdateBootInstanceStatusParams{
		Status:    sql.NullInt64{Int64: statusComplete, Valid: true},
		PublicIp:  sql.NullString{String: result.PublicIP, Valid: result.PublicIP != ""},
		UpdatedAt: sql.NullString{String: now, Valid: true},
		ID:        task.ID,
	}); err != nil {
		e.deps.Logger.Error().Err(err).Str("bootId", bootID).Msg("grabber: update status failed")
	}

	// Increment success count (total count for daily tracking).
	_ = q.IncrementSuccessCount(ctx, repo.IncrementSuccessCountParams{
		UpdatedAt: sql.NullString{String: now, Valid: true},
		ID:        task.ID,
	})

	// 2. Idempotent notification gate.
	notifyFlag := ns(task.NotifyFlag)
	if notifyFlag == "NO" {
		if err := q.MarkNotificationAsSent(ctx, task.ID); err != nil {
			e.deps.Logger.Warn().Err(err).Str("bootId", bootID).Msg("grabber: mark notify failed")
		} else {
			// Row was updated (notify_flag changed from NO → YES) — send notification.
			e.sendGrabNotification(ctx, task, result)
		}
	}

	// 3. Schedule 3-min delayed boot-volume backup.
	// Parity with: delayedTaskExecutor.schedule(() -> eventPublisher.publishEvent(
	//   new InstanceBackUpEvent(this, instanceData)), 3, TimeUnit.MINUTES)
	time.AfterFunc(3*time.Minute, func() {
		e.scheduleBackup(task, result)
	})

	// 4. Store temp instance record for tracking.
	_ = e.saveTemInstance(ctx, task, result)
}

// sendGrabNotification sends a Telegram notification for a successful grab.
func (e *Engine) sendGrabNotification(ctx context.Context, task repo.BootInstance, result *GrabResult) {
	if e.deps.Notifier == nil {
		return
	}
	bootID := ns(task.BootID)
	msg := fmt.Sprintf("抢机成功!\nTask: %s\nInstance: %s\nIP: %s",
		bootID, result.InstanceID, result.PublicIP)
	if err := e.deps.Notifier.Send(ctx, msg); err != nil {
		e.deps.Logger.Warn().Err(err).Str("bootId", bootID).Msg("grabber: notify failed")
	}
}

// scheduleBackup triggers a boot-volume backup 3 minutes after a successful
// grab. If OnGrabSuccess callback is configured (via BackupSvc), it delegates
// to that; otherwise falls back to a log message.
func (e *Engine) scheduleBackup(task repo.BootInstance, result *GrabResult) {
	bootID := ns(task.BootID)
	if e.deps.OnGrabSuccess != nil {
		e.deps.Logger.Info().
			Str("bootId", bootID).
			Str("instanceId", result.InstanceID).
			Msg("grabber: delegating backup to OnGrabSuccess callback")
		e.deps.OnGrabSuccess(context.Background(), task, result)
		return
	}

	e.deps.Logger.Info().
		Str("bootId", bootID).
		Str("instanceId", result.InstanceID).
		Str("publicIp", result.PublicIP).
		Msg("grabber: backup skipped (OnGrabSuccess not configured)")
}

// saveTemInstance stores a temporary instance record for tracking.
func (e *Engine) saveTemInstance(ctx context.Context, task repo.BootInstance, result *GrabResult) error {
	// Find the tenant to get the tenancy OCID.
	if !task.TenantID.Valid {
		return nil
	}
	tenant, err := repo.New(e.deps.Store.Read).FindTenantByID(ctx, task.TenantID.Int64)
	if err != nil {
		return err
	}

	return repo.New(e.deps.Store.Write).InsertTemInstance(ctx, repo.InsertTemInstanceParams{
		Tenancy:            nsStr(tenant.Tenancy),
		InstanceID:         sql.NullString{String: result.InstanceID, Valid: true},
		PublicIp:           sql.NullString{String: result.PublicIP, Valid: result.PublicIP != ""},
		Region:             nsStr(tenant.Region),
		Architecture:       task.Architecture,
		RootPassword:       task.RootPassword,
		CloneBootVolumeID:  sql.NullString{}, // filled after backup
		CloudType:          task.CloudType,
	})
}

func nsStr(v sql.NullString) sql.NullString { return v }
