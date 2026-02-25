package run

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

// captureLog redirects logrus output to a JSON-formatted buffer for the
// duration of fn, then returns everything that was logged. It temporarily
// overrides both the output destination and the formatter so the JSON parser
// in logMbps works regardless of the formatter set by main.go.
func captureLog(fn func()) string {
	var buf bytes.Buffer
	orig := log.StandardLogger().Out
	origFmt := log.StandardLogger().Formatter
	log.SetOutput(&buf)
	log.SetFormatter(&log.JSONFormatter{})
	defer func() {
		log.SetOutput(orig)
		log.SetFormatter(origFmt)
	}()
	fn()
	return buf.String()
}

// logMbps parses JSON logrus output and returns the "mbps" value from the
// first entry whose "msg" field equals msgField.
func logMbps(t *testing.T, output, msgField string) float64 {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry["msg"] == msgField {
			if mbps, ok := entry["mbps"].(float64); ok {
				return mbps
			}
		}
	}
	t.Fatalf("no log entry with msg=%q in:\n%s", msgField, output)
	return 0
}

// makeStats creates a buffered Stats channel pre-loaded with one value.
func makeStats(s Stats) chan Stats {
	ch := make(chan Stats, 1)
	ch <- s
	return ch
}

// --------------------------------------------------------------------------
// printTotals unit tests
// --------------------------------------------------------------------------

func TestPrintTotals_Empty(t *testing.T) {
	// Neither nil nor empty slice should panic or block.
	log.SetOutput(io.Discard)
	defer log.SetOutput(io.Discard)

	printTotals(nil)
	printTotals([]chan Stats{})
}

func TestPrintTotals_SingleConnection(t *testing.T) {
	// One connection, 1 MiB in 1 s → 8 Mbps.
	ch := makeStats(Stats{
		Address:     "127.0.0.1:5201",
		Item:        0,
		Bytes:       1 * 1024 * 1024,
		ElapsedTime: time.Second,
	})

	out := captureLog(func() { printTotals([]chan Stats{ch}) })

	got := logMbps(t, out, "rate average")
	const want = 8.0
	if got != want {
		t.Errorf("mbps = %v, want %v", got, want)
	}
}

// TestPrintTotals_DoubleCountingFix verifies the bug fix where the first
// entry per address was being counted twice. Two connections send different
// byte counts so the correct and buggy formulas yield distinguishable results:
//
//	correct: (2+1) MiB * 8 / (1+1) s * 2 connections = 24 Mbps
//	buggy  : (4+1) MiB * 8 / (2+1) s * 2 connections ≈ 26.67 Mbps
func TestPrintTotals_DoubleCountingFix(t *testing.T) {
	addr := "127.0.0.1:5201"
	ch1 := makeStats(Stats{Address: addr, Item: 0, Bytes: 2 * 1024 * 1024, ElapsedTime: time.Second})
	ch2 := makeStats(Stats{Address: addr, Item: 1, Bytes: 1 * 1024 * 1024, ElapsedTime: time.Second})

	out := captureLog(func() { printTotals([]chan Stats{ch1, ch2}) })

	got := logMbps(t, out, "rate average")
	const want = 24.0
	if got != want {
		t.Errorf("mbps = %v, want %v (double-counting bug may be present)", got, want)
	}
}

func TestPrintTotals_MultipleAddresses(t *testing.T) {
	// Two addresses must produce separate totals and not bleed into each other.
	ch1 := makeStats(Stats{Address: "127.0.0.1:5201", Item: 0, Bytes: 1 * 1024 * 1024, ElapsedTime: time.Second})
	ch2 := makeStats(Stats{Address: "127.0.0.1:5202", Item: 0, Bytes: 2 * 1024 * 1024, ElapsedTime: time.Second})

	out := captureLog(func() { printTotals([]chan Stats{ch1, ch2}) })

	// Both addresses must appear.
	if !strings.Contains(out, "5201") {
		t.Error("expected log entry for address :5201")
	}
	if !strings.Contains(out, "5202") {
		t.Error("expected log entry for address :5202")
	}
}

