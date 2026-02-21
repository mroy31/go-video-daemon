package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mroy31/go-video-daemon/internal/config"
	"github.com/mroy31/go-video-daemon/internal/library"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	PROPERTIES = []string{
		"volume",
		"pause",
		"idle-active",
		"aid",
		"sid",
		//"audio-delay",
		//"sub-delay",
	}
)

type MPVRequest struct {
	Command   []interface{} `json:"command"`
	RequestId int           `json:"request_id"`
}

type MPVResponse struct {
	// specific to event msg
	Id    int    `json:"id"`
	Event string `json:"event"`
	Name  string `json:"name"`

	RequestId int         `json:"request_id"`
	Error     string      `json:"error"`
	Data      interface{} `json:"data"`
	Reason    string      `json:"reason"`
}

type MPVState struct {
	Volume            int
	Status            string // play|pause|stop
	CurrentVideo      *library.Video
	AudioStreamIdx    int
	SubtitleStreamIdx int
	TimePosition      float32
}

type MPVPlayer struct {
	StateChannel  chan *MPVState
	State         MPVState
	db            *gorm.DB
	config        config.PlayerConfig
	socket        net.Conn
	lastRequestId int
	syncResponses map[int]chan MPVResponse
	mu            sync.Mutex
	monitorChan   chan bool
	quit          chan struct{}
}

func (p *MPVPlayer) Init() error {
	p.StateChannel = make(chan *MPVState)
	p.syncResponses = make(map[int]chan MPVResponse)
	p.quit = make(chan struct{})
	p.monitorChan = make(chan bool)
	p.State.Status = "stop"
	p.State.CurrentVideo = nil

	// Observe all properties
	for i, prop := range PROPERTIES {
		if err := p.sendCommand([]interface{}{"observe_property", i + 1, prop}); err != nil {
			return fmt.Errorf("failed to observe property %s: %v", prop, err)
		}
	}

	// Start message reader
	go p.readMessages()
	// Start time position monitoring goroutine
	go p.monitorTimePos()

	return nil
}

func (p *MPVPlayer) sendCommand(command []interface{}) error {
	p.mu.Lock()
	p.lastRequestId++
	req := MPVRequest{
		Command:   command,
		RequestId: p.lastRequestId,
	}
	p.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = p.socket.Write(data)
	return err
}

func (p *MPVPlayer) sendCommandSync(command []interface{}) (MPVResponse, error) {
	p.mu.Lock()
	p.lastRequestId++
	reqId := p.lastRequestId
	ch := make(chan MPVResponse, 1)
	p.syncResponses[reqId] = ch
	p.mu.Unlock()

	req := MPVRequest{
		Command:   command,
		RequestId: reqId,
	}

	data, err := json.Marshal(req)
	if err != nil {
		p.mu.Lock()
		delete(p.syncResponses, reqId)
		p.mu.Unlock()
		return MPVResponse{}, err
	}
	data = append(data, '\n')

	_, err = p.socket.Write(data)
	if err != nil {
		p.mu.Lock()
		delete(p.syncResponses, reqId)
		p.mu.Unlock()
		return MPVResponse{}, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(5 * time.Second):
		p.mu.Lock()
		delete(p.syncResponses, reqId)
		p.mu.Unlock()
		return MPVResponse{}, fmt.Errorf("timeout waiting for response")
	}
}

func (p *MPVPlayer) readMessages() {
	scanner := bufio.NewScanner(p.socket)
	for scanner.Scan() {
		select {
		case <-p.quit:
			return
		default:
		}

		var msg MPVResponse
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			logrus.Warnf("MPV - unable to read ipc responce: %v", err)
			continue
		}

		// Handle synchronous responses
		if msg.RequestId != 0 {
			if ch, ok := p.syncResponses[msg.RequestId]; ok {
				select {
				case ch <- msg:
				default:
				}

				p.mu.Lock()
				delete(p.syncResponses, msg.RequestId)
				p.mu.Unlock()
			}
		}

		if msg.Event == "property-change" {
			switch msg.Name {
			case "volume":
				if data, ok := msg.Data.(float64); ok {
					p.State.Volume = int(data)
				}
			case "pause":
				if data, ok := msg.Data.(bool); ok {
					p.monitorChan <- !data
					if data {
						p.State.Status = "pause"
					} else {
						p.State.Status = "play"
					}
				}
			case "idle-active":
				if data, ok := msg.Data.(bool); ok {
					if data {
						p.monitorChan <- false
						p.State.Status = "stop"
					}
				}
			case "aid":
				if data, ok := msg.Data.(float64); ok {
					p.State.AudioStreamIdx = int(data)
				}
			case "sid":
				if data, ok := msg.Data.(float64); ok {
					p.State.SubtitleStreamIdx = int(data)
				}
			}
			p.StateChannel <- &p.State
		}

		if msg.Event == "end-file" {
			switch msg.Reason {
			case "eof":
				p.State.CurrentVideo.LastPosition = 0
			case "stop":
				p.State.CurrentVideo.LastPosition = int(p.State.TimePosition)
			}

			// update current video in the db before forget it
			p.State.CurrentVideo.PlayedAt = time.Now()
			p.db.Save(p.State.CurrentVideo)
			p.State.CurrentVideo = nil

			p.State.Status = "stop"
			p.monitorChan <- false
			p.StateChannel <- &p.State
		}

		if msg.Event == "playback-restart" {
			p.State.Status = "play"
			p.monitorChan <- true
			p.StateChannel <- &p.State
		}

	}
}

