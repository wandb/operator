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
	defaultMysqlPort     = "3306"
	externalMysqlTimeout = 10 * time.Second
)

func RunExternalMysqlCheck(ctx context.Context, params map[string]string) Result {
	host := params[ParamHost]
	if host == "" {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: "host is required"}
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
	cfg.Timeout = externalMysqlTimeout

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeUnknown, Message: err.Error()}
	}

	db := sql.OpenDB(connector)
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, externalMysqlTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return Result{Check: ExternalMysqlCheck, Outcome: OutcomeFail, Message: fmt.Sprintf("cannot connect to %s: %v", cfg.Addr, err)}
	}
	return Result{Check: ExternalMysqlCheck, Outcome: OutcomePass}
}
