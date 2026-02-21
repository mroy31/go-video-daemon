#!/bin/bash

 mpv \
    --input-ipc-server=/tmp/mpv-ipc-socket \
    --idle --quiet \
    --no-resume-playback
