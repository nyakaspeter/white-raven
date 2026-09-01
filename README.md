# White Raven

White Raven is a torrent media player for Samsung Smart TV E, F, and H series. This repository contains the complete application:

- `widget/`: the legacy Samsung Smart TV widget
- `server/`: White Raven Server (`wrserver`)
- `browser/`: the browser development harness
- `build/`: the widget packager

The project is a fork of the original [White Raven](https://github.com/silentmurdock/whiteraven) widget and [wrserver](https://github.com/silentmurdock/wrserver).

## Features

- Torrent streaming from memory or disk
- Torrentio, Jackett, nCore, and iNSANE torrent search
- Automatic subtitle search by IMDb ID, title, or file hash
- Movie and TV metadata discovery
- Torrent receiver page
- DLNA casting and local media player integration
- Rooted and rootless Samsung TV packages
- Browser harness for development without a TV

## Installing on a Samsung TV

### For non-rooted Samsung E, F, H series

1. Run White Raven Server on another device in the local network.
2. Create a `WhiteRaven` folder on a FAT32 USB drive.
3. Extract the rootless widget zip into that folder.
4. Connect the drive to the TV and launch White Raven.

### For rooted Samsung E, F, H series

1. Connect to your television over FTP/SFTP.
2. Create a folder named as `WhiteRaven` inside the `/mtd_rwcommon/widgets/user` directory.
3. Extract the contents of the downloaded zip file to this directory.
4. Configure any provider credentials or server flags in `server.init` inside the `server` directory.
5. Reboot your television.
6. After reboot White Raven should show up in the apps section. After launching it, it should automatically start the server and connect to it.

## Development

Go 1.27 or newer is required.

Start White Raven Server on port 9000:

```sh
go run ./server -port 9000 -log
```

In another terminal, start the browser harness:

```sh
go run ./browser -server http://127.0.0.1:9000
```

## Building the widget

Build the rootless widget:

```sh
go run ./build rootless -version 0.8.0
```

This creates `build/whiteraven-rootless-0.8.0.zip`.

Build the rooted widget:

```sh
go run ./build rooted -version 0.8.0
```

This cross-compiles `./server` for Linux/ARMv7, embeds it as `server/wrserver`, and creates `build/whiteraven-0.8.0.zip`.

## Building White Raven Server

For local development:

```sh
go build -o build/wrserver ./server
```

For a versioned standalone build, inject the same release value through Go's linker:

```sh
go build -trimpath -ldflags="-s -w -X github.com/nyakaspeter/white-raven/build/version.Value=0.8.0" -o build/wrserver ./server
```

Cross-compilation examples:

```sh
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/nyakaspeter/white-raven/build/version.Value=0.8.0" -o build/wrserver-linux-armv7 ./server
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/nyakaspeter/white-raven/build/version.Value=0.8.0" -o build/wrserver-linux-x64 ./server
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/nyakaspeter/white-raven/build/version.Value=0.8.0" -o build/wrserver-windows-x64.exe ./server
```

## Server command-line arguments

- `-host`: interface/IP to listen on
- `-port`: HTTP port; default `9000`
- `-dlnaport`: DLNA server port; default `3500`
- `-storagetype`: `memory` or `file`; default `memory`
- `-memorysize`: memory storage size in MB; minimum `64`, default `128`
- `-dir`: download directory for file storage; default `data`
- `-downrate`: download limit in Kbps; `0` is unlimited
- `-uprate`: upload limit in Kbps; `0` disables uploading
- `-maxconn`: maximum connections per torrent; default `50`
- `-nodht`: disable DHT
- `-jackettaddress` and `-jackettkey`: Jackett connection
- `-ncoreuser` and `-ncorepassword`: nCore credentials
- `-insaneuser` and `-insanepassword`: iNSANE credentials
- `-osapikey`: OpenSubtitles.com API key
- `-osuser` and `-ospassword`: optional OpenSubtitles.com credentials
- `-tmdbkey`: TMDB API key
- `-receiver`: enable the receiver page; default `true`
- `-cors`: enable API CORS; default `true`
- `-background`: run in the background
- `-log`: enable logs

Example using file storage:

```sh
./build/wrserver -storagetype file -dir downloads
```

Example using Jackett:

```sh
./build/wrserver -jackettaddress http://192.168.0.2:9117 -jackettkey YOUR_API_KEY
```

## Terms of service

[Terms of service](TOS)

## License

[GNU General Public License version 3](LICENSE)
