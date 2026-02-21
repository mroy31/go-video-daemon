FROM golang:1.25 AS build

WORKDIR /go/src
COPY . .

# build server
RUN go get -d ./...
RUN GOOS=linux go build -o /go-video-daemon ./cmd/server/main.go

FROM debian:trixie-slim
LABEL maintainer="mickael.royer@enac.fr"

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -y && \
    apt-get install -y ffmpeg \
    	&& rm -rf /var/lib/apt/lists/*

## install server
COPY --from=build /go-video-daemon /usr/local/bin/go-video-daemon

WORKDIR /root
CMD ["/usr/local/bin/go-video-daemon"]
