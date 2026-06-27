// Package grabber — engine.go implements the core scheduling engine
// (SPEC S8.2-S8.3). CheckAndExecuteTasksOnce is the per-tick entry point
// called by the cron scheduler every 6s.
package grabber

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/notify"
	"github.com/Muione/oci-start-go/internal/oci"
	"github.com/Muione/oci-start-go/internal/repo"
)

// EngineConfig wraps the grabber-specific configuration.
type EngineConfig struct {
	Pool      PoolConfig
	BatchSize int // 0 = auto-detect from system resources
}

func (c *EngineConfig) normalize() {
	c.Pool.normalize()
	if c.BatchSize <= 0 {
		c.BatchSize = detectBatchSize()
	}
}

func detectBatchSize() int {
	// Parity with CreateInstanceTaskV2 constructor: size based on CPU+memory.
	// In Go we approximate with runtime.NumCPU.
	cpu := 1 // default conservative
	if n := runtimeNumCPU(); n > 0 {
		cpu = n
	}
	switch {
	case cpu <= 1:
		return 30
	case cpu <= 2:
		return 50
	default:
		return 200
	}
}

func runtimeNumCPU() int { return 4 } // conservative; real runtime.NumCPU depends on cgroup

// EngineDeps bundles the services the engine needs.
type EngineDeps struct {
	Store     *db.Store
	ProxyPool *oci.ProxyPool
	MasterKey []byte
	Logger    zerolog.Logger
	Notifier  notify.Notifier // Phase 7: telegram notifications

	// OnGrabSuccess is called after a successful grab to schedule backup.
	// nil means backup is skipped (backward-compatible).
	OnGrabSuccess func(ctx context.Context, task repo.BootInstance, result *GrabResult)
}

// Engine is the grab engine instance. It owns the dual pool, the single-flight
// map, and the task key dedup map.
type Engine struct {
	cfg  EngineConfig
	deps EngineDeps

	pool *dualPool

	// activeTaskKeys is the single-flight map: key = tenancy_region_arch, value = bootId.
	// Populated during dedup; entries removed when the task completes/fails/times out.
	activeTaskKeys sync.Map

	// running flag; set to false during shutdown.
	running atomic.Bool
}

// NewEngine creates the grab engine and cleans up stale PROCESSING locks.
func NewEngine(cfg EngineConfig, deps EngineDeps) (*Engine, error) {
	cfg.normalize()
	e := &Engine{
		cfg:  cfg,
		deps: deps,
		pool: newDualPool(cfg.Pool),
	}
	e.running.Store(true)

	// Clean up stale PROCESSING locks from a previous crash.
	ctx := context.Background()
	if err := repo.New(e.deps.Store.Write).DeleteProcessingLocks(ctx); err != nil {
		deps.Logger.Warn().Err(err).Msg("grabber: failed to clean stale processing locks")
	}
	return e, nil
}

// CheckAndExecuteTasksOnce is the per-tick scheduling entry point (parity with
// CreateInstanceTaskV2.checkAndExecuteTasksOnce). Called by the scheduler
// every 6s. It finds expired tasks, deduplicates by tenant+arch, loads
// tenants, and submits each task to the parent pool.
func (e *Engine) CheckAndExecuteTasksOnce(ctx context.Context) {
	if !e.running.Load() {
		return
	}

	start := time.Now()
	q := repo.New(e.deps.Store.Read)
	now := time.Now().Format("2006-01-02 15:04:05")

	tasks, err := q.FindDistinctTasksToExecute(ctx, repo.FindDistinctTasksToExecuteParams{
		NextExecutionTime: sql.NullString{String: now, Valid: true},
		Limit:             int64(e.cfg.BatchSize),
	})
	if err != nil {
		e.deps.Logger.Error().Err(err).Msg("grabber: find tasks")
		return
	}
	if len(tasks) == 0 {
		return
	}

	e.deps.Logger.Debug().Int("count", len(tasks)).Msg("grabber: found expired tasks")

	// Load tenants for dedup keys.
	tenantIDs := make(map[int64]bool)
	for _, t := range tasks {
		if t.TenantID.Valid {
			tenantIDs[t.TenantID.Int64] = true
		}
	}
	tenantMap := make(map[int64]repo.Tenant, len(tenantIDs))
	for tid := range tenantIDs {
		ten, err := repo.New(e.deps.Store.Read).FindTenantByID(ctx, tid)
		if err != nil {
			continue
		}
		tenantMap[tid] = ten
	}

	// Deduplicate and submit to parent pool.
	deduped := e.deduplicateTasks(tasks, tenantMap)
	for _, task := range deduped {
		t := task // capture
		ok := e.pool.submitParent(func() {
			e.processTask(ctx, t)
		})
		if !ok {
			// Pool full or closed — release the key.
			e.removeTaskKey(t.BootID)
		}
	}

	e.deps.Logger.Debug().
		Int("total", len(tasks)).
		Int("deduped", len(deduped)).
		Dur("elapsed", time.Since(start)).
		Msg("grabber: schedule complete")
}

