# go-video-daemon

`go-video-daemon` is an simple application to play video on a server. It is based on

- [connectrpc](https://connectrpc.com/) to build the protocol to control the application from a client
- [mpv](https://mpv.io/) to play video file, it is controlled throught ipc socket
- [ffmpeg](https://www.ffmpeg.org/) to read video file metadata
- [sqlite](https://sqlite.org/) for the video database

## Build the application

To build the application, just use the following command:

```
make build
```

The binary is then available in the folder `bin`.

## Use the application

To use the application, first, you have to install the dependencies:

- mpv
- ffmpeg

When it's done, you have to launch `mpv` in idle mode and with ipc server activated, for example with the following command:

```shell
mpv \
    --input-ipc-server=/tmp/mpv-ipc-socket \
    --idle --quiet \
    --no-resume-playback
```

Finally, you can run `go-video-daemon`

```
./bin/go-video-daemon
```

## Configuration

By default, `go-video-daemon` searches for a configuration file located at `/etc/go-video-daemon.yaml`. You can create this file if you want to change the configuration of the application. The default configuration is

```yaml
listen: "localhost:10123"
player:
  socket: /tmp/mpv-ipc-socket
library:
  database: "db/go-video-daemon.db"
  movies: "/Users/mroyer/Movies/Films"
  tvshows: "/Users/mroyer/Movies/Series"
```

## Client

A web client exitsts to control this application: [player-web-client](https://github.com/mroy31/player-web-client)

## Docker

You can `go-video-daemon` in a docker container, to build the image simply use the following command

```
make docker
```

You can find below an example of `docker-compose.yml` file to run the server with the web client.

```yaml
volumes:
  db:
  cache:

networks:
  video-player-net:
    name: video-player-net

services:
  video-player-server:
    image: video-player-server:latest
    container_name: video-player-server
    restart: unless-stopped
    volumes:
      - db:/db
      - /my/videos/folder:/my/video/folder:ro
      - /tmp/mpv-ipc-socket:/tmp/mpv-ipc-socket
      - ./config-server.yaml:/etc/go-video-daemon/config.yaml:ro
    networks:
      - video-player-net

  video-player-frontend:
    image: video-player-frontend:latest
    container_name: video-player-frontend
    restart: unless-stopped
    ports: 
      - 5171:80
    volumes:
      - cache:/var/cache/nginx
    environment:
      - SERVER_ADDRESS=video-player-server:10123
    networks:
      - video-player-net
```