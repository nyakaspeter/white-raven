package v0

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient/memorystorage"
)

type TorrentStatsResponse struct {
	Success          bool   `json:"success"`
	DownSpeed        string `json:"downspeed"`
	UsefulSpeed      string `json:"usefulspeed"`
	ServeSpeed       string `json:"servespeed"`
	DownData         string `json:"downdata"`
	DownPercent      string `json:"downpercent"`
	FullData         string `json:"fulldata"`
	Peers            string `json:"peers"`
	ReadyAhead       string `json:"readyahead"`
	Readahead        string `json:"readahead"`
	ReadWaitMS       int64  `json:"readwaitms"`
	SlowReads        int64  `json:"slowreads"`
	MaxReadMS        int64  `json:"maxreadms"`
	LRUItems         int    `json:"lruitems"`
	LRUBytes         string `json:"lrubytes"`
	LRUCapacity      int    `json:"lrucapacity"`
	LRUCapacityBytes string `json:"lrucapacitybytes"`
	LRUEvictions     uint64 `json:"lruevictions"`
	LRUAllocations   uint64 `json:"lruallocations"`
	LRUReuses        uint64 `json:"lrureuses"`
}

func GetTorrentStats() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		if t, ok := torrentclient.ActiveTorrents[vars["hash"]]; ok {
			io.WriteString(w, downloadStats(t.Torrent))
		} else {
			http.Error(w, torrentNotFound(), http.StatusNotFound)
		}
	}
}

func downloadStats(torr *torrent.Torrent) string {
	leaf := torrentclient.ActiveTorrents[torr.InfoHash().String()]
	leaf.StatsMutex.Lock()
	defer leaf.StatsMutex.Unlock()

	currentProgress := torr.BytesCompleted()
	torrWorkTime := time.Now()
	torrDivTime := torrWorkTime.Sub(leaf.Prevtime).Seconds()
	if torrDivTime <= 0 {
		torrDivTime = 1
	}
	leaf.Prevtime = torrWorkTime

	torrentStats := torr.Stats()
	useful := torrentStats.BytesReadUsefulData.Int64()
	stream := torrentclient.GetStreamSnapshot(torr.InfoHash().String())
	cache := memorystorage.Status()
	downloadSpeed := byteRate(nonNegativeDelta(currentProgress, leaf.Progress), torrDivTime)
	usefulSpeed := byteRate(nonNegativeDelta(useful, leaf.PrevUseful), torrDivTime)
	serveSpeed := byteRate(nonNegativeDelta(stream.ServedBytes, leaf.PrevServed), torrDivTime)
	readWait := nonNegativeDelta(stream.ReadWait.Nanoseconds(), leaf.PrevWaitNS)
	slowReads := nonNegativeDelta(stream.SlowReads, leaf.PrevSlow)
	evictions := nonNegativeDeltaUint(cache.Evictions, leaf.PrevEvicted)
	leaf.Progress = currentProgress
	leaf.PrevUseful = useful
	leaf.PrevServed = stream.ServedBytes
	leaf.PrevWaitNS = stream.ReadWait.Nanoseconds()
	leaf.PrevSlow = stream.SlowReads
	leaf.PrevEvicted = cache.Evictions

	complete := humanize.Bytes(uint64(currentProgress))
	percent := humanize.FormatFloat("#.", float64(currentProgress)/float64(torr.Info().TotalLength())*100)
	size := humanize.Bytes(uint64(torr.Info().TotalLength()))
	peers := strconv.Itoa(torrentStats.ActivePeers) + "/" + strconv.Itoa(torrentStats.TotalPeers)
	readyAhead := completedAhead(torr, stream)
	lruCapacity := 0
	if torr.Info().PieceLength > 0 {
		lruCapacity = int(cache.Capacity / torr.Info().PieceLength)
	}

	//log.Println("Download speed:", downloadSpeed, "Downloaded data:", complete, "Total length:", size)
	//log.Println("Active peers:", torr.Stats().ActivePeers, "Total peers", torr.Stats().TotalPeers, "Percent:", percent)

	message := TorrentStatsResponse{
		Success:          true,
		DownSpeed:        downloadSpeed,
		UsefulSpeed:      usefulSpeed,
		ServeSpeed:       serveSpeed,
		DownData:         complete,
		DownPercent:      percent,
		FullData:         size,
		Peers:            peers,
		ReadyAhead:       humanize.Bytes(uint64(readyAhead)),
		Readahead:        humanize.Bytes(uint64(stream.Readahead)),
		ReadWaitMS:       time.Duration(readWait).Milliseconds(),
		SlowReads:        slowReads,
		MaxReadMS:        stream.MaxRead.Milliseconds(),
		LRUItems:         cache.Items,
		LRUBytes:         humanize.Bytes(uint64(cache.Bytes)),
		LRUCapacity:      lruCapacity,
		LRUCapacityBytes: humanize.Bytes(uint64(cache.Capacity)),
		LRUEvictions:     evictions,
		LRUAllocations:   cache.Allocations,
		LRUReuses:        cache.Reuses,
	}

	messageString, _ := json.Marshal(message)

	return string(messageString)
}

func byteRate(bytes int64, seconds float64) string {
	return humanize.Bytes(uint64(float64(bytes)/seconds)) + "/s"
}

func nonNegativeDelta(current, previous int64) int64 {
	if current < previous {
		return 0
	}
	return current - previous
}

func nonNegativeDeltaUint(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

func completedAhead(torr *torrent.Torrent, stream torrentclient.StreamSnapshot) int64 {
	if stream.FileLength <= 0 || stream.Position < 0 || stream.Position >= stream.FileLength {
		return 0
	}
	pieceLength := torr.Info().PieceLength
	position := stream.FileOffset + stream.Position
	fileEnd := stream.FileOffset + stream.FileLength
	limit := position + stream.Readahead
	if limit <= position {
		limit = position + pieceLength
	}
	if limit > fileEnd {
		limit = fileEnd
	}
	var ready int64
	for position < limit {
		pieceIndex := int(position / pieceLength)
		if !torr.PieceState(pieceIndex).Complete {
			break
		}
		pieceEnd := int64(pieceIndex+1) * pieceLength
		if pieceEnd > limit {
			pieceEnd = limit
		}
		ready += pieceEnd - position
		position = pieceEnd
	}
	return ready
}
