package httpserver

import (
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/nyakaspeter/white-raven/server/internal/torrentclient"
	"github.com/nyakaspeter/white-raven/server/pkg/subtitles"
	subtitlestypes "github.com/nyakaspeter/white-raven/server/pkg/subtitles/types"
)

func ServeTorrentFile() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		if d, err := base64.StdEncoding.DecodeString(vars["base64path"]); err == nil {
			if t, ok := torrentclient.ActiveTorrents[vars["hash"]]; ok {

				idx := torrentclient.GetFileIndexByPath(string(d), t.Torrent.Files())
				if idx != -1 {
					file := t.Torrent.Files()[idx]

					path := file.DisplayPath()
					started := time.Now()
					completedBefore := t.Torrent.BytesCompleted()
					peersBefore := t.Torrent.Stats()
					streamWriter := &streamResponseWriter{ResponseWriter: w}
					log.Printf(
						"Torrent stream started: hash=%s file=%q range=%q remote=%s peers=%d/%d",
						vars["hash"], path, r.Header.Get("Range"), r.RemoteAddr,
						peersBefore.ActivePeers, peersBefore.TotalPeers,
					)

					torrentclient.IncreaseConnections(path, t)
					torrentclient.ServeTorrentFile(streamWriter, r, file)

					//stop downloading the file when no connections left
					if torrentclient.DecreaseConnections(path, t) <= 0 {
						torrentclient.StopFileDownload(file)
					}
					peersAfter := t.Torrent.Stats()
					log.Printf(
						"Torrent stream finished: hash=%s file=%q status=%d bytes=%d duration=%s downloaded=%d write_error=%v context_error=%v peers=%d/%d",
						vars["hash"], path, streamWriter.Status(), streamWriter.bytes,
						time.Since(started).Round(time.Millisecond),
						t.Torrent.BytesCompleted()-completedBefore, streamWriter.err, r.Context().Err(),
						peersAfter.ActivePeers, peersAfter.TotalPeers,
					)
				} else {
					http.Error(w, "Invalid path", http.StatusNotFound)
					return
				}
			} else {
				log.Println("Unknown torrent:", vars["hash"])
				http.Error(w, "Unknown torrent", http.StatusNotFound)
				return
			}
		} else {
			log.Println(err)
			http.Error(w, "Invalid path", http.StatusNotFound)

			return
		}
	}
}

type streamResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	err    error
}

func (writer *streamResponseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *streamResponseWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	written, err := writer.ResponseWriter.Write(data)
	writer.bytes += int64(written)
	if err != nil && writer.err == nil {
		writer.err = err
	}
	return written, err
}

func (writer *streamResponseWriter) Status() int {
	if writer.status == 0 {
		return http.StatusOK
	}
	return writer.status
}

// Unwrap lets net/http preserve optional response-writer behavior through the
// diagnostic wrapper.
func (writer *streamResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func ServeSubtitleFile() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		params := subtitlestypes.SubtitleParams{}
		params.FileId = vars["fileId"]
		params.TargetType = vars["type"]

		contents := subtitles.GetSubtitleContents(params)

		if contents.Text == "" {
			http.Error(w, "Failed to load subtitle", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Disposition", contents.ContentDisposition)
		w.Header().Set("Content-Type", contents.ContentType)
		io.WriteString(w, contents.Text)
	}
}
