package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	playerv1 "github.com/mroy31/go-video-daemon/gen/player/v1"
	"github.com/mroy31/go-video-daemon/internal/config"
	"github.com/mroy31/go-video-daemon/internal/library"
	"github.com/mroy31/go-video-daemon/internal/player"
	"github.com/sirupsen/logrus"
)

type StateSubscriber struct {
	finished chan<- bool
	stream   *connect.ServerStream[playerv1.PlayerStreamStateResponse]
}

type VideoDaemonServer struct {
	LibrayFactory *library.LibraryFactory
	Player        *player.MPVPlayer
	subscribers   sync.Map
	quit          chan struct{}
}

func (s *VideoDaemonServer) Init() error {
	s.quit = make(chan struct{})

	go func() {
		for {
			select {
			case st := <-s.Player.StateChannel:
				var unsubscribe []string
				// Iterate over all subscribers and send data to each client
				s.subscribers.Range(func(k, v interface{}) bool {
					id, ok := k.(string)
					if !ok {
						logrus.Warnf("Failed to cast state subscriber key: %T", k)
						return false
					}
					sub, ok := v.(StateSubscriber)
					if !ok {
						logrus.Warnf("Failed to cast state subscriber value: %T", v)
						return false
					}
					// Send data over the connect stream to the client
					var videoId int32 = -1
					if st.CurrentVideo != nil {
						videoId = int32(st.CurrentVideo.ID)
					}

					rs := &playerv1.PlayerStreamStateResponse{
						Volume:        int32(st.Volume),
						PlayingStatus: st.Status,
						VideoId:       videoId,
						Aid:           int32(st.AudioStreamIdx),
						Sid:           int32(st.SubtitleStreamIdx),
						TimePosition:  st.TimePosition,
					}
					if err := sub.stream.Send(rs); err != nil {
						logrus.Warnf("Failed to send player state to client: %v", err)
						select {
						case sub.finished <- true:
							logrus.Warnf("Unsubscribed client: %s", id)
						default:
							// Default case is to avoid blocking in case client has already unsubscribed
						}
						// In case of error the client would re-subscribe so close the subscriber stream
						unsubscribe = append(unsubscribe, id)
					}
					return true
				})

				// Unsubscribe erroneous client streams
				for _, id := range unsubscribe {
					s.subscribers.Delete(id)
				}

			case <-s.quit:
				return
			}
		}
	}()
	return nil
}

func (s *VideoDaemonServer) Close() {
	select {
	case s.quit <- struct{}{}:
	default:
		// Already closed
	}

	s.subscribers.Range(func(k, v interface{}) bool {
		sub, ok := v.(StateSubscriber)
		if !ok {
			logrus.Errorf("Failed to cast subscriber value: %T", v)
			return false
		}

		sub.finished <- true
		return true
	})

	s.subscribers.Clear()
}

func (s *VideoDaemonServer) GetVersion(ctx context.Context, rq *playerv1.GetVersionRequest) (*playerv1.GetVersionResponse, error) {
	return &playerv1.GetVersionResponse{
		Version: config.VERSION,
	}, nil
}

func (s *VideoDaemonServer) LibraryUpdate(ctx context.Context, rq *playerv1.LibraryUpdateRequest) (*playerv1.LibraryUpdateResponse, error) {
	library := s.LibrayFactory.GetVideoLibrary(rq.Name)
	if library == nil {
		return nil, fmt.Errorf("unbale to get %s video library: not found", rq.Name)
	}

	if err := library.Update(); err != nil {
		return nil, err
	}

	return &playerv1.LibraryUpdateResponse{}, nil
}

func (s *VideoDaemonServer) LibraryGetFolderContent(ctx context.Context, rq *playerv1.LibraryGetFolderContentRequest) (*playerv1.LibraryGetFolderContentResponse, error) {
	library := s.LibrayFactory.GetVideoLibrary(rq.Name)
	if library == nil {
		return nil, fmt.Errorf("unbale to get %s video library: not found", rq.Name)
	}

	content, err := library.GetFolderContent(rq.Folder)
	if err != nil {
		return nil, err
	}

	var rs playerv1.LibraryGetFolderContentResponse
	for _, f := range content.Childs {
		rs.Folders = append(rs.Folders, &playerv1.LibraryGetFolderContentResponse_Folder{
			Name: f.Name,
			Path: f.Path,
		})
	}
	for _, v := range content.Videos {
		var playedAt = v.PlayedAt.Unix()
		if v.PlayedAt.IsZero() {
			playedAt = 0
		}

		rs.Videos = append(rs.Videos, &playerv1.LibraryGetFolderContentResponse_VideoFile{
			Id:           int32(v.ID),
			Name:         v.Name,
			Path:         v.Path,
			Duration:     float32(v.Duration),
			PlayedAt:     playedAt,
			LastPosition: int32(v.LastPosition),
		})
	}

	return &rs, nil
}

