package preflight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	apiv2 "github.com/wandb/operator/api/v2"
)

const (
	externalMysqlTimeout = 10 * time.Second
)

func RunExternalMysqlCheck(ctx context.Context, conn *apiv2.MysqlConnection, resolve ValueResolver) Result {
	v, err := resolveFields(ctx, resolve, map[string]apiv2.ValueOrSecret{
		"host":     conn.Host,
		"port":     conn.Port,
		"username": conn.Username,
		"password": conn.Password,
		"tls":      conn.Tls,
		"sslCa":    conn.SslCa,
		"sslCert":  conn.SslCert,
		"sslKey":   conn.SslKey,
	})
	if err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v", err)}
	}
	if v["host"] == "" {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: "host is required"}
	}
	if v["port"] == "" {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: "port is required"}
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(v["host"], v["port"])
	cfg.User = v["username"]
	cfg.Passwd = v["password"]
	cfg.Timeout = externalMysqlTimeout
	// Same tls values the operator publishes in the application's DSN, so the check negotiates like the app.
	cfg.TLSConfig = v["tls"]
	tlsCfg, err := mysqlTLSConfig(v["host"], v["tls"], v["sslCa"], v["sslCert"], v["sslKey"])
	if err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: err.Error()}
	}
	if tlsCfg != nil {
		cfg.TLS = tlsCfg
		cfg.AllowFallbackToPlaintext = v["tls"] == "preferred"
	}

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeUnknown, Message: redact(err.Error(), v["password"])}
	}

	db := sql.OpenDB(connector)
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, externalMysqlTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: fmt.Sprintf("cannot connect to %s: %s", cfg.Addr, redact(err.Error(), v["password"]))}
	}
	return Result{Check: ExternalMysqlCheck, Outcome: OutcomePass}
}

// mysqlTLSConfig returns nil when no certificate material is set, leaving the tls mode to the driver.
func mysqlTLSConfig(host, mode, ca, cert, key string) (*tls.Config, error) {
	if mode == "false" || (ca == "" && cert == "" && key == "") {
		return nil, nil
	}
	cfg := &tls.Config{
		ServerName: host,
		// Only when the user chose skip-verify/preferred, matching the driver's handling of those modes.
		InsecureSkipVerify: mode == "skip-verify" || mode == "preferred",
	}
	if ca != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("sslCa is not valid PEM")
		}
		cfg.RootCAs = pool
	}
	if cert != "" || key != "" {
		pair, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			return nil, fmt.Errorf("sslCert/sslKey: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return cfg, nil
}

// redact guards against a driver ever echoing the password back in an error that lands in status or logs.
func redact(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "***")
}
