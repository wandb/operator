package preflight

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	apiv2 "github.com/wandb/operator/api/v2"
)

const externalRedisTimeout = 10 * time.Second

func RunExternalRedisCheck(ctx context.Context, conn *apiv2.RedisConnection, env Env) Result {
	// url is ignored like in external/redis.WriteState, which builds the app's URL from host and port.
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{
		"host":     conn.Host,
		"port":     conn.Port,
		"password": conn.Password,
		"tls":      conn.Tls,
		"sslCa":    conn.SslCa,
	})
	if err != nil {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v", err)}
	}
	host, port := strings.TrimSpace(v["host"]), strings.TrimSpace(v["port"])
	if host == "" {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomeFail, Message: "host is required"}
	}
	if !validPort(port) {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomeFail, Message: fmt.Sprintf("port %q must be an integer between 1 and 65535", port)}
	}
	tlsCfg, err := redisTLSConfig(host, v["tls"], v["sslCa"])
	if err != nil {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomeFail, Message: err.Error()}
	}

	addr := net.JoinHostPort(host, port)
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: v["password"],
		// A custom Dialer replaces go-redis's own, which is the only place TLSConfig is applied, so TLS happens here.
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if tlsCfg == nil {
				return preflightDialer.DialContext(ctx, network, addr)
			}
			return (&tls.Dialer{NetDialer: preflightDialer, Config: tlsCfg}).DialContext(ctx, network, addr)
		},
		DialTimeout: externalRedisTimeout,
		ReadTimeout: externalRedisTimeout,
		// Preflight has its own retry interval; a retrying client would only stretch the timeout.
		MaxRetries:      -1,
		DisableIdentity: true,
	})
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(ctx, externalRedisTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomeFail, Message: fmt.Sprintf("cannot connect to %s: %s", addr, redact(err.Error(), v["password"]))}
	}
	// Some hosted Redis offerings disable INFO; PING has already proven reachability and auth.
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return Result{Check: ExternalRedisCheck, Outcome: OutcomePass, Message: "connected; server version unavailable"}
	}
	return Result{Check: ExternalRedisCheck, Outcome: OutcomePass, Message: "redis " + redisInfoField(info, "redis_version")}
}

// redisTLSConfig follows external/redis.WriteState: an explicit tls value wins, otherwise a CA alone turns TLS on.
func redisTLSConfig(host, mode, ca string) (*tls.Config, error) {
	enabled := ca != ""
	if mode = strings.TrimSpace(mode); mode != "" {
		var err error
		if enabled, err = strconv.ParseBool(mode); err != nil {
			return nil, fmt.Errorf("tls %q must be true or false", mode)
		}
	}
	if !enabled {
		return nil, nil
	}
	cfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if ca != "" {
		pool, ok := parsePEMPool(ca)
		if !ok {
			return nil, fmt.Errorf("sslCa is not valid PEM")
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

func redisInfoField(info, key string) string {
	for _, line := range strings.Split(info, "\n") {
		if val, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
			return val
		}
	}
	return "unknown version"
}
