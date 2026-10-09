package preflight

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	ParamHost     = "host"
	ParamPort     = "port"
	ParamUsername = "username"
	ParamPassword = "password"

	defaultMysqlPort  = "3306"
	externalDBTimeout = 10 * time.Second
)

func RunExternalDBCheck(ctx context.Context, params map[string]string) Result {
	host := params[ParamHost]
	if host == "" {
		return Result{Check: ExternalDBCheck, Outcome: OutcomeFail, Message: "host is required"}
	}
	port := params[ParamPort]
	if port == "" {
		port = defaultMysqlPort
	}

	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.User = params[ParamUsername]
	cfg.Passwd = params[ParamPassword]
	cfg.Timeout = externalDBTimeout

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return Result{Check: ExternalDBCheck, Outcome: OutcomeUnknown, Message: err.Error()}
	}

	db := sql.OpenDB(connector)
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, externalDBTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return Result{Check: ExternalDBCheck, Outcome: OutcomeFail, Message: fmt.Sprintf("cannot connect to %s: %v", cfg.Addr, err)}
	}
	return Result{Check: ExternalDBCheck, Outcome: OutcomePass}
}
