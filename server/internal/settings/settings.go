package settings

import (
	"encoding/json"
	"flag"
)

type TorznabFeed struct {
	URL    string `json:"url"`
	APIKey string `json:"apiKey"`
}

type torznabFeedList []TorznabFeed

func (feeds *torznabFeedList) String() string {
	data, _ := json.Marshal(feeds)
	return string(data)
}

func (feeds *torznabFeedList) Set(value string) error {
	return json.Unmarshal([]byte(value), feeds)
}

type Config struct {
	Host                  string        `json:"host"`
	Port                  int           `json:"port"`
	DlnaPort              int           `json:"dlnaPort"`
	DownloadDir           string        `json:"downloadDir"`
	DownloadRate          int           `json:"downloadRate"`
	UploadRate            int           `json:"uploadRate"`
	MaxConnections        int           `json:"maxConnections"`
	NoDHT                 bool          `json:"noDHT"`
	DisableIPv6           bool          `json:"disableIPv6"`
	DisableUTP            bool          `json:"disableUTP"`
	EnableLog             bool          `json:"-"`
	EnableReceiver        bool          `json:"-"`
	StorageType           string        `json:"storageType"`
	MemorySize            int64         `json:"memorySize"`
	Background            bool          `json:"-"`
	CORS                  bool          `json:"-"`
	TMDBKey               string        `json:"tmdbKey"`
	OpenSubtitlesUser     string        `json:"openSubtitlesUser"`
	OpenSubtitlesPassword string        `json:"openSubtitlesPassword"`
	OpenSubtitlesKey      string        `json:"openSubtitlesKey"`
	TorznabFeeds          []TorznabFeed `json:"torznabFeeds"`
	NcoreUser             string        `json:"ncoreUser"`
	NcorePassword         string        `json:"ncorePassword"`
	InsaneUser            string        `json:"insaneUser"`
	InsanePassword        string        `json:"insanePassword"`
}

func DefaultConfig() Config {
	return Config{
		Port: 9000, DlnaPort: 3500, DownloadDir: "data", MaxConnections: 50,
		DisableIPv6: true, DisableUTP: false,
		EnableLog: true, EnableReceiver: true, StorageType: "memory", MemorySize: 128,
		CORS: true, TMDBKey: "a4d9ad8d2d072c50dc998cc0d1a508fa",
	}
}

func Apply(config Config) {
	Host = &config.Host
	Port = &config.Port
	DlnaPort = &config.DlnaPort
	DownloadDir = &config.DownloadDir
	DownloadRate = &config.DownloadRate
	UploadRate = &config.UploadRate
	MaxConnections = &config.MaxConnections
	NoDHT = &config.NoDHT
	DisableIPv6 = &config.DisableIPv6
	DisableUTP = &config.DisableUTP
	EnableLog = &config.EnableLog
	EnableReceiver = &config.EnableReceiver
	StorageType = &config.StorageType
	MemorySize = &config.MemorySize
	Background = &config.Background
	CORS = &config.CORS
	TMDBKey = &config.TMDBKey
	OpenSubtitlesUser = &config.OpenSubtitlesUser
	OpenSubtitlesPassword = &config.OpenSubtitlesPassword
	OpenSubtitlesKey = &config.OpenSubtitlesKey
	feeds := torznabFeedList(config.TorznabFeeds)
	TorznabFeeds = &feeds
	NcoreUser = &config.NcoreUser
	NcorePassword = &config.NcorePassword
	InsaneUser = &config.InsaneUser
	InsanePassword = &config.InsanePassword
}

