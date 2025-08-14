package memory

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// PoolEvent represents a diagnostic event for the pool
// Type can be "grow", "shrink", "reset", "custom"
type PoolEvent struct {
	Time    time.Time
	Type    string
	Message string
	OldSize int
	NewSize int
	Stats   PoolStats
}

// Pool represents a memory pool with automatic sizing and monitoring
// Now supports custom object types and diagnostics
type Pool struct {
	name       string
	objectSize int
	maxObjects int64
	allocCount int64
	hitCount   int64
	missCount  int64

	pool    sync.Pool
	objects chan interface{}
	stats   *PoolStats
	monitor *PoolMonitor

	// Configuration
	enableMonitoring bool
	enableAutoResize bool
	resizeThreshold  float64
	maxGrowthRate    float64

	factory     func() interface{} // Custom object factory
	diagnostics []PoolEvent        // Recent diagnostic events
	diagMu      sync.Mutex         // Protect diagnostics
}

// PoolStats holds pool statistics
type PoolStats struct {
	Name           string    `json:"name"`
	ObjectSize     int       `json:"object_size"`
	MaxObjects     int64     `json:"max_objects"`
	CurrentObjects int64     `json:"current_objects"`
	AllocCount     int64     `json:"alloc_count"`
	HitCount       int64     `json:"hit_count"`
	MissCount      int64     `json:"miss_count"`
	HitRatio       float64   `json:"hit_ratio"`
	MemoryUsage    int64     `json:"memory_usage_bytes"`
	LastResized    time.Time `json:"last_resized"`
	CreatedAt      time.Time `json:"created_at"`
}

// PoolConfig configures a memory pool
type PoolConfig struct {
	Name             string
	ObjectSize       int
	InitialObjects   int
	MaxObjects       int
	EnableMonitoring bool
	EnableAutoResize bool
	ResizeThreshold  float64 // Hit ratio threshold for resizing
	MaxGrowthRate    float64 // Maximum growth rate per resize
	Factory          func() interface{}
	MonitorInterval  time.Duration // Optional: for tests
}

// NewPool creates a new memory pool (now supports custom object types)
func NewPool(config PoolConfig) *Pool {
	if config.ResizeThreshold == 0 {
		config.ResizeThreshold = 0.8 // 80% hit ratio threshold
	}
	if config.MaxGrowthRate == 0 {
		config.MaxGrowthRate = 0.5 // 50% growth per resize
	}

	p := &Pool{
		name:             config.Name,
		objectSize:       config.ObjectSize,
		maxObjects:       int64(config.MaxObjects),
		enableMonitoring: config.EnableMonitoring,
		enableAutoResize: config.EnableAutoResize,
		resizeThreshold:  config.ResizeThreshold,
		maxGrowthRate:    config.MaxGrowthRate,
		stats: &PoolStats{
			Name:       config.Name,
			ObjectSize: config.ObjectSize,
			MaxObjects: int64(config.MaxObjects),
			CreatedAt:  time.Now(),
		},
	}

	// Use custom factory if provided
	if config.Factory != nil {
		p.factory = config.Factory
		p.pool.New = config.Factory
	} else {
		p.factory = func() interface{} { return make([]byte, config.ObjectSize) }
		p.pool.New = func() interface{} {
			atomic.AddInt64(&p.allocCount, 1)
			atomic.AddInt64(&p.missCount, 1)
			return make([]byte, config.ObjectSize)
		}
	}

	// Pre-allocate initial objects
	if config.InitialObjects > 0 {
		p.objects = make(chan interface{}, config.InitialObjects)
		for i := 0; i < config.InitialObjects; i++ {
			p.objects <- make([]byte, config.ObjectSize)
		}
	}

	// Start monitoring if enabled
	if config.EnableMonitoring {
		interval := config.MonitorInterval
		if interval == 0 {
			interval = 30 * time.Second
		}
		p.monitor = NewPoolMonitorWithInterval(p, interval)
		go p.monitor.Start()
	}

	return p
}

