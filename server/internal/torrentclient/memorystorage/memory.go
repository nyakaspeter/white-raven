package memorystorage

import (
	"context"
	"fmt"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// Just to set pieces UnComplete
type memoryClient struct {
	pc storage.PieceCompletion
}

type memoryTorrent struct {
	cl *memoryClient
	pl int64
	ih metainfo.Hash
	np int // Just to set pieces UnComplete
}

type memoryPiece struct {
	trt *memoryTorrent

	p metainfo.Piece
}

func NewMemoryStorage() storage.ClientImpl {
	ret := &memoryClient{
		pc: storage.NewMapPieceCompletion(),
	}
	setCompletionCallback(func(key metainfo.PieceKey, complete bool) {
		_ = ret.pc.Set(key, complete)
	})

	return ret
}

func (me *memoryClient) Close() error {
	//return me.pc.Close()
	return nil
}

func (me *memoryClient) OpenTorrent(_ context.Context, info *metainfo.Info, infoHash metainfo.Hash) (storage.TorrentImpl, error) {
	torrent := &memoryTorrent{
		cl: me,
		pl: info.PieceLength,
		ih: infoHash,
		np: info.NumPieces(),
	}
	// RAM storage always starts empty. Record that explicitly so a cold seek
	// can request its pieces immediately instead of waiting for asynchronous
	// completion checks against storage that cannot contain prior data.
	for index := 0; index < torrent.np; index++ {
		key := metainfo.PieceKey{InfoHash: infoHash, Index: index}
		if err := me.pc.Set(key, false); err != nil {
			return storage.TorrentImpl{}, fmt.Errorf("initialize piece %d completion: %w", index, err)
		}
	}

	return storage.TorrentImpl{
		Piece: torrent.Piece,
		Close: torrent.Close,
	}, nil
}

func (me *memoryTorrent) Piece(p metainfo.Piece) storage.PieceImpl {
	return &memoryPiece{
		trt: me,
		p:   p,
	}
}

func (me *memoryTorrent) Close() error {
	// Set all pieces UnComplete
	for key := 0; key < me.np; key++ {
		me.cl.pc.Set(metainfo.PieceKey{InfoHash: me.ih, Index: key}, false)
	}

	storageDelete(me.ih)

	return nil
}

func (sp *memoryPiece) Completion() storage.Completion {
	ret, _ := sp.trt.cl.pc.Get(sp.pieceKey())
	return ret
}

func (sp *memoryPiece) MarkComplete() error {
	sp.trt.cl.pc.Set(sp.pieceKey(), true)
	return nil
}

func (sp *memoryPiece) MarkNotComplete() error {
	sp.trt.cl.pc.Set(sp.pieceKey(), false)
	return nil
}

func (sp *memoryPiece) ReadAt(b []byte, off int64) (n int, err error) {
	ci := sp.p.Index()
	bToRead := sp.trt.pl - off
	//log.Printf("Got read for chunk (%d) offset (%d).", ci, off)
	for len(b) != 0 {
		//ck := sp.chunkKey(int(ci))
		var rLen int
		if len(b) < int(bToRead) {
			rLen = len(b)
		} else {
			rLen = int(bToRead)
		}
		i, rerr := storageReadAt(metainfo.PieceKey{InfoHash: sp.trt.ih, Index: ci}, b[:rLen], off)
		//log.Printf("Doing read for chunk (%d) offset (%d) for (%d) bytes and got (%d) bytes.", ci, off, rLen, i)
		n1 := i
		off = 0
		ci++
		b = b[n1:]
		n += n1
		if rerr != nil {
			//log.Printf("Error Reading During Read: %s", rerr)
			err = rerr
			return
		}
	}
	return
}

func (sp *memoryPiece) WriteAt(b []byte, off int64) (n int, err error) {
	ci := sp.p.Index()
	//log.Printf("At chunk (%d), got bytes (%d) and offset (%d).", ci, len(b), off)
	bToWrite := sp.trt.pl - off
	var btw int
	for len(b) != 0 {
		if len(b) > int(bToWrite) {
			btw = int(bToWrite)
		} else {
			btw = len(b)
		}
		//ck := sp.chunkKey(int(ci))
		pieceLength := sp.trt.pl
		if int(ci) == sp.trt.np-1 {
			pieceLength = sp.p.Info.TotalLength() - int64(ci)*sp.trt.pl
		}
		n1, werr := storageWriteAt(metainfo.PieceKey{InfoHash: sp.trt.ih, Index: ci}, pieceLength, b[:btw], off)
		//log.Printf("Writing (%d) bytes [confirm %d] to chunk (%d) - total bytes (%d) - offset (%d) - written (%d) bytes.", btw, len(b[:btw]), ci, len(b), off, n1)
		if werr != nil {
			//log.Printf("Error Writing During Write: %s", werr)
			err = werr
			return
		}
		if n1 > len(b) {
			break
		}
		b = b[n1:]
		off = 0
		bToWrite = sp.trt.pl - off
		ci++
		n += n1
	}
	return
}

func (sp *memoryPiece) pieceKey() metainfo.PieceKey {
	return metainfo.PieceKey{InfoHash: sp.trt.ih, Index: sp.p.Index()}
}
