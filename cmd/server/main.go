package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mroy31/go-video-daemon/gen/player/v1/playerv1connect"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"connectrpc.com/validate"
	"github.com/rs/cors"

	"github.com/mroy31/go-video-daemon/internal/config"
	"github.com/mroy31/go-video-daemon/internal/library"
	"github.com/mroy31/go-video-daemon/internal/player"
	"github.com/mroy31/go-video-daemon/internal/server"
	"github.com/sirupsen/logrus"
)

var (
	verbose   = flag.Bool("verbose", false, "Display more messages")
	logFile   = flag.String("log-file", "", "Path of the log file (default: stdout)")
	conf      = flag.String("conf-file", config.CONFIG_FILE, "Configuration path")
	isRunning = true
)

// withCORS adds CORS support to a Connect HTTP handler.
func withCORS(h http.Handler) http.Handler {
	middleware := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	})
	return middleware.Handler(h)
}

func main() {
	flag.Parse()

	// init log
	logWriter := os.Stderr
	if *logFile != "" {
		f, err := os.Create(*logFile)
		if err != nil {
			logrus.Fatalf("Unable to create log file %s: %v", *logFile, err)
		}
		defer f.Close()

		logWriter = f
	}
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(logWriter)
	logrus.SetLevel(logrus.InfoLevel)
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Info("Starting go-video-daemon - version " + config.VERSION)

	// parse config
	config.InitServerConfig()
	if _, err := os.Stat(*conf); os.IsNotExist(err) {
		logrus.Warnf("Config file %s not exit, skip it", *conf)
	} else {
		if err := config.ParseServerConfig(*conf); err != nil {
			logrus.Fatalf("Unable to parse config file %s: %v", *conf, err)
		}
	}

	// init database and libraries
	lFactory, err := library.InitLibraryFactory(config.ServerConfig.Library)
	if err != nil {
		logrus.Fatalf("Init LibraryFactory - %v", err)
	}

	// update all libraries
	if err := lFactory.UpdateAllLibraries(); err != nil {
		logrus.Fatalf("Unable to update libraries: %v", err)
	}

	// init player
	player, err := player.InitMPVPlayer(lFactory.Db, config.ServerConfig.Player)
	if err != nil {
		logrus.Fatalf("Init MPVPlayer - %v", err)
	}

	videoServer, err := server.NewServer(lFactory, player)
	if err != nil {
		logrus.Fatalf("Init VideoServer - %v", err)
	}

	// Init and start HTTP server
	mux := http.NewServeMux()
	path, handler := playerv1connect.NewVideoPlayerServiceHandler(
		videoServer,
		// Validation via Protovalidate is almost always recommended
		connect.WithInterceptors(validate.NewInterceptor()),
	)
	mux.Handle(path, withCORS(handler))

	p := new(http.Protocols)
	p.SetHTTP1(true)
	// Use h2c so we can serve HTTP/2 without TLS.
	p.SetUnencryptedHTTP2(true)

	s := http.Server{
		Addr:      config.ServerConfig.Listen,
		Handler:   mux,
		Protocols: p,
	}
	go func() {
		logrus.Infof("Listen on: %s", config.ServerConfig.Listen)
		if err := s.ListenAndServe(); err != nil {
			if isRunning {
				logrus.Fatalf("Unable to launch http server on %s - %v\n", config.ServerConfig.Listen, err)
			} else {
				logrus.Infoln("HTTP: Close server")
			}
		}
	}()

	done := make(chan bool, 1)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		isRunning = false
		logrus.Warnln("\r- Ctrl+C pressed in Terminal")
		// close video server
		videoServer.Close()

		// stop http server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			// handle err
			logrus.Errorf("Unable to stop HTTP server : %s", err)
		}

		// close player
		player.Close()

		done <- true
	}()

	<-done
}
