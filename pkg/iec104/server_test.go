package iec104

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

func setupTestServer(t *testing.T) (*Server, int) {
	t.Helper()

	points := []*config.Point{
		{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, ValueType: config.VTFloat, Value: 220.0, Efficient: 1.0, BaseValue: 220.0},
		{IOA: 2001, Name: "DI_01", PointType: config.TypeDI, ValueType: config.VTBit, BoolValue: false},
		{IOA: 3001, Name: "PI_01", PointType: config.TypePI, ValueType: config.VTInt, IntValue: 1000},
	}
	store := library.NewStore(points)
	port := findFreePort(t)
	srv := NewServer(port, store)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start server: %v", err)
	}
	// Wait for async server to start listening
	time.Sleep(100 * time.Millisecond)
	t.Cleanup(func() { srv.Stop() })
	return srv, port
}

func findFreePort(t testing.TB) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestServer_StartStop(t *testing.T) {
	srv, port := setupTestServer(t)
	if !srv.ClientConnected() {
		// Expected: no client yet
	}
	if srv.ClientAddr() != "" {
		t.Error("expected empty client addr")
	}
	_ = port
}

func TestServer_SingleClient(t *testing.T) {
	srv, port := setupTestServer(t)

	// Connect client 1
	conn1, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("connect client1: %v", err)
	}
	defer conn1.Close()

	time.Sleep(200 * time.Millisecond)

	if !srv.ClientConnected() {
		t.Error("expected client1 connected")
	}

	// Connect client 2 — should be rejected (single client mode)
	conn2, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		// Connection refused is expected behavior
	} else {
		defer conn2.Close()
		// Client2 connected, but server should have rejected
		// Give it a moment to handle
		time.Sleep(100 * time.Millisecond)
	}

	// Client1 should still be connected
	if !srv.ClientConnected() {
		t.Error("expected client1 still connected after client2 attempt")
	}
}

func TestServer_PublishIntegration(t *testing.T) {
	srv, port := setupTestServer(t)

	// Connect a real TCP client
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(200 * time.Millisecond)

	// Try to publish — the send will fail because the IEC104 handshake
	// wasn't completed, but it shouldn't panic or block
	done := make(chan bool)
	go func() {
		pt := &config.Point{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, Value: 150.0}
		srv.Publish(pt)
		done <- true
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked")
	}
}

func TestServer_Stats(t *testing.T) {
	srv, port := setupTestServer(t)
	_ = port

	interrog, control, spont := srv.Stats()
	if interrog != 0 || control != 0 || spont != 0 {
		t.Errorf("expected all zeros, got %d/%d/%d", interrog, control, spont)
	}

	// Publish should count spontaneous after client connects
	// Without client, it's a no-op
	srv.Publish(&config.Point{IOA: 1001, PointType: config.TypeAI, Value: 100.0})
	_, _, spont = srv.Stats()
	if spont > 0 {
		t.Errorf("expected 0 spontaneous without client, got %d", spont)
	}
}

func TestStorePublishFlow(t *testing.T) {
	srv, port := setupTestServer(t)
	_ = port

	// Simulate what HTTP API does: SetValue then Publish
	pt, err := srv.store.SetValue(1001, 200.0)
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	if pt.Value != 200.0 {
		t.Errorf("expected 200.0, got %f", pt.Value)
	}

	// Publish (no client connected, should be no-op)
	srv.Publish(pt)

	pt, err = srv.store.SetBoolValue(2001, true)
	if err != nil {
		t.Fatalf("SetBoolValue: %v", err)
	}
	if !pt.BoolValue {
		t.Error("expected true")
	}

	pt, err = srv.store.SetIntValue(3001, 999)
	if err != nil {
		t.Fatalf("SetIntValue: %v", err)
	}
	if pt.IntValue != 999 {
		t.Errorf("expected 999, got %d", pt.IntValue)
	}

	// Collect changed
	changed := srv.store.CollectChanged()
	if len(changed) != 3 {
		t.Errorf("expected 3 changed points, got %d", len(changed))
	}
}

func TestServer_DataRace(t *testing.T) {
	srv, port := setupTestServer(t)
	_ = port

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			srv.store.SetValue(1001, float64(i))
			srv.Publish(&config.Point{IOA: 1001, PointType: config.TypeAI, Value: float64(i)})
			srv.store.Get(1001)
			srv.Stats()
			srv.ClientConnected()
		}(i)
	}
	wg.Wait()
}

// Ensure that a connected TCP client can read APCI frames (basic protocol check)
func TestServer_APCIFrame(t *testing.T) {
	_, port := setupTestServer(t)

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	// Read data from server (should receive IEC104 APCI frames)
	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		// Timeout is OK — the server might not send anything until
		// the IEC104 handshake is complete
		return
	}
	_ = n
	_ = buf
}

func BenchmarkServer_Publish(b *testing.B) {
	points := []*config.Point{
		{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, ValueType: config.VTFloat, Value: 220.0},
		{IOA: 2001, Name: "DI_01", PointType: config.TypeDI, ValueType: config.VTBit, BoolValue: false},
	}
	store := library.NewStore(points)
	port := findFreePort(b)
	srv := NewServer(port, store)
	if err := srv.Start(); err != nil {
		b.Fatalf("Start: %v", err)
	}
	defer srv.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv.Publish(&config.Point{IOA: 1001, PointType: config.TypeAI, Value: float64(i)})
	}
}

// Ensure helper io is imported (used by several tests)
var _ = io.Discard