// NewPoolMonitorWithInterval creates a new pool monitor with custom interval
func NewPoolMonitorWithInterval(pool *Pool, interval time.Duration) *PoolMonitor {
	return &PoolMonitor{
		pool:     pool,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Get retrieves an object from the pool (interface{} for custom support)
func (p *Pool) Get() interface{} {
	var obj interface{}
	if p.objects != nil {
		select {
		case obj = <-p.objects:
			atomic.AddInt64(&p.hitCount, 1)
		default:
			obj = p.pool.Get()
		}
	} else {
		obj = p.pool.Get()
	}
	// For []byte pools, reset as before
	if p.factory != nil && p.objectSize > 0 {
		if buf, ok := obj.([]byte); ok {
			if len(buf) != p.objectSize {
				buf = make([]byte, p.objectSize)
				atomic.AddInt64(&p.allocCount, 1)
			} else {
				for i := range buf {
					buf[i] = 0
				}
			}
			return buf
		}
	}
	return obj
}

// GetBytes is a helper for []byte pools
func (p *Pool) GetBytes() []byte {
	obj := p.Get()
	if buf, ok := obj.([]byte); ok {
		return buf
	}
	return nil
}

// Put returns an object to the pool (interface{} for custom support)
func (p *Pool) Put(obj interface{}) {
	if p.objectSize > 0 {
		if buf, ok := obj.([]byte); ok {
			if len(buf) != p.objectSize {
				return
			}
		}
	}
	if p.objects != nil {
		select {
		case p.objects <- obj:
			return
		default:
		}
	}
	p.pool.Put(obj)
}

// AddEvent logs a diagnostic event
func (p *Pool) AddEvent(event PoolEvent) {
	p.diagMu.Lock()
	defer p.diagMu.Unlock()
	if len(p.diagnostics) > 100 {
		p.diagnostics = p.diagnostics[1:]
	}
	p.diagnostics = append(p.diagnostics, event)
}

// GetEvents returns recent diagnostic events
func (p *Pool) GetEvents() []PoolEvent {
	p.diagMu.Lock()
	defer p.diagMu.Unlock()
	return append([]PoolEvent(nil), p.diagnostics...)
}

// GetStats returns current pool statistics
func (p *Pool) GetStats() *PoolStats {
	stats := *p.stats
	stats.AllocCount = atomic.LoadInt64(&p.allocCount)
	stats.HitCount = atomic.LoadInt64(&p.hitCount)
	stats.MissCount = atomic.LoadInt64(&p.missCount)

	total := stats.HitCount + stats.MissCount
	if total > 0 {
		stats.HitRatio = float64(stats.HitCount) / float64(total)
	}

	if p.objects != nil {
		stats.CurrentObjects = int64(len(p.objects))
	}

	stats.MemoryUsage = stats.CurrentObjects * int64(p.objectSize)

	return &stats
}

// ResetAndDrain fully drains and resets the pool
func (p *Pool) ResetAndDrain() {
	p.Clear()
	if p.objects != nil {
		for len(p.objects) < cap(p.objects) {
			p.objects <- p.factory()
		}
	}
	p.AddEvent(PoolEvent{
		Time:    time.Now(),
		Type:    "reset",
		Message: "Pool fully reset and drained",
		Stats:   *p.GetStats(),
	})
}

// Resize adjusts the pool size based on usage patterns
func (p *Pool) Resize(newSize int) {
	if !p.enableAutoResize || newSize <= 0 {
		return
	}

	// Create new buffered channel if size increased
	if p.objects == nil || newSize > cap(p.objects) {
		oldObjects := p.objects
		p.objects = make(chan interface{}, newSize)

		// Transfer existing objects
		if oldObjects != nil {
			for {
				select {
				case obj := <-oldObjects:
					select {
					case p.objects <- obj:
					default:
						// New channel full, put back to sync.Pool
						p.pool.Put(obj)
					}
				default:
					goto done
				}
			}
		}
	done:

		// Fill remaining slots with new objects
		for len(p.objects) < newSize {
			p.objects <- make([]byte, p.objectSize)
		}

		atomic.StoreInt64(&p.maxObjects, int64(newSize))
		p.stats.LastResized = time.Now()
		// Reset hit/miss counters after resize for adaptive logic
		atomic.StoreInt64(&p.hitCount, 0)
		atomic.StoreInt64(&p.missCount, 0)
	}
}

// Clear empties the pool
func (p *Pool) Clear() {
	if p.objects != nil {
		for {
			select {
			case <-p.objects:
			default:
				return
			}
		}
	}
}

// Close closes the pool and stops monitoring
func (p *Pool) Close() {
	if p.monitor != nil {
		p.monitor.Stop()
	}
	p.Clear()
}

// PoolMonitor monitors pool performance and triggers resizing
type PoolMonitor struct {
	pool     *Pool
	interval time.Duration
	stopCh   chan struct{}
	running  int32
}

// NewPoolMonitor creates a new pool monitor
func NewPoolMonitor(pool *Pool) *PoolMonitor {
	return &PoolMonitor{
		pool:     pool,
		interval: 30 * time.Second, // Monitor every 30 seconds
		stopCh:   make(chan struct{}),
	}
}

// Start begins monitoring the pool
func (pm *PoolMonitor) Start() {
	if !atomic.CompareAndSwapInt32(&pm.running, 0, 1) {
		return // Already running
	}

	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.checkAndResize()
		case <-pm.stopCh:
			return
		}
	}
}