func TestPrintTotals_ZeroElapsed(t *testing.T) {
	// Zero elapsed time must not cause a division by zero panic.
	ch := makeStats(Stats{
		Address:     "127.0.0.1:5201",
		Item:        0,
		Bytes:       0,
		ElapsedTime: 0,
	})

	out := captureLog(func() { printTotals([]chan Stats{ch}) })

	if !strings.Contains(out, "no data transferred") {
		t.Errorf("expected 'no data transferred' message, got:\n%s", out)
	}
}

func TestPrintTotals_DoesNotBlock(t *testing.T) {
	// printTotals must drain all channels and return; it must not block.
	chs := make([]chan Stats, 5)
	for i := range chs {
		chs[i] = makeStats(Stats{
			Address:     "127.0.0.1:5201",
			Item:        i,
			Bytes:       uint64(i+1) * 1024 * 1024,
			ElapsedTime: time.Second,
		})
	}

	done := make(chan struct{})
	go func() {
		captureLog(func() { printTotals(chs) })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("printTotals blocked and did not return")
	}
}

// --------------------------------------------------------------------------
// clientRunTcp unit tests
// --------------------------------------------------------------------------

// drainListener starts an accept-and-discard loop on l and returns when
// the test ends (caller must close l to stop the loop).
func drainListener(l net.Listener) {
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 32*1024)
				for {
					if _, err := c.Read(buf); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
}

func TestClientRunTcp_BadAddress(t *testing.T) {
	// An unresolvable address must signal connectChan and send zero stats
	// so the caller never deadlocks.
	routine := Routine{
		address:     "not_an_address",
		item:        0,
		buffer:      make([]byte, 1024),
		connectChan: make(chan struct{}, 1),
		beginChan:   make(chan struct{}, 1),
		endChan:     make(chan struct{}, 1),
		statsChan:   make(chan Stats, 1),
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	log.SetOutput(io.Discard)
	go clientRunTcp(routine, wg)

	select {
	case <-routine.connectChan:
	case <-time.After(2 * time.Second):
		t.Fatal("connectChan was not signalled after resolve failure")
	}

	wg.Wait()

	select {
	case s := <-routine.statsChan:
		if s.Bytes != 0 {
			t.Errorf("expected 0 bytes on failure, got %d", s.Bytes)
		}
	default:
		t.Fatal("statsChan was not populated after resolve failure")
	}
}

func TestClientRunTcp_ConnectFailure(t *testing.T) {
	// Listen on an ephemeral port then immediately close it, guaranteeing
	// the dial attempt will receive ECONNREFUSED.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	routine := Routine{
		address:     addr,
		item:        0,
		buffer:      make([]byte, 1024),
		connectChan: make(chan struct{}, 1),
		beginChan:   make(chan struct{}, 1),
		endChan:     make(chan struct{}, 1),
		statsChan:   make(chan Stats, 1),
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	log.SetOutput(io.Discard)
	go clientRunTcp(routine, wg)

	select {
	case <-routine.connectChan:
	case <-time.After(2 * time.Second):
		t.Fatal("connectChan was not signalled after dial failure")
	}

	wg.Wait()

	select {
	case s := <-routine.statsChan:
		if s.Bytes != 0 {
			t.Errorf("expected 0 bytes on failure, got %d", s.Bytes)
		}
	default:
		t.Fatal("statsChan was not populated after dial failure")
	}
}

func TestClientRunTcp_Success(t *testing.T) {
	// Happy path: goroutine connects, writes data, and sends non-zero stats.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	drainListener(l)

	routine := Routine{
		address:     l.Addr().String(),
		item:        0,
		buffer:      make([]byte, 4*1024),
		connectChan: make(chan struct{}, 1),
		beginChan:   make(chan struct{}, 1),
		endChan:     make(chan struct{}, 1),
		statsChan:   make(chan Stats, 1),
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	log.SetOutput(io.Discard)
	go clientRunTcp(routine, wg)

	select {
	case <-routine.connectChan:
	case <-time.After(2 * time.Second):
		t.Fatal("connectChan was not signalled")
	}

	routine.beginChan <- struct{}{}
	time.Sleep(50 * time.Millisecond)
	routine.endChan <- struct{}{}
	wg.Wait()

	select {
	case s := <-routine.statsChan:
		if s.Bytes == 0 {
			t.Error("expected non-zero bytes after successful run")
		}
		if s.ElapsedTime == 0 {
			t.Error("expected non-zero elapsed time after successful run")
		}
	default:
		t.Fatal("statsChan was not populated after successful run")
	}
}

func TestClientRunTcp_WriteErrorSendsStats(t *testing.T) {
	// The server closes its side of the connection before the client writes,
	// causing a write error. statsChan must still be sent (via defer).
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	// Accept once and immediately close to trigger a write error on the client.
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	routine := Routine{
		address:     l.Addr().String(),
		item:        0,
		buffer:      make([]byte, 4*1024),
		connectChan: make(chan struct{}, 1),
		beginChan:   make(chan struct{}, 1),
		endChan:     make(chan struct{}, 1),
		statsChan:   make(chan Stats, 1),
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)

	log.SetOutput(io.Discard)
	go clientRunTcp(routine, wg)

	select {
	case <-routine.connectChan:
	case <-time.After(2 * time.Second):
		t.Fatal("connectChan was not signalled")
	}

	routine.beginChan <- struct{}{}
	wg.Wait()

	select {
	case <-routine.statsChan:
		// success: statsChan was sent even on write error
	default:
		t.Fatal("statsChan was not populated after write error")
	}
}

// --------------------------------------------------------------------------
// serverRunTcp unit tests
// --------------------------------------------------------------------------

// acceptTCP starts a listener, dials from a client goroutine, and returns
// the server-side *net.TCPConn. Caller owns both conn and the client conn.
func acceptTCP(t *testing.T) (serverConn *net.TCPConn, clientConn net.Conn) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	serverCh := make(chan *net.TCPConn, 1)
	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			return
		}
		serverCh <- conn.(*net.TCPConn)
	}()

	clientConn, err = net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	serverConn = <-serverCh
	return serverConn, clientConn
}

