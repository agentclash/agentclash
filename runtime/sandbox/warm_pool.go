package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// WarmPoolConfig configures an optional per-worker warm sandbox pool.
//
// v1 is intentionally per-process: replicas do not share warm sandboxes.
// Cross-replica sharing belongs with a future control-plane pool if needed.
type WarmPoolConfig struct {
	// Size is the target number of warm sandboxes per pool key. 0 disables.
	Size int
	// TTL is how long an unused warm sandbox may idle before Destroy. Default 10m.
	TTL time.Duration
	// FillTimeout bounds each background Create used to replenish the pool.
	FillTimeout time.Duration
	// Metrics receives pool hit/miss/fill/expire counters (optional).
	Metrics Metrics
	// Logger defaults to slog.Default when nil.
	Logger *slog.Logger
	// Clock overrides time.Now for tests.
	Clock func() time.Time
}

// WarmPool is a Provider decorator that checks out pre-created sessions keyed by
// (TemplateID, ToolPolicy hash) before falling through to the inner provider.
type WarmPool struct {
	fillCtx     context.Context
	stopFill    context.CancelFunc
	inner       Provider
	size        int
	ttl         time.Duration
	fillTimeout time.Duration
	metrics     Metrics
	logger      *slog.Logger
	clock       func() time.Time

	mu         sync.Mutex
	pools      map[string][]*warmEntry
	stopCh     chan struct{}
	stopped    bool
	closeDone  chan struct{}
	closeErr   error
	cleanupErr error
	wg         sync.WaitGroup
}

type warmEntry struct {
	session   Session
	request   CreateRequest
	createdAt time.Time
}

// WrapWarmPool returns inner unchanged when Size <= 0.
func WrapWarmPool(inner Provider, cfg WarmPoolConfig) *WarmPool {
	if inner == nil {
		inner = UnconfiguredProvider{}
	}
	if cfg.Size <= 0 {
		return nil
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	fillTimeout := cfg.FillTimeout
	if fillTimeout <= 0 {
		fillTimeout = 2 * time.Minute
	}
	metrics := cfg.Metrics
	if metrics == nil {
		metrics = NoopMetrics{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	fillCtx, stopFill := context.WithCancel(context.Background())
	return &WarmPool{
		fillCtx:     fillCtx,
		stopFill:    stopFill,
		inner:       inner,
		size:        cfg.Size,
		ttl:         ttl,
		fillTimeout: fillTimeout,
		metrics:     metrics,
		logger:      logger,
		clock:       clock,
		pools:       make(map[string][]*warmEntry),
		stopCh:      make(chan struct{}),
	}
}

// Start begins the idle-expiry loop. Safe to call once.
func (p *WarmPool) Start() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.expireIdle()
			}
		}
	}()
}

// Close stops the filler/expiry loops and destroys remaining warm sessions.
func (p *WarmPool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return p.awaitClose(ctx)
	}
	p.closeDone = make(chan struct{})
	p.stopped = true
	p.stopFill()
	close(p.stopCh)
	entries := make([]*warmEntry, 0)
	for key, list := range p.pools {
		entries = append(entries, list...)
		delete(p.pools, key)
	}
	p.mu.Unlock()
	// Wait for tracked fills/expiry and destroy the idle sessions concurrently.
	// The caller's deadline also bounds providers that ignore cancellation.
	go func() {
		var cleanup sync.WaitGroup
		errs := make(chan error, len(entries))
		for _, entry := range entries {
			cleanup.Add(1)
			go func() {
				defer cleanup.Done()
				errs <- entry.session.Destroy(ctx)
				p.metrics.WarmPoolExpire()
			}()
		}
		cleanup.Wait()
		p.wg.Wait()
		close(errs)
		var firstErr error
		for err := range errs {
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		p.mu.Lock()
		p.closeErr = errors.Join(firstErr, p.cleanupErr)
		close(p.closeDone)
		p.mu.Unlock()
	}()
	return p.awaitClose(ctx)
}

func (p *WarmPool) awaitClose(ctx context.Context) error {
	select {
	case <-p.closeDone:
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *WarmPool) Create(ctx context.Context, request CreateRequest) (Session, error) {
	if p == nil {
		return nil, ErrProviderNotConfigured
	}
	if !p.begin() {
		return nil, errors.New("warm pool is closed")
	}
	defer p.wg.Done()
	key := PoolKey(request)
	if session, ok := p.checkout(key, request); ok {
		p.metrics.WarmPoolHit()
		p.scheduleFill(key, request)
		return session, nil
	}
	p.metrics.WarmPoolMiss()
	session, err := p.inner.Create(ctx, request)
	if err != nil {
		return nil, err
	}
	p.scheduleFill(key, request)
	return session, nil
}

// EnsureWarm pre-fills the pool for request's key up to Size (best-effort).
func (p *WarmPool) EnsureWarm(ctx context.Context, request CreateRequest) {
	if p == nil {
		return
	}
	if !p.begin() {
		return
	}
	defer p.wg.Done()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(p.fillCtx, cancel)
	defer stop()
	key := PoolKey(request)
	for p.poolLen(key) < p.size {
		if err := ctx.Err(); err != nil {
			return
		}
		if !p.fillOne(ctx, key, request) {
			return
		}
	}
}

func (p *WarmPool) begin() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return false
	}
	p.wg.Add(1)
	return true
}