// Stop stops the pool monitor
func (pm *PoolMonitor) Stop() {
	if atomic.CompareAndSwapInt32(&pm.running, 1, 0) {
		close(pm.stopCh)
	}
}

// checkAndResize checks pool performance and resizes if needed
func (pm *PoolMonitor) checkAndResize() {
	stats := pm.pool.GetStats()

	// Only resize if we have enough data points
	if stats.AllocCount < 100 {
		return
	}

	currentSize := int(stats.MaxObjects)
	missRate := float64(stats.MissCount) / float64(stats.AllocCount)
	growthFactor := 1.0 + (missRate * pm.pool.maxGrowthRate)
	shrinkFactor := 1.0 - (stats.HitRatio * pm.pool.maxGrowthRate)

	// Grow if hit ratio is low (misses are high)
	if stats.HitRatio < pm.pool.resizeThreshold {
		newSize := int(float64(currentSize) * growthFactor)
		maxSize := currentSize * 2
		if newSize > maxSize {
			newSize = maxSize
		}
		if newSize > currentSize {
			pm.pool.Resize(newSize)
			msg := fmt.Sprintf("Pool '%s' grown to %d objects (hit ratio: %.2f, miss rate: %.2f)", stats.Name, newSize, stats.HitRatio, missRate)
			fmt.Println("[PoolMonitor]", msg)
			pm.pool.AddEvent(PoolEvent{
				Time:    time.Now(),
				Type:    "grow",
				Message: msg,
				OldSize: currentSize,
				NewSize: newSize,
				Stats:   *stats,
			})
		}
		return
	}

	// Shrink if hit ratio is very high (pool is overprovisioned)
	if stats.HitRatio > 0.98 && currentSize > 32 { // Don't shrink below 32
		newSize := int(float64(currentSize) * shrinkFactor)
		minSize := 32
		if newSize < minSize {
			newSize = minSize
		}
		if newSize < currentSize {
			pm.pool.Resize(newSize)
			msg := fmt.Sprintf("Pool '%s' shrunk to %d objects (hit ratio: %.2f)", stats.Name, newSize, stats.HitRatio)
			fmt.Println("[PoolMonitor]", msg)
			pm.pool.AddEvent(PoolEvent{
				Time:    time.Now(),
				Type:    "shrink",
				Message: msg,
				OldSize: currentSize,
				NewSize: newSize,
				Stats:   *stats,
			})
		}
	}
}

// Global pool manager for common buffer sizes
type PoolManager struct {
	pools map[int]*Pool
	mutex sync.RWMutex
}

var globalPoolManager = &PoolManager{
	pools: make(map[int]*Pool),
}

// GetGlobalPool returns a global pool for the specified buffer size
func GetGlobalPool(size int) *Pool {
	globalPoolManager.mutex.RLock()
	pool, exists := globalPoolManager.pools[size]
	globalPoolManager.mutex.RUnlock()

	if exists {
		return pool
	}

	globalPoolManager.mutex.Lock()
	defer globalPoolManager.mutex.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := globalPoolManager.pools[size]; exists {
		return pool
	}

	// Create new pool
	config := PoolConfig{
		Name:             fmt.Sprintf("global-%d", size),
		ObjectSize:       size,
		InitialObjects:   50,
		MaxObjects:       1000,
		EnableMonitoring: true,
		EnableAutoResize: true,
	}

	pool = NewPool(config)
	globalPoolManager.pools[size] = pool

	return pool
}

// GetBuffer retrieves a buffer from the global pool
func GetBuffer(size int) []byte {
	poolSize := nextPowerOf2(size)
	pool := GetGlobalPool(poolSize)
	buf := pool.GetBytes()
	if buf == nil {
		return make([]byte, size)
	}
	return buf[:size]
}

// PutBuffer returns a buffer to the global pool
func PutBuffer(buf []byte) {
	if cap(buf) == 0 {
		return
	}

	poolSize := cap(buf)
	pool := GetGlobalPool(poolSize)

	// Reset slice to full capacity before returning
	fullBuf := buf[:cap(buf)]
	pool.Put(fullBuf)
}

