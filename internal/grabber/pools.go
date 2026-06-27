// Package grabber — pools.go implements the dual isolated goroutine pools
// (SPEC S8.3, S16). The parent pool handles scheduling/dispatch (lightweight);
// the API pool handles OCI I/O (heavy). Physical isolation via two separate
// buffered channels prevents the parent pool from self-exhausting and
// deadlocking waiting for child results.
package grabber

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// PoolConfig tunes the two pools. ParentSize and ApiSize default to
// CPU-scaled values when <= 0.
type PoolConfig struct {
	ParentSize int // scheduling goroutines (default: CPU/2, min 2, max 8)
	ApiSize    int // OCI API goroutines (default: CPU, min 4, max 16)
	ApiTimeout int // seconds (default 80)
}

func (c *PoolConfig) normalize() {
	if c.ApiTimeout <= 0 {
		c.ApiTimeout = 80
	}
	cpu := runtime.NumCPU()
	if cpu < 1 {
		cpu = 1
	}
	if c.ParentSize <= 0 {
		c.ParentSize = cpu / 2
		if c.ParentSize < 2 {
			c.ParentSize = 2
		}
		if c.ParentSize > 8 {
			c.ParentSize = 8
		}
	}
	if c.ApiSize <= 0 {
		c.ApiSize = cpu
		if c.ApiSize < 4 {
			c.ApiSize = 4
		}
		if c.ApiSize > 16 {
			c.ApiSize = 16
		}
	}
}

// PoolMetrics exposes pool stats for the /api/systemStatus endpoint.
type PoolMetrics struct {
	Active    int `json:"active"`
	Queue     int `json:"queue"`
	Size      int `json:"size"`
	Completed int64 `json:"completed"`
}

// dualPool holds the two isolated worker pools. Each is a semaphore over a
// buffered channel: sending a token acquires a slot, workers receive tokens
// and execute submitted functions.
type dualPool struct {
	parent    chan struct{}
	api       chan struct{}
	parentCfg int
	apiCfg    int

	parentCompleted atomic.Int64
	apiCompleted    atomic.Int64

	mu     sync.RWMutex
	closed bool
}

func newDualPool(cfg PoolConfig) *dualPool {
	cfg.normalize()
	p := &dualPool{
		parent:    make(chan struct{}, cfg.ParentSize),
		api:       make(chan struct{}, cfg.ApiSize),
		parentCfg: cfg.ParentSize,
		apiCfg:    cfg.ApiSize,
	}
	// Pre-fill tokens.
	for i := 0; i < cfg.ParentSize; i++ {
		p.parent <- struct{}{}
	}
	for i := 0; i < cfg.ApiSize; i++ {
		p.api <- struct{}{}
	}
	return p
}

// submitParent runs fn in the parent pool. Returns false if the pool is closed
// or full (non-blocking — overflow is dropped).
func (p *dualPool) submitParent(fn func()) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return false
	}
	select {
	case tok := <-p.parent:
		go func() {
			defer func() { p.parent <- tok }()
			fn()
			p.parentCompleted.Add(1)
		}()
		return true
	default:
		return false
	}
}

// submitAPI runs fn in the API pool. Returns false if closed or full.
func (p *dualPool) submitAPI(fn func()) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return false
	}
	select {
	case tok := <-p.api:
		go func() {
			defer func() { p.api <- tok }()
			fn()
			p.apiCompleted.Add(1)
		}()
		return true
	default:
		return false
	}
}

// runAPI waits for an API slot and runs fn. Blocks until a slot opens or
// pool is closed. Used for submissions that must not be dropped
// (e.g. the actual OCI launch call, which was already admitted by the
// parent-pool dedup gate).
func (p *dualPool) runAPI(fn func()) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()
	tok := <-p.api
	defer func() { p.api <- tok }()
	fn()
	p.apiCompleted.Add(1)
}

// shutdown drains both pools and prevents new submissions.
func (p *dualPool) shutdown() {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
}

func (p *dualPool) parentMetrics() PoolMetrics {
	return PoolMetrics{
		Active:    p.parentCfg - len(p.parent),
		Queue:     max(0, len(p.parent)-p.parentCfg),
		Size:      p.parentCfg,
		Completed: p.parentCompleted.Load(),
	}
}

func (p *dualPool) apiMetrics() PoolMetrics {
	return PoolMetrics{
		Active: p.apiCfg - len(p.api),
		Queue:  0, // api pool uses blocking runAPI, no queue
		Size:   p.apiCfg,
		Completed: p.apiCompleted.Load(),
	}
}

// TaskFn is the function signature for a grab task submission.
// ctx carries the 80s timeout; user carries the resolved tenant+task data.
type TaskFn func(ctx context.Context) error

// GrabResult holds the outcome of a single grab attempt.
type GrabResult struct {
	TaskID             string `json:"taskId"`
	Success            bool   `json:"success"`
	InstanceID         string `json:"instanceId,omitempty"`
	PublicIP           string `json:"publicIp,omitempty"`
	Error              string `json:"error,omitempty"`
	AvailabilityDomain string `json:"availabilityDomain,omitempty"`
	Shape              string `json:"shape,omitempty"`
	ImageID            string `json:"imageId,omitempty"`
	SubnetID           string `json:"subnetId,omitempty"`
	NsgID              string `json:"nsgId,omitempty"`
}

func sprintErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var _ = fmt.Sprintf // keep fmt import for future use
