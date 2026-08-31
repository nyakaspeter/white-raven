# White Raven

White Raven is a torrent player application for Samsung Smart TV E, F, H series. This repository contains the Smart TV Widget part of the application. For it to work, the [White Raven Server](https://github.com/nyakaspeter/White-Raven-Server) application must also be running on the TV (which is only possible if it's rooted), or on the local network. I recommend setting up a [Jackett](https://github.com/Jackett/Jackett) server too, to get the most out of the application. This repo is a fork of [White Raven](https://github.com/silentmurdock/whiteraven).

## Features

- Torrent server can run locally on rooted TV
- Torrent streaming from memory
- Automatic subtitle search by IMDB ID, Text or by File Hash
- Torrent receiver page allow to stream any magnet link or torrent file
- Favourites handler
- Subtitle style settings
- You can search for movies, episodes or whole season/series packs
- You can play torrents with multiple video files inside

## How to use

### Rootless version

If you don't have root on your television or just don't want to run the server on the TV, you can also run it from another device on your local network. For this you have to [download](https://github.com/nyakaspeter/White-Raven/releases) the rootless version of White Raven and run the [White Raven Server](https://github.com/nyakaspeter/White-Raven-Server/releases) separately. I also recommend setting up a [Jackett](https://github.com/Jackett/Jackett) server on your local network, to be able to use a significantly higher number of torrent trackers. Instructions for running White Raven Server and Jackett on various devices can be found [here](https://github.com/nyakaspeter/White-Raven-Server#how-to-use).

#### Running the widget from USB on Samsung Smart TV E, F, H series

0. Ensure that White Raven Server is running on the local network.
1. Grab a FAT32 formatted USB stick and create a folder named as `WhiteRaven` in it's root.
2. Extract the contents of the downloaded White Raven rootless zip file to this directory.
3. Plug the pendrive into your television.
4. White Raven should show up in the apps section. After launching it, it should automatically connect to the server. The pendrive has to be plugged in while using the app.

#### Installing the widget on rooted Samsung Smart TV E, F, H series</summary>

0. Ensure that White Raven Server is running on the local network.
1. Connect to your television over FTP/SFTP.
2. Create a folder named as `WhiteRaven` inside the `/mtd_rwcommon/widgets/user` directory.
3. Extract the contents of the downloaded zip file to this directory.
4. Reboot your television.
5. After reboot White Raven should show up in the apps section. After launching it, it should automatically connect to the server.

### Root version

If you have a rooted E, F, or H series Samsung Smart TV, you can run both the client and the server simultaneously on your television. For this you have to [download](https://github.com/nyakaspeter/White-Raven/releases) the root version of White Raven. I also recommend setting up a [Jackett](https://github.com/Jackett/Jackett) server on your local network, to be able to use a significantly higher number of torrent trackers. Unfortunately Jackett cannot be run from the TV itself, but it can run on a number of devices, I've written about it [here](https://github.com/nyakaspeter/Raven-Torrent#how-to-use).

#### Installing the widget on rooted Samsung Smart TV E, F, H series

1. Connect to your television over FTP/SFTP.
2. Create a folder named as `WhiteRaven` inside the `/mtd_rwcommon/widgets/user` directory.
3. Extract the contents of the downloaded zip file to this directory.
4. If you want to use the Jackett provider or tweak server settings, modify `server.init` inside the `server` directory. See the [documentation](https://github.com/nyakaspeter/Raven-Torrent#cli-arguments) for launch arguments.
5. Reboot your television.
6. After reboot White Raven should show up in the apps section. After launching it, it should automatically start the server and connect to it.

## Build instructions

You can build the White Raven widget zip file by running the following commands from the project directory. [Go](https://golang.org/) must be installed for these to work.

#### Building the rootless version

`go run build/build.go rootless`

#### Building the rooted version

First you have to [download](https://github.com/nyakaspeter/Raven-Torrent/releases) or [build](https://github.com/nyakaspeter/Raven-Torrent#build-instructions) the ARM version of the server binary and place it in the `build` directory, then run:

`go run build/build.go rooted -serverfile="build/raven"`

## Browser compatibility mode

Start White Raven server first, then run this command from the White Raven repository root:

`go run ./browser/devserver -server http://127.0.0.1:9000`

Open `http://127.0.0.1:8080/`. The White Raven server address, including a custom port, is configured with the `-server` argument.

## Terms of service
[TERMS OF SERVICE](TOS)

## License
[GNU GENERAL PUBLIC LICENSE Version 3](LICENSE)