// deduplicateTasks filters tasks by single-flight key (tenancy+region+arch).
// For each unique key, only the earliest-expiring task is kept; subsequent
// tasks for the same key are skipped this round. Also skips tasks whose key
// is already held by an in-flight grab.
func (e *Engine) deduplicateTasks(
	tasks []repo.FindDistinctTasksToExecuteRow,
	tenantMap map[int64]repo.Tenant,
) map[string]repo.FindDistinctTasksToExecuteRow {
	out := make(map[string]repo.FindDistinctTasksToExecuteRow)
	for _, task := range tasks {
		if !task.TenantID.Valid {
			continue
		}
		ten, ok := tenantMap[task.TenantID.Int64]
		if !ok {
			continue
		}
		key := e.taskKey(ten, task.Architecture)
		if _, exists := e.activeTaskKeys.Load(key); exists {
			// Key held by an in-flight grab — skip entire key this round.
			continue
		}
		if _, seen := out[key]; seen {
			// Already picked the earliest task for this key.
			continue
		}
		e.activeTaskKeys.Store(key, ns(task.BootID))
		out[key] = task
	}
	return out
}

// processTask runs in the parent pool (lightweight). It validates the task,
// advances next_execution_time to prevent re-selection, checks the time-window
// gate, resolves the tenant+OCI credentials, and submits to the API pool.
func (e *Engine) processTask(ctx context.Context, task repo.FindDistinctTasksToExecuteRow) {
	q := repo.New(e.deps.Store.Write)
	bootID := ns(task.BootID)

	defer func() {
		if r := recover(); r != nil {
			e.deps.Logger.Error().Interface("panic", r).Str("bootId", bootID).Msg("grabber: processTask panic")
			e.removeTaskKey(task.BootID)
		}
	}()

	// Re-fetch the task to check current status.
	latest, err := q.FindBootInstanceByID(ctx, task.ID)
	if err != nil {
		e.removeTaskKey(task.BootID)
		return
	}
	if ni(latest.Status) != 1 {
		e.removeTaskKey(latest.BootID)
		return
	}

	// Advance next_execution_time to prevent re-pick.
	loopTime := ni(latest.LoopTime)
	if loopTime <= 0 {
		loopTime = 6
	}
	nextTime := time.Now().Add(time.Duration(loopTime) * time.Second).Format("2006-01-02 15:04:05")
	if err := q.AdvanceNextExecutionTime(ctx, repo.AdvanceNextExecutionTimeParams{
		NextExecutionTime: sql.NullString{String: nextTime, Valid: true},
		ID:                latest.ID,
	}); err != nil {
		e.deps.Logger.Warn().Err(err).Str("bootId", bootID).Msg("grabber: advance time failed")
		e.removeTaskKey(latest.BootID)
		return
	}

	// Check time-window gate (data_gap field: "HH:MM-HH:MM").
	if !isCurrentHourInRange(ns(latest.DataGap)) {
		e.removeTaskKey(latest.BootID)
		return
	}

	// Resolve tenant credentials.
	if !latest.TenantID.Valid {
		e.removeTaskKey(latest.BootID)
		return
	}
	tenant, err := repo.New(e.deps.Store.Read).FindTenantByID(ctx, latest.TenantID.Int64)
	if err != nil {
		e.deps.Logger.Warn().Err(err).Int64("tenantId", latest.TenantID.Int64).Msg("grabber: tenant not found")
		e.removeTaskKey(latest.BootID)
		return
	}

	creds := tenantToCreds(tenant)

	// Submit the actual OCI grab to the API pool (blocking admit — we
	// already passed the dedup gate; this must not be dropped).
	e.pool.runAPI(func() {
		e.executeGrabTask(ctx, latest, creds)
	})
}