func TestServerRunTcp_ZeroElapsed(t *testing.T) {
	// When the client closes immediately (no data), elapsed is zero.
	// Must not divide by zero; must log the "no data" message instead.
	serverConn, clientConn := acceptTCP(t)
	clientConn.Close() // EOF immediately

	out := captureLog(func() { serverRunTcp(serverConn, 4096) })

	if !strings.Contains(out, "connection closed before any data received") {
		t.Errorf("expected zero-elapsed log message, got:\n%s", out)
	}
}

func TestServerRunTcp_DataAccumulation(t *testing.T) {
	// After receiving data, serverRunTcp must log a "rate average" entry.
	serverConn, clientConn := acceptTCP(t)

	go func() {
		defer clientConn.Close()
		buf := make([]byte, 4096)
		for i := 0; i < 20; i++ {
			if _, err := clientConn.Write(buf); err != nil {
				return
			}
		}
	}()

	out := captureLog(func() { serverRunTcp(serverConn, 4096) })

	if !strings.Contains(out, "rate average") {
		t.Errorf("expected 'rate average' log entry, got:\n%s", out)
	}
}

func TestServerRunTcp_ConcurrentBuffers(t *testing.T) {
	// Two concurrent serverRunTcp calls must not race on a shared buffer.
	// Run with -race to validate the per-goroutine allocation fix.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	const numConns = 2
	serverConns := make(chan *net.TCPConn, numConns)

	go func() {
		for i := 0; i < numConns; i++ {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			serverConns <- conn.(*net.TCPConn)
		}
	}()

	// Dial numConns clients; each writes distinct data then closes.
	for i := 0; i < numConns; i++ {
		go func(val byte) {
			client, err := net.Dial("tcp", l.Addr().String())
			if err != nil {
				return
			}
			defer client.Close()
			buf := bytes.Repeat([]byte{val}, 32*1024)
			for j := 0; j < 10; j++ {
				if _, err := client.Write(buf); err != nil {
					return
				}
			}
		}(byte(i + 1))
	}

	var wg sync.WaitGroup
	log.SetOutput(io.Discard)
	for i := 0; i < numConns; i++ {
		conn := <-serverConns
		wg.Add(1)
		go func(c *net.TCPConn) {
			defer wg.Done()
			serverRunTcp(c, 4096)
		}(conn)
	}
	wg.Wait()
}