func Current() Config {
	return Config{
		Host: *Host, Port: *Port, DlnaPort: *DlnaPort, DownloadDir: *DownloadDir,
		DownloadRate: *DownloadRate, UploadRate: *UploadRate, MaxConnections: *MaxConnections,
		NoDHT: *NoDHT, DisableIPv6: *DisableIPv6, DisableUTP: *DisableUTP,
		EnableLog: *EnableLog, EnableReceiver: *EnableReceiver,
		StorageType: *StorageType, MemorySize: *MemorySize, Background: *Background, CORS: *CORS,
		TMDBKey: *TMDBKey, OpenSubtitlesUser: *OpenSubtitlesUser,
		OpenSubtitlesPassword: *OpenSubtitlesPassword, OpenSubtitlesKey: *OpenSubtitlesKey,
		TorznabFeeds: append([]TorznabFeed(nil), (*TorznabFeeds)...),
		NcoreUser:    *NcoreUser, NcorePassword: *NcorePassword,
		InsaneUser: *InsaneUser, InsanePassword: *InsanePassword,
	}
}

var Host *string
var Port *int
var DlnaPort *int
var DownloadDir *string
var DownloadRate *int
var UploadRate *int
var MaxConnections *int
var NoDHT *bool
var DisableIPv6 *bool
var DisableUTP *bool
var EnableLog *bool
var EnableReceiver *bool
var StorageType *string
var MemorySize *int64
var Background *bool
var CORS *bool
var TMDBKey *string
var OpenSubtitlesUser *string
var OpenSubtitlesPassword *string
var OpenSubtitlesKey *string
var TorznabFeeds *torznabFeedList
var NcoreUser *string
var NcorePassword *string
var InsaneUser *string
var InsanePassword *string

func Init() {
	Host = flag.String("host", "", "listening server ip")
	Port = flag.Int("port", 9000, "listening port")
	DlnaPort = flag.Int("dlnaport", 3500, "DLNA server port")
	DownloadDir = flag.String("dir", "data", "specify the directory where files will be downloaded to if storagetype is set to \"file\"")
	DownloadRate = flag.Int("downrate", 0, "download speed rate in Kbps")
	UploadRate = flag.Int("uprate", 0, "upload speed rate in Kbps")
	MaxConnections = flag.Int("maxconn", 50, "max connections per torrent")
	NoDHT = flag.Bool("nodht", false, "disable dht")
	DisableIPv6 = flag.Bool("noipv6", false, "disable IPv6 torrent sockets")
	DisableUTP = flag.Bool("noutp", false, "disable uTP torrent sockets")
	EnableLog = flag.Bool("log", false, "enable log messages")
	EnableReceiver = flag.Bool("receiver", true, "enable torrent receiver page")
	StorageType = flag.String("storagetype", "memory", "select storage type (must be set to \"memory\" or \"file\")")
	Background = flag.Bool("background", false, "run the server in the background")
	CORS = flag.Bool("cors", true, "enable CORS")
	MemorySize = flag.Int64("memorysize", 128, "specify the storage memory size in MB if storagetype is set to \"memory\" (minimum 64)") // 64MB is optimal for TVs
	TMDBKey = flag.String("tmdbkey", "a4d9ad8d2d072c50dc998cc0d1a508fa", "set external TMDB API key")
	feeds := torznabFeedList{}
	TorznabFeeds = &feeds
	flag.Var(TorznabFeeds, "torznabfeeds", "set Torznab feeds as a JSON array of url and apiKey objects")
	OpenSubtitlesKey = flag.String("osapikey", "", "set OpenSubtitles.com API key")
	OpenSubtitlesUser = flag.String("osuser", "", "set optional OpenSubtitles.com username for authenticated subtitle downloads")
	OpenSubtitlesPassword = flag.String("ospassword", "", "set optional OpenSubtitles.com password for authenticated subtitle downloads")
	NcoreUser = flag.String("ncoreuser", "", "set nCore username")
	NcorePassword = flag.String("ncorepassword", "", "set nCore password")
	InsaneUser = flag.String("insaneuser", "", "set iNSANE username")
	InsanePassword = flag.String("insanepassword", "", "set iNSANE password")
	flag.Parse()
}
