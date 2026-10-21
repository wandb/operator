package preflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
)

const externalClickHouseTimeout = 10 * time.Second

func RunExternalClickHouseCheck(ctx context.Context, conn *apiv2.ClickHouseConnection, env Env) Result {
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{
		"url":         conn.URL,
		"host":        conn.Host,
		"httpPort":    conn.HTTPPort,
		"tcpPort":     conn.TCPPort,
		"database":    conn.Database,
		"username":    conn.Username,
		"password":    conn.Password,
		"replicated":  conn.Replicated,
		"clusterName": conn.ClusterName,
	})
	if err != nil {
		return Result{Check: ExternalClickHouseCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v",
			err)}
	}
	fail := func(msg string) Result {
		return Result{Check: ExternalClickHouseCheck, Outcome: OutcomeFail, Message: redact(msg, v["password"])}
	}

	replicated := false
	if s := strings.TrimSpace(v["replicated"]); s != "" {
		if replicated, err = strconv.ParseBool(s); err != nil {
			return fail(fmt.Sprintf("replicated %q must be true or false", s))
		}
	}
	if replicated && v["clusterName"] == "" {
		return fail("clusterName is required when replicated is true")
	}
	base, err := clickHouseHTTPBase(v)
	if err != nil {
		return fail(err.Error())
	}
	if p := v["tcpPort"]; p != "" && !validPort(p) {
		return fail(fmt.Sprintf("tcpPort %q must be an integer between 1 and 65535", p))
	}

	ctx, cancel := context.WithTimeout(ctx, externalClickHouseTimeout)
	defer cancel()
	ch := clickHouseHTTP{base: base, user: v["username"], password: v["password"], database: v["database"]}

	// The database header makes ClickHouse reject a missing database, which the operator never creates.
	version, err := ch.query(ctx, "SELECT version()", nil)
	if err != nil {
		return fail(fmt.Sprintf("query %s: %v", base.Host, err))
	}
	if replicated {
		n, err := ch.query(ctx, "SELECT count() FROM system.clusters WHERE cluster = {cluster:String}",
			map[string]string{"cluster": v["clusterName"]})
		if err != nil {
			return fail(fmt.Sprintf("list clusters: %v", err))
		}
		if n == "0" {
			return fail(fmt.Sprintf("cluster %q is not defined in system.clusters", v["clusterName"]))
		}
		if _, err := ch.query(ctx, "SELECT count() FROM system.zookeeper WHERE path = '/'", nil); err != nil {
			return fail(fmt.Sprintf("replicated tables need Keeper/ZooKeeper, which is not reachable: %v", err))
		}
	}
	if p := v["tcpPort"]; p != "" {
		var d net.Dialer
		c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(base.Hostname(), p))
		if err != nil {
			return fail(fmt.Sprintf("native protocol port: %v", err))
		}
		_ = c.Close()
	}
	return Result{Check: ExternalClickHouseCheck, Outcome: OutcomePass, Message: "clickhouse " + version}
}

// clickHouseHTTPBase prefers an http(s) url, else host + httpPort. There is no TLS field, so a host-based
// connection is probed over plain HTTP.
func clickHouseHTTPBase(v map[string]string) (*url.URL, error) {
	if raw := v["url"]; raw != "" {
		// url.Parse errors echo the input, which can carry credentials.
		u, err := url.Parse(raw)
		if err != nil {
			return nil, errors.New("url is not a valid URL")
		}
		if u.Scheme == "http" || u.Scheme == "https" {
			return &url.URL{Scheme: u.Scheme, Host: u.Host}, nil
		}
	}
	host := strings.TrimSpace(v["host"])
	if host == "" {
		return nil, errors.New("host is required unless url is an http(s) URL")
	}
	if !validPort(v["httpPort"]) {
		return nil, fmt.Errorf("httpPort %q must be an integer between 1 and 65535", v["httpPort"])
	}
	return &url.URL{Scheme: "http", Host: net.JoinHostPort(host, strings.TrimSpace(v["httpPort"]))}, nil
}

type clickHouseHTTP struct {
	base                     *url.URL
	user, password, database string
}

// query runs q over the HTTP interface; params bind to {name:Type} placeholders so values are never spliced into SQL.
func (c clickHouseHTTP) query(ctx context.Context, q string, params map[string]string) (string, error) {
	u := *c.base
	values := url.Values{}
	for k, val := range params {
		values.Set("param_"+k, val)
	}
	u.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(q))
	if err != nil {
		return "", err
	}
	if c.user != "" {
		req.Header.Set("X-ClickHouse-User", c.user)
	}
	if c.password != "" {
		req.Header.Set("X-ClickHouse-Key", c.password)
	}
	if c.database != "" {
		req.Header.Set("X-ClickHouse-Database", c.database)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	text := strings.TrimSpace(string(body))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, text)
	}
	return text, nil
}