// --------------------------------------------------------------------------
// Client.Run integration tests
// --------------------------------------------------------------------------

func TestClientRun_EndToEnd(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	drainListener(l)

	client := Client{
		Addresses:   []string{l.Addr().String()},
		Connections: 1,
		Duration:    100 * time.Millisecond,
		Size:        4 * 1024,
	}

	log.SetOutput(io.Discard)
	if err := client.Run(); err != nil {
		t.Fatalf("client.Run() error: %v", err)
	}
}

func TestClientRun_MultipleConnsMultipleAddresses(t *testing.T) {
	// Two listeners, two connections each → four goroutines; all must complete
	// and printTotals must drain all four statsChan entries without deadlocking.
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 1: %v", err)
	}
	defer l1.Close()
	drainListener(l1)

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 2: %v", err)
	}
	defer l2.Close()
	drainListener(l2)

	client := Client{
		Addresses:   []string{l1.Addr().String(), l2.Addr().String()},
		Connections: 2,
		Duration:    100 * time.Millisecond,
		Size:        4 * 1024,
	}

	log.SetOutput(io.Discard)
	done := make(chan error, 1)
	go func() { done <- client.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("client.Run() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client.Run() did not return (deadlock?)")
	}
}

func TestClientRun_AllConnectFailures(t *testing.T) {
	// All connection attempts fail (port closed). Client.Run must still
	// return nil without deadlocking at the connectChan drain loop.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	l.Close() // nothing is listening

	client := Client{
		Addresses:   []string{addr},
		Connections: 3,
		Duration:    100 * time.Millisecond,
		Size:        4 * 1024,
	}

	log.SetOutput(io.Discard)
	done := make(chan error, 1)
	go func() { done <- client.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("client.Run() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client.Run() did not return (deadlock?)")
	}
}

// --------------------------------------------------------------------------
// Server.Run integration tests
// --------------------------------------------------------------------------

func TestServerRun_ListenError(t *testing.T) {
	// Binding to an address already in use must return a non-nil error.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	server := &Server{Address: l.Addr().String(), Size: 4096}
	if err := server.Run(); err == nil {
		t.Error("expected non-nil error when address is already in use, got nil")
	}
}

func TestServerRun_AcceptsAndDispatches(t *testing.T) {
	// Server.Run accepts a connection and dispatches it to serverRunTcp.
	// Server.Run has no shutdown path, so the goroutine is intentionally
	// leaked; the test process cleans it up on exit.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	server := &Server{Address: addr, Size: 4096}

	log.SetOutput(io.Discard)
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Run() }()

	// Wait briefly for the listener to become ready.
	time.Sleep(20 * time.Millisecond)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	buf := make([]byte, 4096)
	for i := 0; i < 5; i++ {
		if _, err := conn.Write(buf); err != nil {
			break
		}
	}
	conn.Close()

	// Give serverRunTcp time to log its result.
	time.Sleep(100 * time.Millisecond)

	// Server.Run itself should still be alive (infinite accept loop).
	select {
	case err := <-serverDone:
		t.Fatalf("server.Run() exited unexpectedly: %v", err)
	default:
		// expected: server is still running
	}
}
