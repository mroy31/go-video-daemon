package server_test

import (
	"context"
	"testing"

	playerv1 "github.com/mroy31/go-video-daemon/gen/player/v1"
	"github.com/mroy31/go-video-daemon/internal/config"
	"github.com/mroy31/go-video-daemon/internal/library"
	"github.com/mroy31/go-video-daemon/internal/player"
	"github.com/mroy31/go-video-daemon/internal/server"
)

func TestGetVersion(t *testing.T) {
	s := &server.VideoDaemonServer{}
	rs, err := s.GetVersion(context.Background(), &playerv1.GetVersionRequest{})
	if err != nil {
		t.Fatalf("GetVersion returned unexpected error: %v", err)
	}
	if rs.Version != config.VERSION {
		t.Fatalf("expected version %s, got %s", config.VERSION, rs.Version)
	}
}

func TestPlayerGetState_NoCurrentVideo(t *testing.T) {
	mpv := &player.MPVPlayer{}
	mpv.StateChannel = make(chan *player.MPVState)
	mpv.StateChannel <- &player.MPVState{} // harmless, not read by GetState
	mpv.StateChannel = nil

	mpv.State = player.MPVState{
		Volume:            10,
		Status:            "pause",
		CurrentVideo:      nil,
		AudioStreamIdx:    1,
		SubtitleStreamIdx: 2,
		TimePosition:      3.5,
	}

	s := &server.VideoDaemonServer{Player: mpv}
	rs, err := s.PlayerGetState(context.Background(), &playerv1.PlayerGetStateRequest{})
	if err != nil {
		t.Fatalf("PlayerGetState returned error: %v", err)
	}
	if rs.VideoId != -1 {
		t.Fatalf("expected VideoId -1 when no current video, got %d", rs.VideoId)
	}
	if rs.Volume != int32(10) {
		t.Fatalf("expected volume 10, got %d", rs.Volume)
	}
}

func TestPlayerGetState_WithCurrentVideo(t *testing.T) {
	vid := &library.Video{}
	vid.ID = 42

	mpv := &player.MPVPlayer{}
	mpv.State = player.MPVState{
		Volume:            77,
		Status:            "play",
		CurrentVideo:      vid,
		AudioStreamIdx:    2,
		SubtitleStreamIdx: 3,
		TimePosition:      12.34,
	}

	s := &server.VideoDaemonServer{Player: mpv}
	rs, err := s.PlayerGetState(context.Background(), &playerv1.PlayerGetStateRequest{})
	if err != nil {
		t.Fatalf("PlayerGetState returned error: %v", err)
	}
	if rs.VideoId != int32(vid.ID) {
		t.Fatalf("expected video id %d, got %d", vid.ID, rs.VideoId)
	}
	if rs.Volume != int32(77) {
		t.Fatalf("expected volume 77, got %d", rs.Volume)
	}
}

func TestLibraryUpdate_NotFound(t *testing.T) {
	lf := &library.LibraryFactory{Libraries: map[string]library.VideoLibrary{}}
	s := &server.VideoDaemonServer{LibrayFactory: lf}

	_, err := s.LibraryUpdate(context.Background(), &playerv1.LibraryUpdateRequest{Name: "missing"})
	if err == nil {
		t.Fatalf("expected error when updating missing library, got nil")
	}
}

func TestLibraryGetFolderContent_NotFound(t *testing.T) {
	lf := &library.LibraryFactory{Libraries: map[string]library.VideoLibrary{}}
	s := &server.VideoDaemonServer{LibrayFactory: lf}

	_, err := s.LibraryGetFolderContent(context.Background(), &playerv1.LibraryGetFolderContentRequest{Name: "missing", Folder: "/"})
	if err == nil {
		t.Fatalf("expected error when getting folder content for missing library, got nil")
	}
}