// nextPowerOf2 returns the next power of 2 >= n
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}

	// Handle powers of 2
	if n&(n-1) == 0 {
		return n
	}

	// Find next power of 2
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// FastBuffer provides a zero-allocation buffer for small, frequent operations
type FastBuffer struct {
	data []byte
	pos  int
}

// NewFastBuffer creates a new fast buffer
func NewFastBuffer(size int) *FastBuffer {
	return &FastBuffer{
		data: make([]byte, size),
		pos:  0,
	}
}

// Write appends data to the buffer
func (fb *FastBuffer) Write(p []byte) (int, error) {
	if fb.pos+len(p) > len(fb.data) {
		// Buffer full, need to resize
		newSize := len(fb.data) * 2
		if newSize < fb.pos+len(p) {
			newSize = fb.pos + len(p)
		}

		newData := make([]byte, newSize)
		copy(newData, fb.data[:fb.pos])
		fb.data = newData
	}

	copy(fb.data[fb.pos:], p)
	fb.pos += len(p)
	return len(p), nil
}

// WriteByte writes a single byte
func (fb *FastBuffer) WriteByte(b byte) error {
	if fb.pos >= len(fb.data) {
		fb.grow()
	}
	fb.data[fb.pos] = b
	fb.pos++
	return nil
}

// WriteString writes a string
func (fb *FastBuffer) WriteString(s string) {
	// Use unsafe to avoid string->[]byte allocation
	if fb.pos+len(s) > len(fb.data) {
		fb.growTo(fb.pos + len(s))
	}

	copy(fb.data[fb.pos:], *(*[]byte)(unsafe.Pointer(&s)))
	fb.pos += len(s)
}

// Bytes returns the current buffer contents
func (fb *FastBuffer) Bytes() []byte {
	return fb.data[:fb.pos]
}

// Reset resets the buffer for reuse
func (fb *FastBuffer) Reset() {
	fb.pos = 0
}

// Len returns the current buffer length
func (fb *FastBuffer) Len() int {
	return fb.pos
}

// Cap returns the buffer capacity
func (fb *FastBuffer) Cap() int {
	return len(fb.data)
}

// grow increases buffer size by 2x
func (fb *FastBuffer) grow() {
	fb.growTo(len(fb.data) * 2)
}

// growTo increases buffer size to specific size
func (fb *FastBuffer) growTo(size int) {
	newData := make([]byte, size)
	copy(newData, fb.data[:fb.pos])
	fb.data = newData
}

// MemoryStats provides system memory statistics
type MemoryStats struct {
	Alloc         uint64  // Bytes allocated and in use
	TotalAlloc    uint64  // Total bytes allocated (even if freed)
	Sys           uint64  // Bytes obtained from system
	Lookups       uint64  // Number of pointer lookups
	Mallocs       uint64  // Number of mallocs
	Frees         uint64  // Number of frees
	HeapAlloc     uint64  // Bytes allocated and in use
	HeapSys       uint64  // Bytes obtained from system
	HeapIdle      uint64  // Bytes in idle spans
	HeapInuse     uint64  // Bytes in in-use spans
	HeapReleased  uint64  // Bytes released to OS
	GCCPUFraction float64 // Fraction of CPU time used by GC
	NumGC         uint32  // Number of completed GC cycles
}

// GetMemoryStats returns current memory statistics
func GetMemoryStats() *MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		Lookups:       m.Lookups,
		Mallocs:       m.Mallocs,
		Frees:         m.Frees,
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		HeapReleased:  m.HeapReleased,
		GCCPUFraction: m.GCCPUFraction,
		NumGC:         m.NumGC,
	}
}

// ForceGC forces a garbage collection cycle
func ForceGC() {
	runtime.GC()
}

// SetGCPercent sets the garbage collection target percentage
func SetGCPercent(percent int) int {
	return debug.SetGCPercent(percent)
}

// High-Performance Extensions for Phase 2C

// MessagePool is a high-performance pool for message objects
type MessagePool struct {
	pool     sync.Pool
	gets     int64
	puts     int64
	hits     int64
	misses   int64
	capacity int64
}

// Message represents a pooled message object
type Message struct {
	ID        uint64
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string][]byte
	Timestamp int64
	Checksum  uint32
	pooled    bool
}

// NewMessagePool creates a new message pool
func NewMessagePool(capacity int64) *MessagePool {
	mp := &MessagePool{
		capacity: capacity,
	}

	mp.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&mp.misses, 1)
			return &Message{
				Headers: make(map[string][]byte),
				pooled:  true,
			}
		},
	}

	return mp
}

