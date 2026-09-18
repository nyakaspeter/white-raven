package memorystorage

import (
	"container/list"
	"io"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
)

const megaByte = 1024 * 1024

type CacheStatus struct {
	Items       int
	Bytes       int64
	Capacity    int64
	Evictions   uint64
	Allocations uint64
	Reuses      uint64
}

type cacheEntry struct {
	key  metainfo.PieceKey
	data []byte
	elem *list.Element
}

// pieceCache is byte bounded rather than item bounded: torrents with different
// piece sizes can safely share it. Evicted buffers are retained and reused, so
// sequential playback stops allocating once the cache has warmed up.
type pieceCache struct {
	mu sync.Mutex

	capacity    int64
	resident    int64
	active      int64
	entries     map[metainfo.PieceKey]*cacheEntry
	lru         *list.List
	recycled    [][]byte
	evictions   uint64
	allocations uint64
	reuses      uint64
	completion  func(metainfo.PieceKey, bool)
}

var cache *pieceCache

func SetMemorySize(memorySize int64, _ int64) {
	cache = &pieceCache{
		// Keep the previous 75% cache budget, leaving room for the Go runtime,
		// torrent bookkeeping, the HTTP server and the widget.
		capacity: memorySize * megaByte * 75 / 100,
		entries:  make(map[metainfo.PieceKey]*cacheEntry),
		lru:      list.New(),
	}
}

func setCompletionCallback(callback func(metainfo.PieceKey, bool)) {
	cache.mu.Lock()
	cache.completion = callback
	cache.mu.Unlock()
}

func Status() CacheStatus {
	if cache == nil {
		return CacheStatus{}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return CacheStatus{
		Items:       len(cache.entries),
		Bytes:       cache.active,
		Capacity:    cache.capacity,
		Evictions:   cache.evictions,
		Allocations: cache.allocations,
		Reuses:      cache.reuses,
	}
}

func (c *pieceCache) read(key metainfo.PieceKey, b []byte, off int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil || off >= int64(len(entry.data)) {
		return 0, io.EOF
	}
	c.lru.MoveToFront(entry.elem)
	n := copy(b, entry.data[off:])
	if n != len(b) {
		return n, io.EOF
	}
	return n, nil
}

func (c *pieceCache) write(key metainfo.PieceKey, pieceLength int64, b []byte, off int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		buffer := c.acquire(int(pieceLength))
		entry = &cacheEntry{key: key, data: buffer[:pieceLength]}
		entry.elem = c.lru.PushFront(entry)
		c.entries[key] = entry
		c.active += int64(cap(buffer))
	} else {
		c.lru.MoveToFront(entry.elem)
	}
	if off < 0 || off+int64(len(b)) > int64(len(entry.data)) {
		return 0, io.ErrShortWrite
	}
	copy(entry.data[off:], b)
	return len(b), nil
}

func (c *pieceCache) acquire(size int) []byte {
	for {
		best := -1
		for i, buffer := range c.recycled {
			if cap(buffer) >= size && (best == -1 || cap(buffer) < cap(c.recycled[best])) {
				best = i
			}
		}
		if best >= 0 {
			buffer := c.recycled[best]
			c.recycled[best] = c.recycled[len(c.recycled)-1]
			c.recycled = c.recycled[:len(c.recycled)-1]
			c.reuses++
			return buffer[:size]
		}

		if c.resident+int64(size) <= c.capacity {
			c.resident += int64(size)
			c.allocations++
			return make([]byte, size)
		}

		if c.lru.Len() != 0 {
			c.evictOldest(true)
			continue
		}

		// Retained buffers can have incompatible sizes when more than one
		// torrent is active. Drop one before allocating a correctly sized one.
		buffer := c.recycled[len(c.recycled)-1]
		c.recycled = c.recycled[:len(c.recycled)-1]
		c.resident -= int64(cap(buffer))
	}
}

func (c *pieceCache) evictOldest(recycle bool) {
	elem := c.lru.Back()
	if elem == nil {
		return
	}
	entry := elem.Value.(*cacheEntry)
	c.lru.Remove(elem)
	delete(c.entries, entry.key)
	c.active -= int64(cap(entry.data))
	c.evictions++
	if c.completion != nil {
		c.completion(entry.key, false)
	}
	if recycle {
		c.recycled = append(c.recycled, entry.data[:cap(entry.data)])
	} else {
		c.resident -= int64(cap(entry.data))
	}
}

func (c *pieceCache) deleteTorrent(infoHash metainfo.Hash) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for elem := c.lru.Back(); elem != nil; {
		previous := elem.Prev()
		entry := elem.Value.(*cacheEntry)
		if entry.key.InfoHash == infoHash {
			c.lru.Remove(elem)
			delete(c.entries, entry.key)
			c.active -= int64(cap(entry.data))
			c.resident -= int64(cap(entry.data))
		}
		elem = previous
	}
}

func storageWriteAt(key metainfo.PieceKey, pieceLength int64, b []byte, off int64) (int, error) {
	return cache.write(key, pieceLength, b, off)
}

func storageReadAt(key metainfo.PieceKey, b []byte, off int64) (int, error) {
	return cache.read(key, b, off)
}

func storageDelete(infoHash metainfo.Hash) {
	cache.deleteTorrent(infoHash)
}
