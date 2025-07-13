package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// WALEntry represents a single entry in the write-ahead log
type WALEntry struct {
	Timestamp int64
	Size      uint32
	Checksum  uint32
	Data      []byte
}

// WALSegment represents a single WAL file segment
type WALSegment struct {
	ID     uint64
	File   *os.File
	Size   int64
	Synced bool
	mu     sync.RWMutex
}

// WALWriter implements write-ahead logging for durability
type WALWriter struct {
	basePath       string
	maxSegmentSize int64
	syncInterval   time.Duration
	currentSegment *WALSegment
	segments       map[uint64]*WALSegment
	nextSegmentID  uint64
	mu             sync.RWMutex
	syncTicker     *time.Ticker
	stopCh         chan struct{}
}

// WALConfig holds configuration for WAL
type WALConfig struct {
	BasePath       string
	MaxSegmentSize int64
	SyncInterval   time.Duration
}

// DefaultWALConfig returns default WAL configuration
func DefaultWALConfig() *WALConfig {
	return &WALConfig{
		BasePath:       "./data/wal",
		MaxSegmentSize: 64 * 1024 * 1024, // 64MB segments
		SyncInterval:   100 * time.Millisecond,
	}
}

// NewWALWriter creates a new WAL writer
func NewWALWriter(config *WALConfig) (*WALWriter, error) {
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	wal := &WALWriter{
		basePath:       config.BasePath,
		maxSegmentSize: config.MaxSegmentSize,
		syncInterval:   config.SyncInterval,
		segments:       make(map[uint64]*WALSegment),
		stopCh:         make(chan struct{}),
	}

	// Create initial segment
	if err := wal.rotateSegment(); err != nil {
		return nil, fmt.Errorf("failed to create initial segment: %w", err)
	}

	// Start sync goroutine
	wal.syncTicker = time.NewTicker(config.SyncInterval)
	go wal.syncLoop()

	return wal, nil
}

// Write writes data to the WAL and returns the offset
func (w *WALWriter) Write(data []byte) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate segment
	if w.currentSegment.Size >= w.maxSegmentSize {
		if err := w.rotateSegment(); err != nil {
			return 0, fmt.Errorf("failed to rotate segment: %w", err)
		}
	}

	entry := &WALEntry{
		Timestamp: time.Now().UnixNano(),
		Size:      uint32(len(data)),
		Checksum:  crc32Checksum(data),
		Data:      data,
	}

	offset := w.currentSegment.Size

	// Write entry header
	header := make([]byte, 16) // 8 + 4 + 4 bytes
	binary.LittleEndian.PutUint64(header[0:8], uint64(entry.Timestamp))
	binary.LittleEndian.PutUint32(header[8:12], entry.Size)
	binary.LittleEndian.PutUint32(header[12:16], entry.Checksum)

	if _, err := w.currentSegment.File.Write(header); err != nil {
		return 0, fmt.Errorf("failed to write entry header: %w", err)
	}

	// Write entry data
	if _, err := w.currentSegment.File.Write(entry.Data); err != nil {
		return 0, fmt.Errorf("failed to write entry data: %w", err)
	}

	w.currentSegment.Size += int64(16 + len(data))
	w.currentSegment.Synced = false

	return offset, nil
}

// WriteBatch writes multiple entries in a single operation
func (w *WALWriter) WriteBatch(dataSlices [][]byte) ([]int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	offsets := make([]int64, len(dataSlices))
	totalSize := int64(0)

	// Calculate total size needed
	for _, data := range dataSlices {
		totalSize += int64(16 + len(data)) // header + data
	}

	// Check if we need to rotate segment
	if w.currentSegment.Size+totalSize >= w.maxSegmentSize {
		if err := w.rotateSegment(); err != nil {
			return nil, fmt.Errorf("failed to rotate segment: %w", err)
		}
	}

	// Write all entries
	for i, data := range dataSlices {
		entry := &WALEntry{
			Timestamp: time.Now().UnixNano(),
			Size:      uint32(len(data)),
			Checksum:  crc32Checksum(data),
			Data:      data,
		}

		offsets[i] = w.currentSegment.Size

		// Write entry header
		header := make([]byte, 16)
		binary.LittleEndian.PutUint64(header[0:8], uint64(entry.Timestamp))
		binary.LittleEndian.PutUint32(header[8:12], entry.Size)
		binary.LittleEndian.PutUint32(header[12:16], entry.Checksum)

		if _, err := w.currentSegment.File.Write(header); err != nil {
			return nil, fmt.Errorf("failed to write entry header: %w", err)
		}

		// Write entry data
		if _, err := w.currentSegment.File.Write(entry.Data); err != nil {
			return nil, fmt.Errorf("failed to write entry data: %w", err)
		}

		w.currentSegment.Size += int64(16 + len(data))
	}

	w.currentSegment.Synced = false
	return offsets, nil
}