func (p *WarmPool) checkout(key string, request CreateRequest) (Session, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.pools[key]
	for len(list) > 0 {
		entry := list[0]
		list = list[1:]
		p.pools[key] = list
		if p.clock().Sub(entry.createdAt) > p.ttl {
			p.destroyAsyncLocked(entry.session)
			p.metrics.WarmPoolExpire()
			continue
		}
		if !poolRequestsMatch(entry.request, request) {
			p.destroyAsyncLocked(entry.session)
			p.metrics.WarmPoolExpire()
			continue
		}
		return entry.session, true
	}
	return nil, false
}

func (p *WarmPool) scheduleFill(key string, request CreateRequest) {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.wg.Add(1)
	p.mu.Unlock()
	go func() {
		defer p.wg.Done()
		ctx, cancel := context.WithTimeout(p.fillCtx, p.fillTimeout)
		defer cancel()
		for p.poolLen(key) < p.size {
			select {
			case <-p.stopCh:
				return
			default:
			}
			if !p.fillOne(ctx, key, request) {
				return
			}
		}
	}()
}

func (p *WarmPool) fillOne(ctx context.Context, key string, request CreateRequest) bool {
	session, err := p.inner.Create(ctx, cloneCreateRequest(request))
	if err != nil {
		p.logger.Warn("warm pool fill failed", "pool_key", key, "error", err)
		return false
	}
	p.mu.Lock()
	if p.stopped || len(p.pools[key]) >= p.size {
		p.mu.Unlock()
		// Stay inside the tracked fill until a late-created session is destroyed.
		p.destroyQuiet(session)
		return false
	}
	defer p.mu.Unlock()
	p.pools[key] = append(p.pools[key], &warmEntry{
		session:   session,
		request:   cloneCreateRequest(request),
		createdAt: p.clock(),
	})
	p.metrics.WarmPoolFill()
	return true
}

func (p *WarmPool) poolLen(key string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pools[key])
}

func (p *WarmPool) expireIdle() {
	now := p.clock()
	var expired []Session
	p.mu.Lock()
	for key, list := range p.pools {
		kept := list[:0]
		for _, entry := range list {
			if now.Sub(entry.createdAt) > p.ttl {
				expired = append(expired, entry.session)
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(p.pools, key)
		} else {
			p.pools[key] = kept
		}
	}
	p.mu.Unlock()
	for _, session := range expired {
		p.destroyQuiet(session)
		p.metrics.WarmPoolExpire()
	}
}

// Called with mu held, so Close cannot race Wait against this Add.
func (p *WarmPool) destroyAsyncLocked(session Session) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.destroyQuiet(session)
	}()
}

func (p *WarmPool) destroyQuiet(session Session) {
	if session == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := session.Destroy(ctx); err != nil {
		p.mu.Lock()
		if p.cleanupErr == nil {
			p.cleanupErr = err
		}
		p.mu.Unlock()
	}
}

type poolKeyPayload struct {
	TemplateID         string            `json:"template_id"`
	Timeout            int64             `json:"timeout_ns,omitempty"`
	ToolPolicy         ToolPolicy        `json:"tool_policy"`
	Filesystem         FilesystemSpec    `json:"filesystem"`
	Labels             map[string]string `json:"labels,omitempty"`
	EnvVars            map[string]string `json:"env_vars,omitempty"`
	NetworkAllowlist   []string          `json:"network_allowlist,omitempty"`
	AdditionalPackages []string          `json:"additional_packages,omitempty"`
}

func poolFingerprint(request CreateRequest) [32]byte {
	payload := poolKeyPayload{
		TemplateID:         request.TemplateID,
		Timeout:            int64(request.Timeout),
		ToolPolicy:         request.ToolPolicy,
		Filesystem:         request.Filesystem,
		Labels:             request.Labels,
		EnvVars:            request.EnvVars,
		NetworkAllowlist:   request.NetworkAllowlist,
		AdditionalPackages: request.AdditionalPackages,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte("{}")
	}
	return sha256.Sum256(data)
}

func poolRequestsMatch(a, b CreateRequest) bool {
	return poolFingerprint(a) == poolFingerprint(b)
}

// PoolKey returns a stable key for sandbox configuration (excludes RunID/RunAgentID).
func PoolKey(request CreateRequest) string {
	sum := poolFingerprint(request)
	return request.TemplateID + ":" + hex.EncodeToString(sum[:8])
}