// Get retrieves a message from the pool
func (mp *MessagePool) Get() *Message {
	atomic.AddInt64(&mp.gets, 1)
	msg := mp.pool.Get().(*Message)
	if msg.pooled {
		atomic.AddInt64(&mp.hits, 1)
	}
	msg.pooled = false
	return msg
}

// Put returns a message to the pool
func (mp *MessagePool) Put(msg *Message) {
	if msg == nil {
		return
	}

	// Reset message for reuse
	msg.Reset()
	msg.pooled = true

	atomic.AddInt64(&mp.puts, 1)
	mp.pool.Put(msg)
}

// Reset resets a message for reuse
func (m *Message) Reset() {
	m.ID = 0
	m.Topic = ""
	m.Partition = 0
	m.Offset = 0
	m.Key = m.Key[:0]     // Keep underlying array
	m.Value = m.Value[:0] // Keep underlying array
	m.Timestamp = 0
	m.Checksum = 0

	// Clear headers map but keep it allocated
	for k := range m.Headers {
		delete(m.Headers, k)
	}
}

// Stats returns pool statistics
func (mp *MessagePool) Stats() PoolStats {
	gets := atomic.LoadInt64(&mp.gets)
	_ = atomic.LoadInt64(&mp.puts) // Track puts but not used in current stats
	hits := atomic.LoadInt64(&mp.hits)
	misses := atomic.LoadInt64(&mp.misses)

	var hitRatio float64
	if gets > 0 {
		hitRatio = float64(hits) / float64(gets)
	}

	return PoolStats{
		Name:           "MessagePool",
		ObjectSize:     64, // Approximate message size
		MaxObjects:     mp.capacity,
		CurrentObjects: 0, // Would need tracking
		AllocCount:     gets,
		HitCount:       hits,
		MissCount:      misses,
		HitRatio:       hitRatio,
		MemoryUsage:    mp.capacity * 64,
		LastResized:    time.Now(),
		CreatedAt:      time.Now(),
	}
}

// BufferPool manages byte buffer pools for different sizes
type BufferPool struct {
	pools map[int]*sync.Pool
	sizes []int
	mu    sync.RWMutex
	stats map[int]int64 // usage count per size
}

func NewBufferPool() *BufferPool {
	sizes := []int{32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}
	pools := make(map[int]*sync.Pool)
	stats := make(map[int]int64)
	for _, size := range sizes {
		size := size // capture for closure
		pools[size] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, size)
			},
		}
		stats[size] = 0
	}
	return &BufferPool{
		pools: pools,
		sizes: sizes,
		stats: stats,
	}
}

func (bp *BufferPool) Get(size int) []byte {
	poolSize := bp.findPoolSize(size)
	if poolSize == 0 {
		return make([]byte, 0, size)
	}
	bp.mu.Lock()
	bp.stats[poolSize]++
	bp.mu.Unlock()
	bp.mu.RLock()
	pool := bp.pools[poolSize]
	bp.mu.RUnlock()
	buf := pool.Get().([]byte)
	return buf[:0]
}

func (bp *BufferPool) GetStats() map[int]int64 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	copyStats := make(map[int]int64)
	for k, v := range bp.stats {
		copyStats[k] = v
	}
	return copyStats
}

// findPoolSize finds the appropriate pool size for the given size
func (bp *BufferPool) findPoolSize(size int) int {
	for _, poolSize := range bp.sizes {
		if size <= poolSize {
			return poolSize
		}
	}
	return 0 // Too large
}

// put returns a buffer to the appropriate pool
func (bp *BufferPool) Put(buf []byte) {
	if len(buf) == 0 {
		return // Nothing to put back
	}
	poolSize := bp.findPoolSize(cap(buf))
	if poolSize == 0 {
		return // No appropriate pool
	}
	bp.mu.RLock()
	pool := bp.pools[poolSize]
	bp.mu.RUnlock()
	buf = buf[:0] // Reset slice to empty but keep capacity
	pool.Put(buf)
	bp.mu.Lock()
	bp.stats[poolSize]--
	bp.mu.Unlock()
}

// Usage example and best practices (as comments):
/*
// Example: Custom object pool
customPool := NewPool(PoolConfig{
	Name: "custom",
	ObjectSize: 0, // ignored
	Factory: func() interface{} { return &MyStruct{} },
})
obj := customPool.Get().(*MyStruct)
customPool.Put(obj)

// Example: Accessing diagnostics
for _, event := range customPool.GetEvents() {
	fmt.Println(event.Time, event.Type, event.Message)
}
*/