// Sync forces a sync of the current segment
func (w *WALWriter) Sync() error {
	w.mu.RLock()
	segment := w.currentSegment
	w.mu.RUnlock()

	if segment == nil {
		return nil
	}

	segment.mu.Lock()
	defer segment.mu.Unlock()

	if err := segment.File.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment: %w", err)
	}

	segment.Synced = true
	return nil
}

// Close closes the WAL writer
func (w *WALWriter) Close() error {
	close(w.stopCh)
	w.syncTicker.Stop()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Sync and close all segments
	for _, segment := range w.segments {
		segment.mu.Lock()
		if !segment.Synced {
			segment.File.Sync()
		}
		segment.File.Close()
		segment.mu.Unlock()
	}

	return nil
}

// rotateSegment creates a new segment file
func (w *WALWriter) rotateSegment() error {
	segmentID := w.nextSegmentID
	w.nextSegmentID++

	filename := fmt.Sprintf("%s/segment-%06d.wal", w.basePath, segmentID)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create segment file: %w", err)
	}

	segment := &WALSegment{
		ID:     segmentID,
		File:   file,
		Size:   0,
		Synced: true,
	}

	w.segments[segmentID] = segment
	w.currentSegment = segment

	return nil
}

// syncLoop periodically syncs the current segment
func (w *WALWriter) syncLoop() {
	for {
		select {
		case <-w.syncTicker.C:
			w.Sync()
		case <-w.stopCh:
			return
		}
	}
}

// crc32Checksum calculates CRC32 checksum
func crc32Checksum(data []byte) uint32 {
	// Simple checksum implementation
	// In production, use hardware-accelerated CRC32
	var crc uint32 = 0xFFFFFFFF
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 == 1 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// WALReader reads from WAL segments
type WALReader struct {
	basePath string
	segments map[uint64]*WALSegment
	mu       sync.RWMutex
}

// NewWALReader creates a new WAL reader
func NewWALReader(basePath string) (*WALReader, error) {
	reader := &WALReader{
		basePath: basePath,
		segments: make(map[uint64]*WALSegment),
	}

	// Load existing segments
	if err := reader.loadSegments(); err != nil {
		return nil, fmt.Errorf("failed to load segments: %w", err)
	}

	return reader, nil
}

// ReadEntry reads a single entry from the specified segment and offset
func (r *WALReader) ReadEntry(segmentID uint64, offset int64) (*WALEntry, error) {
	r.mu.RLock()
	segment, exists := r.segments[segmentID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("segment %d not found", segmentID)
	}

	segment.mu.RLock()
	defer segment.mu.RUnlock()

	// Seek to offset
	if _, err := segment.File.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to offset: %w", err)
	}

	// Read header
	header := make([]byte, 16)
	if _, err := io.ReadFull(segment.File, header); err != nil {
		return nil, fmt.Errorf("failed to read entry header: %w", err)
	}

	timestamp := int64(binary.LittleEndian.Uint64(header[0:8]))
	size := binary.LittleEndian.Uint32(header[8:12])
	checksum := binary.LittleEndian.Uint32(header[12:16])

	// Read data
	data := make([]byte, size)
	if _, err := io.ReadFull(segment.File, data); err != nil {
		return nil, fmt.Errorf("failed to read entry data: %w", err)
	}

	// Verify checksum
	if crc32Checksum(data) != checksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return &WALEntry{
		Timestamp: timestamp,
		Size:      size,
		Checksum:  checksum,
		Data:      data,
	}, nil
}

// loadSegments loads existing segment files
func (r *WALReader) loadSegments() error {
	entries, err := os.ReadDir(r.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No segments yet
		}
		return fmt.Errorf("failed to read WAL directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		var segmentID uint64
		if n, err := fmt.Sscanf(entry.Name(), "segment-%06d.wal", &segmentID); n != 1 || err != nil {
			continue // Skip non-segment files
		}

		filename := fmt.Sprintf("%s/%s", r.basePath, entry.Name())
		file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open segment file: %w", err)
		}

		info, err := file.Stat()
		if err != nil {
			file.Close()
			return fmt.Errorf("failed to stat segment file: %w", err)
		}

		segment := &WALSegment{
			ID:     segmentID,
			File:   file,
			Size:   info.Size(),
			Synced: true,
		}

		r.segments[segmentID] = segment
	}

	return nil
}

// Close closes the WAL reader
func (r *WALReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, segment := range r.segments {
		segment.mu.Lock()
		segment.File.Close()
		segment.mu.Unlock()
	}

	return nil
}