// executeGrabTask runs in the API pool. It wraps the OCI launch call in an
// 80s non-blocking timeout. On completion, success/failure handlers are called.
// ALL paths release the single-flight key (via defer).
func (e *Engine) executeGrabTask(ctx context.Context, task repo.BootInstance, creds oci.Credentials) {
	bootID := ns(task.BootID)
	tctx, cancel := context.WithTimeout(ctx, time.Duration(e.cfg.Pool.ApiTimeout)*time.Second)
	defer cancel()
	defer e.removeTaskKey(task.BootID)

	e.deps.Logger.Debug().Str("bootId", bootID).Msg("grabber: executing grab")

	done := make(chan *GrabResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.deps.Logger.Error().Interface("panic", r).Str("bootId", bootID).Msg("grabber: launch panic")
				done <- &GrabResult{TaskID: bootID, Success: false, Error: fmt.Sprintf("panic: %v", r)}
			}
		}()
		result := e.launchInstance(tctx, task, creds)
		done <- result
	}()

	select {
	case result := <-done:
		if result.Success {
			e.onGrabSuccess(ctx, task, result)
		} else {
			e.onGrabFailure(ctx, task, result)
		}
	case <-tctx.Done():
		result := &GrabResult{TaskID: bootID, Success: false, Error: fmt.Sprintf("timeout (%ds)", e.cfg.Pool.ApiTimeout)}
		e.deps.Logger.Warn().Str("bootId", bootID).Msg("grabber: grab timeout")
		e.onGrabFailure(ctx, task, result)
	}
}

// removeTaskKey releases the single-flight key for tasks whose value matches
// the given bootId. Uses the bootId-based removal strategy (parity with V3:
// removeIf value == bootId), avoiding a DB re-query for the key.
func (e *Engine) removeTaskKey(bootID sql.NullString) {
	id := ns(bootID)
	if id == "" {
		return
	}
	var found bool
	e.activeTaskKeys.Range(func(k, v any) bool {
		if v.(string) == id {
			e.activeTaskKeys.Delete(k)
			found = true
			return false
		}
		return true
	})
	if found {
		e.deps.Logger.Debug().Str("bootId", id).Msg("grabber: released key")
	}
}

// SystemStatus returns pool metrics and task counts for the monitoring endpoint.
func (e *Engine) SystemStatus(ctx context.Context) map[string]any {
	runningCount, _ := repo.New(e.deps.Store.Read).CountRunningTasks(ctx)
	totalCount, _ := repo.New(e.deps.Store.Read).CountTotalTasks(ctx)

	activeKeyCount := 0
	e.activeTaskKeys.Range(func(_, _ any) bool { activeKeyCount++; return true })

	return map[string]any{
		"totalTasks":    totalCount,
		"runningTasks":  runningCount,
		"activeKeyCount": activeKeyCount,
		"parentPool":    e.pool.parentMetrics(),
		"apiPool":       e.pool.apiMetrics(),
		"batchSize":     e.cfg.BatchSize,
		"running":       e.running.Load(),
	}
}

// Shutdown stops accepting new tasks and drains the pools.
func (e *Engine) Shutdown() {
	e.deps.Logger.Info().Msg("grabber: shutting down")
	e.running.Store(false)
	e.pool.shutdown()
	e.deps.Logger.Info().Msg("grabber: stopped")
}

// taskKey builds the single-flight key: tenancy + "_" + region + "_" + arch.
func (e *Engine) taskKey(tenant repo.Tenant, arch sql.NullString) string {
	return ns(tenant.Tenancy) + "_" + ns(tenant.Region) + "_" + ns(arch)
}

// tenantToCreds extracts OCI credentials from a repo.Tenant.
func tenantToCreds(t repo.Tenant) oci.Credentials {
	return oci.Credentials{
		Tenancy:     ns(t.Tenancy),
		UserID:      ns(t.TenantID),
		Fingerprint: ns(t.Fingerprint),
		Region:      ns(t.Region),
		KeyFileBlob: ns(t.KeyFileBlob),
		KeyFile:     ns(t.KeyFile),
	}
}

// --- helpers (shared across grabber package) ---

func ns(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func ni(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

// isCurrentHourInRange checks if the current hour falls within the data_gap
// range (format: "HH:MM-HH:MM"). Empty/NO means always in range.
func isCurrentHourInRange(dataGap string) bool {
	if dataGap == "" || dataGap == "NO" {
		return true
	}
	// Simple implementation: parse "HH:MM-HH:MM".
	var startH, startM, endH, endM int
	n, err := fmt.Sscanf(dataGap, "%d:%d-%d:%d", &startH, &startM, &endH, &endM)
	if n != 4 || err != nil {
		return true // malformed → allow
	}
	now := time.Now()
	nowMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	if startMinutes <= endMinutes {
		return nowMinutes >= startMinutes && nowMinutes <= endMinutes
	}
	// Overnight range (e.g. 22:00-06:00).
	return nowMinutes >= startMinutes || nowMinutes <= endMinutes
}