func (p *MPVPlayer) monitorTimePos() {
	isMonitoring := false

	for {
		select {
		case <-p.quit:
			return
		case isMonitoring = <-p.monitorChan:
		case <-time.After(time.Second * 1):
		}

		if isMonitoring {
			if err := p.updateTimePos(); err != nil {
				logrus.Warnf("MPV - unable to get time pos: %v", err)
				continue
			}
			p.StateChannel <- &p.State
		}
	}
}

func (p *MPVPlayer) updateTimePos() error {
	if p.State.CurrentVideo != nil {
		property, err := p.sendCommandSync([]interface{}{"get_property", "time-pos"})
		if err != nil {
			return err
		}

		if property.Error != "success" {
			return fmt.Errorf("unable to get playback-time prop: %s", property.Error)
		}

		if data, ok := property.Data.(float64); ok {
			p.State.TimePosition = float32(data)
		}
	}

	return nil
}

func (p *MPVPlayer) GetState() (*MPVState, error) {
	return &p.State, nil
}

func (p *MPVPlayer) OpenVideo(video *library.Video) error {
	// Load the file into MPV
	if _, err := p.sendCommandSync([]interface{}{"loadfile", video.Path}); err != nil {
		return fmt.Errorf("failed to load file: %v", err)
	}
	p.State.CurrentVideo = video

	// Ensure playback is started (unpause)
	if _, err := p.sendCommandSync([]interface{}{"set_property", "pause", false}); err != nil {
		return fmt.Errorf("failed to start playback: %v", err)
	}

	return nil
}

func (p *MPVPlayer) Stop() error {
	// update playback time before stopping the video
	if err := p.updateTimePos(); err != nil {
		logrus.Warnf("MPV - unable to update playback time: %v", err)
	}

	// Stop playback in MPV
	if _, err := p.sendCommandSync([]interface{}{"stop"}); err != nil {
		return fmt.Errorf("failed to stop playback: %v", err)
	}

	return nil
}

func (p *MPVPlayer) PlayPause() error {
	// Toggle pause state in MPV
	if _, err := p.sendCommandSync([]interface{}{"cycle", "pause"}); err != nil {
		return fmt.Errorf("failed to toggle pause: %v", err)
	}

	return nil
}

func (p *MPVPlayer) SetProperty(name string, value int32) error {
	if _, err := p.sendCommandSync([]interface{}{"set_property", name, value}); err != nil {
		return fmt.Errorf("failed to set property %s to %d: %v", name, value, err)
	}

	return nil
}

func (p *MPVPlayer) Close() error {
	// Stop the message reader goroutine
	select {
	case p.quit <- struct{}{}:
	default:
		// Already closed
	}

	// Close the socket
	if p.socket != nil {
		return p.socket.Close()
	}
	return nil
}

func InitMPVPlayer(db *gorm.DB, config config.PlayerConfig) (*MPVPlayer, error) {
	const attempts = 5
	var c net.Conn
	var err error

	for i := 1; i <= attempts; i++ {
		c, err = net.Dial("unix", config.Socket)
		if err == nil {
			break
		}
		if i < attempts {
			logrus.Warnf("MPV: try %d/%d - failed to connect to IPC socket: %v", i, attempts, err)
			time.Sleep(5 * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("unable to connect to mpv socket after %d attempts: %v", attempts, err)
	}

	player := &MPVPlayer{
		db:            db,
		config:        config,
		socket:        c,
		lastRequestId: 1,
	}
	if err := player.Init(); err != nil {
		c.Close()
		return nil, err
	}

	return player, nil
}
