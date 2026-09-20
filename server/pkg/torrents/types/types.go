package types

type MovieTorrent struct {
	DedupKey string `json:"-"`
	Hash     string `json:"hash"`
	Quality  string `json:"quality"`
	Size     string `json:"size"`
	Provider string `json:"provider"`
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Seeds    string `json:"seeds"`
	Peers    string `json:"peers"`
	Magnet   string `json:"magnet"`
	Torrent  string `json:"torrent"`
}

type ShowTorrent struct {
	DedupKey string `json:"-"`
	Hash     string `json:"hash"`
	Quality  string `json:"quality"`
	Season   string `json:"season"`
	Episode  string `json:"episode"`
	Size     string `json:"size"`
	Provider string `json:"provider"`
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Seeds    string `json:"seeds"`
	Peers    string `json:"peers"`
	Magnet   string `json:"magnet"`
	Torrent  string `json:"torrent"`
}

type MovieParams struct {
	ImdbId     string `json:"imdbId"`
	SearchText string `json:"searchText"`
}

type ShowParams struct {
	ImdbId     string `json:"imdbId"`
	SearchText string `json:"searchText"`
	Season     string `json:"season"`
	Episode    string `json:"episode"`
}

type SourceParams struct {
	Torznab   TorznabParams   `json:"torznab"`
	Ncore     NcoreParams     `json:"ncore"`
	Insane    InsaneParams    `json:"insane"`
	Torrentio TorrentioParams `json:"torrentio"`
}

type TorznabParams struct {
	Enabled bool `json:"enabled"`
}

type NcoreParams struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type InsaneParams struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type TorrentioParams struct {
	Enabled bool `json:"enabled"`
}
