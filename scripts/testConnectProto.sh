#!/bin/bash

# GetVersion command
curl \
    --header "Content-Type: application/json" \
    --data '{}' \
    http://localhost:10123/player.v1.VideoPlayerService/GetVersion

printf "\n"

# LibraryGetFolderContent command
curl \
    --header "Content-Type: application/json" \
    --data '{"name": "tvshows", "folder": ""}' \
    http://localhost:10123/player.v1.VideoPlayerService/LibraryGetFolderContent

printf "\n"

# PlayerGetState command
curl \
    --header "Content-Type: application/json" \
    --data '{}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerGetState

printf "\n"

curl \
    --header "Content-Type: application/json" \
    --data '{"name": "volume", "value": 44}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerSetProperty

printf "\n"

curl \
    --header "Content-Type: application/json" \
    --data '{}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerGetState

exit 0
printf "\n"

# PlayerOpenVideo command
curl \
    --header "Content-Type: application/json" \
    --data '{"videoid": 1}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerOpenVideo

printf "\n"
sleep 5

curl \
    --header "Content-Type: application/json" \
    --data '{}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerGetState

printf "\n"

# PlayerStop command
curl \
    --header "Content-Type: application/json" \
    --data '{}' \
    http://localhost:10123/player.v1.VideoPlayerService/PlayerStop