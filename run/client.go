package run

import (
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	Addresses   []string
	Connections int
	Duration    time.Duration
	Size        int
}

type Routine struct {
	address     string
	item        int
	buffer      []byte
	connectChan chan struct{}
	beginChan   chan struct{}
	endChan     chan struct{}
	statsChan   chan Stats
}

func (client *Client) Run() error {
	buffer := make([]byte, client.Size)
	connectChans := make([]chan struct{}, 0)
	beginChans := make([]chan struct{}, 0)
	endChans := make([]chan struct{}, 0)
	statsChans := make([]chan Stats, 0)
	wg := &sync.WaitGroup{}

	for i := 0; i < len(client.Addresses); i++ {
		for j := 0; j < client.Connections; j++ {
			routine := Routine{
				address:     client.Addresses[i],
				item:        j,
				buffer:      buffer,
				connectChan: make(chan struct{}, 1),
				beginChan:   make(chan struct{}, 1),
				endChan:     make(chan struct{}, 1),
				statsChan:   make(chan Stats, 1),
			}
			wg.Add(1)
			go clientRunTcp(routine, wg)
			connectChans = append(connectChans, routine.connectChan)
			beginChans = append(beginChans, routine.beginChan)
			endChans = append(endChans, routine.endChan)
			statsChans = append(statsChans, routine.statsChan)
		}
	}

	// wait for all go routines to connect
	for i := range connectChans {
		<-connectChans[i]
	}

	log.Info("all connected")

	// start go routines
	for i := range beginChans {
		beginChans[i] <- struct{}{}
	}

	time.Sleep(client.Duration)

	// stop go routines
	for i := range endChans {
		endChans[i] <- struct{}{}
	}

	wg.Wait()

	printTotals(statsChans)
	return nil
}

func clientRunTcp(routine Routine, wg *sync.WaitGroup) {
	defer wg.Done()
	addr, err := net.ResolveTCPAddr("tcp", routine.address)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	err = conn.SetWriteBuffer(len(routine.buffer))
	if err != nil {
		fmt.Println(err)
		return
	}

	log.WithFields(log.Fields{
		"address": conn.LocalAddr(),
		"remote":  conn.RemoteAddr(),
	}).Info("connected")

	routine.connectChan <- struct{}{} // notify connected
	<-routine.beginChan               // block until all routines are connected

	log.WithFields(log.Fields{
		"address": conn.LocalAddr(),
		"remote":  conn.RemoteAddr(),
	}).Info("starting")

	totalBytes := uint64(0)
	totalElapsed := time.Duration(0)

	done := false
	for !done {
		now := time.Now()
		n, err := conn.Write(routine.buffer)
		if err != nil {
			done = true
			continue
		}
		elapsed := time.Since(now)

		totalBytes = totalBytes + uint64(n)
		totalElapsed = totalElapsed + elapsed

		select {
		case <-routine.endChan:
			done = true
		default:
		}
	}
	routine.statsChan <- Stats{
		Address:     conn.RemoteAddr().String(),
		Item:        routine.item,
		Bytes:       totalBytes,
		ElapsedTime: totalElapsed,
	}

	mbps := float64(totalBytes) * 8 / 1024 / 1024 / totalElapsed.Seconds()
	log.WithFields(log.Fields{
		"address": conn.LocalAddr(),
		"mbps":    mbps,
	}).Info("rate")
}

func printTotals(statsChans []chan Stats) {
	totals := make(map[string]*Stats)
	items := make(map[string]int)
	// calculate bandwidth
	for i := range statsChans {
		s := <-statsChans[i]
		_, ok := totals[s.Address]
		if !ok {
			items[s.Address] = 0
			totals[s.Address] = &Stats{
				Address:     s.Address,
				Bytes:       s.Bytes,
				ElapsedTime: s.ElapsedTime,
			}
		}
		items[s.Address] = items[s.Address] + 1
		totals[s.Address].Bytes = totals[s.Address].Bytes + s.Bytes
		totals[s.Address].ElapsedTime = totals[s.Address].ElapsedTime + s.ElapsedTime
	}
	for k := range totals {
		mbps := float64(totals[k].Bytes) * 8 / 1024 / 1024 / totals[k].ElapsedTime.Seconds() * float64(items[k])
		log.WithFields(log.Fields{
			"remote": totals[k].Address,
			"mbps":   mbps,
		}).Info("rate average")
	}
}
