// Package grabber — failure.go handles grab failure (SPEC S8.7).
// Network-temporary errors (DNS, UnknownHost, Connect) do NOT increment
// failCount. Other errors increment failCount and notify via Telegram.
// ALL failure paths release the single-flight key (the caller in
// executeGrabTask already does defer removeTaskKey).
package grabber

import (
	"context"
	"fmt"
	"strings"

	"github.com/Muione/oci-start-go/internal/oci/region"
	"github.com/Muione/oci-start-go/internal/repo"
)

// onGrabFailure handles a failed grab attempt. It classifies the error,
// optionally increments the fail counter, and sends an alert for hard errors.
func (e *Engine) onGrabFailure(ctx context.Context, task repo.BootInstance, result *GrabResult) {
	bootID := ns(task.BootID)
	errMsg := result.Error

	e.deps.Logger.Warn().
		Str("bootId", bootID).
		Str("error", errMsg).
		Msg("grabber: grab failed")

	// Classify the error.
	if isNetworkTemporaryError(errMsg) {
		// Network-temporary: do NOT increment failCount.
		// The task will retry on the next scheduling cycle.
		e.deps.Logger.Debug().
			Str("bootId", bootID).
			Str("error", errMsg).
			Msg("grabber: network temporary error, skipping fail count")
		return
	}

	// Hard error: increment failCount + notify.
	e.incrementFailCount(ctx, task, errMsg)
	e.sendFailureNotification(ctx, task, errMsg)
}

// isNetworkTemporaryError returns true for DNS/connectivity errors that should
// not count against the task's failure limit.
func isNetworkTemporaryError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unknownhost") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connect:") ||
		strings.Contains(lower, "timeout") && !strings.Contains(lower, "80s")
}

// incrementFailCount atomically increments the task's fail_count and sends a
// Telegram error notification.
func (e *Engine) incrementFailCount(ctx context.Context, task repo.BootInstance, errMsg string) {
	q := repo.New(e.deps.Store.Write)
	if err := q.IncrementFailCount(ctx, task.ID); err != nil {
		e.deps.Logger.Error().Err(err).Str("bootId", ns(task.BootID)).Msg("grabber: increment fail count error")
	}
	e.deps.Logger.Warn().
		Str("bootId", ns(task.BootID)).
		Str("error", errMsg).
		Msg("grabber: fail count incremented")
}

// sendFailureNotification sends a Telegram notification for a hard failure.
func (e *Engine) sendFailureNotification(ctx context.Context, task repo.BootInstance, errMsg string) {
	if e.deps.Notifier == nil {
		return
	}
	bootID := ns(task.BootID)
	tenantName := "unknown"
	regionName := "unknown"
	if task.TenantID.Valid {
		tenant, err := repo.New(e.deps.Store.Read).FindTenantByID(ctx, task.TenantID.Int64)
		if err == nil {
			tenantName = ns(tenant.UserName)
			regionName = region.NameByCode(region.CodeByName(ns(tenant.Region)))
		}
	}
	msg := fmt.Sprintf("[抢机失败]\nTask: %s\n用户: %s\n区域: %s\n错误: %s",
		bootID, tenantName, regionName, errMsg)
	if err := e.deps.Notifier.Send(ctx, msg); err != nil {
		e.deps.Logger.Warn().Err(err).Str("bootId", bootID).Msg("grabber: notify failed")
	}
}