func (s *VideoDaemonServer) LibraryGetVideoByID(ctx context.Context, rq *playerv1.LibraryGetVideoByIDRequest) (*playerv1.LibraryGetVideoByIDResponse, error) {
	video, err := s.LibrayFactory.GetVideoById(int(rq.Videoid))
	if err != nil {
		return nil, err
	}

	r := &playerv1.LibraryGetVideoByIDResponse{
		Id:       int32(video.ID),
		Name:     video.Name,
		Duration: float32(video.Duration),
	}

	for _, s := range video.AudioStreams {
		r.AudioStreams = append(r.AudioStreams, &playerv1.LibraryGetVideoByIDResponse_Stream{
			Idx:  int32(s.Idx),
			Lang: s.Lang,
		})
	}

	for _, s := range video.SubtitleStreams {
		r.SubStreams = append(r.SubStreams, &playerv1.LibraryGetVideoByIDResponse_Stream{
			Idx:  int32(s.Idx),
			Lang: s.Lang,
		})
	}

	return r, nil
}

func (s *VideoDaemonServer) PlayerGetState(ctx context.Context, rq *playerv1.PlayerGetStateRequest) (*playerv1.PlayerGetStateResponse, error) {
	state, err := s.Player.GetState()
	if err != nil {
		return nil, fmt.Errorf("unable to get mpv state: %v", err)
	}

	var videoId int32 = -1
	if state.CurrentVideo != nil {
		videoId = int32(state.CurrentVideo.ID)
	}

	return &playerv1.PlayerGetStateResponse{
		Volume:        int32(state.Volume),
		PlayingStatus: state.Status,
		VideoId:       videoId,
		Aid:           int32(state.AudioStreamIdx),
		Sid:           int32(state.SubtitleStreamIdx),
		TimePosition:  state.TimePosition,
	}, nil
}

func (s *VideoDaemonServer) PlayerStreamState(
	ctx context.Context,
	rq *playerv1.PlayerStreamStateRequest,
	stream *connect.ServerStream[playerv1.PlayerStreamStateResponse],
) error {
	st, err := s.Player.GetState()
	if err != nil {
		return fmt.Errorf("unable to get mpv state: %v", err)
	}

	// Send data over the connect stream to the client
	var videoId int32 = -1
	if st.CurrentVideo != nil {
		videoId = int32(st.CurrentVideo.ID)
	}

	if err := stream.Send(&playerv1.PlayerStreamStateResponse{
		Volume:        int32(st.Volume),
		PlayingStatus: st.Status,
		VideoId:       videoId,
		Aid:           int32(st.AudioStreamIdx),
		Sid:           int32(st.SubtitleStreamIdx),
		TimePosition:  st.TimePosition,
	}); err != nil {
		return fmt.Errorf("unable to send mpv state throught client stream: %v", err)
	}

	clientId := fmt.Sprintf("%s_%s", rq.Client, uuid.New().String())
	fin := make(chan bool)
	s.subscribers.Store(clientId, StateSubscriber{
		finished: fin,
		stream:   stream,
	})

	for {
		select {
		case <-fin:
			logrus.Infof("Closing stream for client ID: %s", rq.Client)
			s.subscribers.Delete(clientId)
			return nil
		case <-ctx.Done():
			logrus.Debugf("Client ID %s has disconnected", rq.Client)
			s.subscribers.Delete(clientId)
			return nil
		}
	}
}

func (s *VideoDaemonServer) PlayerOpenVideo(ctx context.Context, rq *playerv1.PlayerOpenVideoRequest) (*playerv1.PlayerOpenVideoResponse, error) {
	video, err := s.LibrayFactory.GetVideoById(int(rq.Videoid))
	if err != nil {
		return nil, err
	}

	if err := s.Player.OpenVideo(video); err != nil {
		return nil, err
	}

	if rq.Position > 0 {
		// wait video is played before set time-pos property
		go func() {
			for {
				try := 0
				select {
				case st := <-s.Player.StateChannel:
					if st.Status == "play" {
						s.Player.SetProperty("time-pos", rq.Position)
						return
					}
				case <-time.After(time.Second * 1):
					try++
				}

				if try > 5 {
					return
				}
			}
		}()
	}

	return &playerv1.PlayerOpenVideoResponse{}, nil
}

func (s *VideoDaemonServer) PlayerStop(ctx context.Context, rq *playerv1.PlayerStopRequest) (*playerv1.PlayerStopResponse, error) {
	if err := s.Player.Stop(); err != nil {
		return nil, err
	}

	return &playerv1.PlayerStopResponse{}, nil
}

func (s *VideoDaemonServer) PlayerPlayPause(ctx context.Context, rq *playerv1.PlayerPlayPauseRequest) (*playerv1.PlayerPlayPauseResponse, error) {
	if err := s.Player.PlayPause(); err != nil {
		return nil, err
	}

	return &playerv1.PlayerPlayPauseResponse{}, nil
}

func (s *VideoDaemonServer) PlayerSetProperty(ctx context.Context, rq *playerv1.PlayerSetPropertyRequest) (*playerv1.PlayerSetPropertyResponse, error) {
	if err := s.Player.SetProperty(rq.Name, rq.Value); err != nil {
		return nil, err
	}

	return &playerv1.PlayerSetPropertyResponse{}, nil
}

func NewServer(lFactory *library.LibraryFactory, p *player.MPVPlayer) (*VideoDaemonServer, error) {
	s := &VideoDaemonServer{
		LibrayFactory: lFactory,
		Player:        p,
	}
	if err := s.Init(); err != nil {
		return nil, fmt.Errorf("unbale to init video server: %v", err)
	}

	return s, nil
}
