package run

import (
	"fmt"
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
		fmt.Println(err)
		return err
	}
	defer l.Close()

	buffer := make([]byte, server.Size)
	for {
		client, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		conn, ok := client.(*net.TCPConn)
		if !ok {
			log.Error("invalid connection type")
			continue
		}
		err = conn.SetReadBuffer(len(buffer))
		if err != nil {
			log.Error("unable to set read buffer size")
			return err
		}

		log.WithFields(log.Fields{
			"address": client.LocalAddr(),
			"remote":  client.RemoteAddr(),
		}).Info("connected")

		go serverRunTcp(conn, buffer)
	}
}

func serverRunTcp(conn *net.TCPConn, buffer []byte) {
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
	mbps := float64(totalBytes) * 8 / 1024 / 1024 / totalElapsed.Seconds()

	log.WithFields(log.Fields{
		"remote": conn.RemoteAddr(),
		"mbps":   mbps,
	}).Info("rate average")
}
