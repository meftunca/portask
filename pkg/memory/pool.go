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

// Pool represents a memory pool with automatic sizing and monitoring
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
}

// NewPool creates a new memory pool
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

	// Initialize sync.Pool with factory function
	p.pool.New = func() interface{} {
		atomic.AddInt64(&p.allocCount, 1)
		atomic.AddInt64(&p.missCount, 1)
		return make([]byte, config.ObjectSize)
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
		p.monitor = NewPoolMonitor(p)
		go p.monitor.Start()
	}

	return p
}

// Get retrieves an object from the pool
func (p *Pool) Get() []byte {
	var obj interface{}

	// Try to get from buffered channel first (faster)
	if p.objects != nil {
		select {
		case obj = <-p.objects:
			atomic.AddInt64(&p.hitCount, 1)
		default:
			// Channel empty, fall back to sync.Pool
			obj = p.pool.Get()
		}
	} else {
		obj = p.pool.Get()
	}

	// Reset the byte slice
	buf := obj.([]byte)
	if len(buf) != p.objectSize {
		// Size mismatch, create new buffer
		buf = make([]byte, p.objectSize)
		atomic.AddInt64(&p.allocCount, 1)
	} else {
		// Clear the buffer for reuse
		for i := range buf {
			buf[i] = 0
		}
	}

	return buf
}

// Put returns an object to the pool
func (p *Pool) Put(obj []byte) {
	if len(obj) != p.objectSize {
		// Wrong size, don't return to pool
		return
	}

	// Try to put back to buffered channel first
	if p.objects != nil {
		select {
		case p.objects <- obj:
			return
		default:
			// Channel full, fall back to sync.Pool
		}
	}

	p.pool.Put(obj)
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

	// Check if hit ratio is below threshold
	if stats.HitRatio < pm.pool.resizeThreshold {
		// Calculate new size based on miss rate
		currentSize := int(stats.MaxObjects)
		missRate := float64(stats.MissCount) / float64(stats.AllocCount)
		growthFactor := 1.0 + (missRate * pm.pool.maxGrowthRate)
		newSize := int(float64(currentSize) * growthFactor)

		// Cap the growth to prevent excessive memory usage
		maxSize := currentSize * 2
		if newSize > maxSize {
			newSize = maxSize
		}

		pm.pool.Resize(newSize)
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
	// Round up to next power of 2 for better pool utilization
	poolSize := nextPowerOf2(size)
	pool := GetGlobalPool(poolSize)
	buf := pool.Get()

	// Return slice of requested size
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
func (fb *FastBuffer) WriteByte(b byte) {
	if fb.pos >= len(fb.data) {
		fb.grow()
	}
	fb.data[fb.pos] = b
	fb.pos++
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
}

// NewBufferPool creates a new buffer pool with predefined sizes
func NewBufferPool() *BufferPool {
	sizes := []int{32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}
	pools := make(map[int]*sync.Pool)

	for _, size := range sizes {
		size := size // capture for closure
		pools[size] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, size)
			},
		}
	}

	return &BufferPool{
		pools: pools,
		sizes: sizes,
	}
}

// Get retrieves a buffer of at least the specified size
func (bp *BufferPool) Get(size int) []byte {
	poolSize := bp.findPoolSize(size)
	if poolSize == 0 {
		// Size too large, allocate directly
		return make([]byte, 0, size)
	}

	bp.mu.RLock()
	pool := bp.pools[poolSize]
	bp.mu.RUnlock()

	buf := pool.Get().([]byte)
	return buf[:0] // Reset length but keep capacity
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf []byte) {
	if buf == nil {
		return
	}

	capacity := cap(buf)
	poolSize := bp.findPoolSize(capacity)
	if poolSize == 0 {
		// Buffer too large for pooling
		return
	}

	bp.mu.RLock()
	pool := bp.pools[poolSize]
	bp.mu.RUnlock()

	pool.Put(buf)
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
