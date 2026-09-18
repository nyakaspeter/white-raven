package types

import (
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// Torrent lock structure
type TorrentLeaf struct {
	Torrent     *torrent.Torrent
	Progress    int64          // Downoad stats measurement
	Prevtime    time.Time      // Previous time for progress calculation
	FileClients map[string]int // Count active connections
	StatsMutex  sync.Mutex
	PrevUseful  int64
	PrevServed  int64
	PrevWaitNS  int64
	PrevSlow    int64
	PrevEvicted uint64
}

type TorrentFile struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Length string `json:"length"`
}

type TorrentInfo struct {
	Name   string        `json:"name"`
	Hash   string        `json:"hash"`
	Length string        `json:"length"`
	Files  []TorrentFile `json:"files"`
}
