package torrentclient

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	alog "github.com/anacrolix/log"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/nyakaspeter/white-raven/server/internal/settings"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient/memorystorage"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient/types"
	"github.com/nyakaspeter/white-raven/server/pkg/utils"
	"golang.org/x/time/rate"
)

const resolveTimeout = time.Second * 35
const megaByte = 1024 * 1024

var torrentClient *torrent.Client
var receivedTorrent *metainfo.MetaInfo = nil
var maxPieceLength int64 = 16

var ActiveTorrents map[string]*types.TorrentLeaf

const slowStreamRead = 100 * time.Millisecond

type StreamSnapshot struct {
	ServedBytes int64
	ReadWait    time.Duration
	SlowReads   int64
	MaxRead     time.Duration
	Position    int64
	FileOffset  int64
	FileLength  int64
	Readahead   int64
}

type streamTelemetry struct {
	servedBytes atomic.Int64
	readWaitNS  atomic.Int64
	slowReads   atomic.Int64
	maxReadNS   atomic.Int64
	position    atomic.Int64
	fileOffset  atomic.Int64
	fileLength  atomic.Int64
	readahead   atomic.Int64
}

var streamTelemetryByHash sync.Map

func telemetryFor(hash string) *streamTelemetry {
	value, _ := streamTelemetryByHash.LoadOrStore(hash, &streamTelemetry{})
	return value.(*streamTelemetry)
}

func GetStreamSnapshot(hash string) StreamSnapshot {
	telemetry := telemetryFor(hash)
	return StreamSnapshot{
		ServedBytes: telemetry.servedBytes.Load(),
		ReadWait:    time.Duration(telemetry.readWaitNS.Load()),
		SlowReads:   telemetry.slowReads.Load(),
		MaxRead:     time.Duration(telemetry.maxReadNS.Load()),
		Position:    telemetry.position.Load(),
		FileOffset:  telemetry.fileOffset.Load(),
		FileLength:  telemetry.fileLength.Load(),
		Readahead:   telemetry.readahead.Load(),
	}
}

type monitoredTorrentReader struct {
	torrent.Reader
	telemetry *streamTelemetry
	position  int64
}

func (reader *monitoredTorrentReader) Read(buffer []byte) (int, error) {
	started := time.Now()
	n, err := reader.Reader.Read(buffer)
	elapsed := time.Since(started)
	reader.position += int64(n)
	reader.telemetry.position.Store(reader.position)
	reader.telemetry.servedBytes.Add(int64(n))
	reader.telemetry.readWaitNS.Add(elapsed.Nanoseconds())
	if elapsed >= slowStreamRead {
		reader.telemetry.slowReads.Add(1)
	}
	for {
		previous := reader.telemetry.maxReadNS.Load()
		if elapsed.Nanoseconds() <= previous || reader.telemetry.maxReadNS.CompareAndSwap(previous, elapsed.Nanoseconds()) {
			break
		}
	}
	return n, err
}

func (reader *monitoredTorrentReader) Seek(offset int64, whence int) (int64, error) {
	position, err := reader.Reader.Seek(offset, whence)
	if err == nil {
		reader.position = position
		reader.telemetry.position.Store(position)
	}
	return position, err
}

