package preflight

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
)

// The check sets its own Dialer, which replaces the one go-redis applies TLSConfig in, so TLS must still happen.
func TestRunExternalRedisCheck_UsesTLSThroughCustomDialer(t *testing.T) {
	certSrv := httptest.NewTLSServer(http.NotFoundHandler())
	serverTLS := certSrv.TLS
	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certSrv.Certificate().Raw}))
	certSrv.Close()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	handshake := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			handshake <- err
			return
		}
		defer func() { _ = conn.Close() }()
		handshake <- conn.(*tls.Conn).Handshake()
	}()

	conn := &apiv2.RedisConnection{
		Host:  apiv2.ValueOrSecret{Value: "127.0.0.1"},
		Port:  apiv2.ValueOrSecret{Value: strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)},
		Tls:   apiv2.ValueOrSecret{Value: "true"},
		SslCa: apiv2.ValueOrSecret{Value: caPEM},
	}
	// The listener doesn't speak RESP, so the check itself fails; only the handshake matters here.
	_ = RunExternalRedisCheck(context.Background(), conn, Env{Resolve: literalResolver})

	if err := <-handshake; err != nil {
		t.Fatalf("TLS handshake = %v, want a completed handshake", err)
	}
}
