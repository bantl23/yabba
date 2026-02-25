package run

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

type Server struct {
	Address string
	Size    int
}

func (server *Server) Run() error {
	l, err := net.Listen("tcp", server.Address)
	if err != nil {
		log.WithError(err).Error("failed to listen")
		return err
	}
	defer l.Close()

	for {
		client, err := l.Accept()
		if err != nil {
			log.WithError(err).Error("failed to accept connection")
			continue
		}
		conn, ok := client.(*net.TCPConn)
		if !ok {
			log.Error("invalid connection type")
			continue
		}
		err = conn.SetReadBuffer(server.Size)
		if err != nil {
			// Non-fatal: log and continue serving other connections.
			log.WithError(err).Error("unable to set read buffer size")
		}

		log.WithFields(log.Fields{
			"address": client.LocalAddr(),
			"remote":  client.RemoteAddr(),
		}).Info("connected")

		go serverRunTcp(conn, server.Size)
	}
}

func serverRunTcp(conn *net.TCPConn, size int) {
	// Each goroutine allocates its own buffer to avoid data races.
	buffer := make([]byte, size)
	totalBytes := float64(0)
	totalElapsed := time.Duration(0)

	done := false
	for !done {
		now := time.Now()
		n, err := conn.Read(buffer)
		if err != nil {
			done = true
			continue
		}
		elapsed := time.Since(now)

		totalBytes = totalBytes + float64(n)
		totalElapsed = totalElapsed + elapsed
	}

	if totalElapsed.Seconds() > 0 {
		mbps := totalBytes * 8 / 1024 / 1024 / totalElapsed.Seconds()
		log.WithFields(log.Fields{
			"remote": conn.RemoteAddr(),
			"mbps":   mbps,
		}).Info("rate average")
	} else {
		log.WithFields(log.Fields{
			"remote": conn.RemoteAddr(),
		}).Info("connection closed before any data received")
	}
}