func StartTorrentClient() (*torrent.Client, error) {
	ActiveTorrents = make(map[string]*types.TorrentLeaf)
	streamTelemetryByHash = sync.Map{}

	cfg := torrent.NewDefaultClientConfig()

	if *settings.StorageType == "memory" {
		maxPieceLength = int64(math.Floor(float64(*settings.MemorySize) * 100 / 75 / 8))
		memorystorage.SetMemorySize(*settings.MemorySize)
		cfg.DefaultStorage = memorystorage.NewMemoryStorage()
	} else if *settings.StorageType == "file" {
		cfg.DefaultStorage = storage.NewFileByInfoHash(*settings.DownloadDir)
		cfg.DataDir = *settings.DownloadDir
	}

	cfg.EstablishedConnsPerTorrent = *settings.MaxConnections
	cfg.NoDHT = *settings.NoDHT
	cfg.DisableUTP = *settings.DisableUTP
	cfg.DisableIPv6 = *settings.DisableIPv6

	// Discard or show the logs
	if !*settings.EnableLog {
		discardLogger := alog.NewLogger()
		discardLogger.SetHandlers(alog.DiscardHandler)
		cfg.Logger = discardLogger
	}
	//cfg.Debug = true

	// up/download speed rate in bytes per second from megabits per second
	downrate := int((*settings.DownloadRate * 1024) / 8)
	uprate := int((*settings.UploadRate * 1024) / 8)

	if downrate != 0 {
		cfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(downrate), downrate)
	}

	if uprate == 0 {
		cfg.NoUpload = true
	} else {
		cfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(uprate), uprate)
	}

	var err error = nil
	torrentClient, err = torrent.NewClient(cfg)
	if err == nil {
		log.Printf(
			"Torrent client started: storage=%s memory=%dMB max_connections=%d dht=%t ipv6=%t utp=%t download_limit_kbps=%d upload_limit_kbps=%d",
			*settings.StorageType, *settings.MemorySize, *settings.MaxConnections,
			!*settings.NoDHT, !*settings.DisableIPv6, !*settings.DisableUTP,
			*settings.DownloadRate, *settings.UploadRate,
		)
	}

	return torrentClient, err
}

func StopTorrentClient() {
	if torrentClient == nil {
		return
	}

	torrentClient.Close()

	torrentClient = nil
	ActiveTorrents = nil
	receivedTorrent = nil

}

func AddTorrent(uri string) types.TorrentInfo {
	info := types.TorrentInfo{}
	torrent := addTorrentFromUri(uri)

	if torrent == nil {
		return info
	}

	info.Hash = torrent.InfoHash().String()
	info.Name = torrent.Name()
	info.Length = strconv.FormatInt(torrent.Length(), 10)
	sortFiles(torrent.Files())

	for _, f := range torrent.Files() {
		tf := types.TorrentFile{
			Name:   f.DisplayPath(),
			Url:    "http://" + utils.GetLocalIP() + ":" + strconv.Itoa(*settings.Port) + "/file/" + f.Torrent().InfoHash().String() + "/" + base64.StdEncoding.EncodeToString([]byte(f.DisplayPath())),
			Length: strconv.FormatInt(f.FileInfo().Length, 10),
		}

		info.Files = append(info.Files, tf)
	}

	return info
}

func ServeTorrentFile(w http.ResponseWriter, r *http.Request, file *torrent.File) {
	w.Header().Set("TransferMode.DLNA.ORG", "Streaming")
	w.Header().Set("contentFeatures.dlna.org", "DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000")

	torrentReader := file.NewReader()
	defer torrentReader.Close()
	torrentReader.SetContext(r.Context())
	telemetry := telemetryFor(file.Torrent().InfoHash().String())
	telemetry.fileOffset.Store(file.Offset())
	telemetry.fileLength.Store(file.Length())
	readahead := streamReadahead(file.Torrent().Info().PieceLength, memorystorage.Status().Capacity)
	torrentReader.SetReadahead(readahead)
	telemetry.readahead.Store(readahead)
	torrentReader.SetResponsive()
	reader := &monitoredTorrentReader{Reader: torrentReader, telemetry: telemetry}

	path := file.FileInfo().Path
	fname := ""
	if len(path) == 0 {
		fname = file.DisplayPath()
	} else {
		fname = path[len(path)-1]
	}
	w.Header().Set("Content-Type", mediaContentType(fname))
	http.ServeContent(w, r, fname, time.Unix(0, 0), reader)
}

func mediaContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mkv", ".mk3d":
		return "video/x-matroska"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi", ".divx":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".asf":
		return "video/x-ms-asf"
	case ".ts", ".m2ts", ".mts":
		return "video/mp2t"
	case ".mpg", ".mpeg", ".mpe", ".vob":
		return "video/mpeg"
	case ".ogv":
		return "video/ogg"
	case ".3gp":
		return "video/3gpp"
	case ".3g2":
		return "video/3gpp2"
	case ".flv":
		return "video/x-flv"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".oga", ".opus":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".wma":
		return "audio/x-ms-wma"
	default:
		return "application/octet-stream"
	}
}

func streamReadahead(pieceLength, cacheCapacity int64) int64 {
	readahead := cacheCapacity / 4
	if readahead < pieceLength {
		return pieceLength
	}
	return readahead
}

func GetActiveTorrents() []types.TorrentInfo {
	activeTorrents := []types.TorrentInfo{}

	for _, t := range ActiveTorrents {
		at := types.TorrentInfo{}
		at.Hash = t.Torrent.InfoHash().String()
		at.Name = t.Torrent.Name()
		at.Length = strconv.FormatInt(t.Torrent.Length(), 10)

		for _, f := range t.Torrent.Files() {
			tf := types.TorrentFile{
				Name:   f.DisplayPath(),
				Url:    "http://" + utils.GetLocalIP() + ":" + strconv.Itoa(*settings.Port) + "/file/" + f.Torrent().InfoHash().String() + "/" + base64.StdEncoding.EncodeToString([]byte(f.DisplayPath())),
				Length: strconv.FormatInt(f.FileInfo().Length, 10),
			}

			at.Files = append(at.Files, tf)
		}

		activeTorrents = append(activeTorrents, at)
	}

	return activeTorrents
}

func RemoveTorrent(hash string) error {
	if t, ok := ActiveTorrents[hash]; ok {
		for _, f := range t.Torrent.Files() {
			StopFileDownload(f)
		}
		t.Torrent.Drop()
		delete(ActiveTorrents, hash)
		streamTelemetryByHash.Delete(hash)
		return nil
	}

	return errors.New("torrent not found")
}

func StopFileDownload(file *torrent.File) {
	if file != nil {
		file.SetPriority(torrent.PiecePriorityNone)
	}
}

func IncreaseConnections(path string, t *types.TorrentLeaf) int {
	if v, ok := t.FileClients[path]; ok {
		v++
		t.FileClients[path] = v
		return v
	} else {
		t.FileClients[path] = 1
		return 1
	}
}

func DecreaseConnections(path string, t *types.TorrentLeaf) int {
	if v, ok := t.FileClients[path]; ok {
		v--
		t.FileClients[path] = v
		return v
	} else {
		t.FileClients[path] = 0
		return 0
	}
}

func GetFileIndexByPath(search string, files []*torrent.File) int {

	for i, f := range files {
		if search == f.DisplayPath() {
			return i
		}
	}

	return -1
}

func CalculateOpensubtitlesHash(file *torrent.File) string {
	chunkSize := 65536
	fileReader := file.NewReader()

	if file.Length() < int64(chunkSize) {
		return "0"
	}

	// The First and Last 65536 bytes are used to calculate the hash
	buffer := make([]byte, chunkSize*2)

	fileReader.Seek(0, 0)
	_, err := fileReader.Read(buffer[:chunkSize])
	if err != nil {
		return "0"
	}

	fileReader.Seek(-(int64(chunkSize)), 2)
	_, err = fileReader.Read(buffer[chunkSize:])
	if err != nil && err != io.EOF {
		return "0"
	}

	// Convert to uint64, and sum.
	var hash uint64
	nums := make([]uint64, ((chunkSize * 2) / 8))
	bufferReader := bytes.NewReader(buffer)
	err = binary.Read(bufferReader, binary.LittleEndian, &nums)
	if err != nil {
		return "0"
	}
	for _, num := range nums {
		hash += num
	}

	return fmt.Sprintf("%016x", hash+uint64(file.Length()))
}
