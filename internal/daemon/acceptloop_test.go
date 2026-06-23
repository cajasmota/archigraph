package daemon

import (
	"io"
	"log/slog"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// timeoutErr is a synthetic net.Error whose Timeout() is true — the modern
// stand-in for the deprecated Temporary() that http.Server.Serve keys off.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "synthetic timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// fakeListener returns a scripted sequence of Accept() results.
type fakeListener struct {
	mu    sync.Mutex
	steps []acceptStep
	calls int32
}

type acceptStep struct {
	conn net.Conn
	err  error
}

func (f *fakeListener) Accept() (net.Conn, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.steps) == 0 {
		// Exhausted — behave like a closed listener so the loop exits.
		return nil, net.ErrClosed
	}
	step := f.steps[0]
	f.steps = f.steps[1:]
	return step.conn, step.err
}

func (f *fakeListener) Close() error   { return nil }
func (f *fakeListener) Addr() net.Addr { return fakeAddr{} }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

// pipeConn is a no-op net.Conn good enough for ServeCodec to attempt a read
// and then unblock. Read returns EOF immediately so the conn goroutine exits.
type pipeConn struct{ served *int32 }

func (p *pipeConn) Read(b []byte) (int, error)         { atomic.AddInt32(p.served, 1); return 0, io.EOF }
func (p *pipeConn) Write(b []byte) (int, error)        { return len(b), nil }
func (p *pipeConn) Close() error                       { return nil }
func (p *pipeConn) LocalAddr() net.Addr                { return fakeAddr{} }
func (p *pipeConn) RemoteAddr() net.Addr               { return fakeAddr{} }
func (p *pipeConn) SetDeadline(t time.Time) error      { return nil }
func (p *pipeConn) SetReadDeadline(t time.Time) error  { return nil }
func (p *pipeConn) SetWriteDeadline(t time.Time) error { return nil }

func TestAcceptLoopSurvivesTransientErrors(t *testing.T) {
	var served int32
	conn := &pipeConn{served: &served}

	fl := &fakeListener{steps: []acceptStep{
		{err: timeoutErr{}},   // transient: net.Error Timeout()
		{err: syscall.EMFILE}, // transient: fd exhaustion
		{conn: conn},          // a real connection — must be served
		{err: net.ErrClosed},  // clean shutdown
	}}

	srv := rpc.NewServer()
	var wg sync.WaitGroup
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	start := time.Now()
	// Backoff is 5ms then 10ms = ~15ms total for two transient errors.
	go acceptLoop(fl, srv, &wg, logger, done)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("acceptLoop did not return; it should exit cleanly on ErrClosed")
	}
	wg.Wait()

	// It must NOT have exited on the transient errors: all four scripted
	// steps must have been consumed (4 Accept calls).
	if got := atomic.LoadInt32(&fl.calls); got < 4 {
		t.Fatalf("acceptLoop exited early: only %d Accept() calls, want >=4 "+
			"(transient errors were treated as fatal)", got)
	}
	// The real conn must have been served (its Read called once).
	if got := atomic.LoadInt32(&served); got != 1 {
		t.Fatalf("real conn not served: served=%d want 1", got)
	}
	// Backoff sanity: two transient errors back off at least 5ms+10ms.
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("backoff too short: %v; expected >=10ms for two transient errors", elapsed)
	}
}

func TestAcceptLoopBackoffResets(t *testing.T) {
	var served int32
	// Transient, success, then three transient in a row. If backoff did NOT
	// reset after the successful Accept, the post-success transient sleeps
	// would start from a doubled value. We assert the loop consumes every
	// scripted step (i.e. never treats transient as fatal) and serves both
	// real conns.
	fl := &fakeListener{steps: []acceptStep{
		{err: syscall.ECONNABORTED},
		{conn: &pipeConn{served: &served}},
		{err: syscall.ENOMEM},
		{err: timeoutErr{}},
		{conn: &pipeConn{served: &served}},
		{err: net.ErrClosed},
	}}

	srv := rpc.NewServer()
	var wg sync.WaitGroup
	done := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	go acceptLoop(fl, srv, &wg, logger, done)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("acceptLoop did not return")
	}
	wg.Wait()

	if got := atomic.LoadInt32(&fl.calls); got < 6 {
		t.Fatalf("acceptLoop exited early: %d Accept() calls, want >=6", got)
	}
	if got := atomic.LoadInt32(&served); got != 2 {
		t.Fatalf("real conns served=%d want 2", got)
	}
}

func TestIsTransientAcceptErr(t *testing.T) {
	transient := []error{
		syscall.EMFILE, syscall.ENFILE, syscall.ENOMEM,
		syscall.ENOBUFS, syscall.ECONNABORTED, timeoutErr{},
	}
	for _, e := range transient {
		if !isTransientAcceptErr(e) {
			t.Errorf("isTransientAcceptErr(%v) = false, want true", e)
		}
	}
	if isTransientAcceptErr(net.ErrClosed) {
		t.Error("net.ErrClosed should not be classified as transient")
	}
	if isTransientAcceptErr(syscall.EINVAL) {
		t.Error("EINVAL should not be classified as transient")
	}
}
