package preflight

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
)

// TestRegistryRelations guards DependsOn and Covers, which fail silently on a typo: an unknown name never matches,
// a cycle never runs, and a covered check on a different CRField never shares a FieldPath with its coverer.
func TestRegistryRelations(t *testing.T) {
	for name, c := range Checks {
		for _, d := range c.DependsOn {
			if d == name {
				t.Errorf("%s depends on itself", name)
			} else if _, ok := Checks[d]; !ok {
				t.Errorf("%s depends on unknown check %q", name, d)
			}
		}
		for _, cv := range c.Covers {
			covered, ok := Checks[cv]
			switch {
			case cv == name:
				t.Errorf("%s covers itself", name)
			case !ok:
				t.Errorf("%s covers unknown check %q", name, cv)
			case covered.CRField != c.CRField:
				t.Errorf("%s covers %s but their CRFields differ (%q vs %q)", name, cv, c.CRField, covered.CRField)
			case slices.Contains(covered.Covers, name):
				t.Errorf("%s and %s cover each other", name, cv)
			}
		}
	}
	if cycle := dependsOnCycle(Checks); cycle != nil {
		t.Errorf("DependsOn cycle: %s", strings.Join(cycle, " -> "))
	}
}

// dependsOnCycle returns one DependsOn cycle in checks, or nil. Unknown names are TestRegistryRelations' job.
func dependsOnCycle(checks map[string]Check) []string {
	const (
		unvisited = iota
		inProgress
		done
	)
	state := map[string]int{}
	var path []string
	var visit func(string) []string
	visit = func(name string) []string {
		switch state[name] {
		case inProgress:
			for i, p := range path {
				if p == name {
					return append(append([]string{}, path[i:]...), name)
				}
			}
		case done:
			return nil
		}
		state[name] = inProgress
		path = append(path, name)
		for _, d := range checks[name].DependsOn {
			if cycle := visit(d); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		state[name] = done
		return nil
	}
	for name := range checks {
		if cycle := visit(name); cycle != nil {
			return cycle
		}
	}
	return nil
}

func TestDependsOnCycleDetection(t *testing.T) {
	checks := map[string]Check{
		"a": {DependsOn: []string{"b"}},
		"b": {DependsOn: []string{"c"}},
		"c": {DependsOn: []string{"a"}},
	}
	if dependsOnCycle(checks) == nil {
		t.Fatal("dependsOnCycle missed a -> b -> c -> a")
	}
	delete(checks, "c")
	if cycle := dependsOnCycle(checks); cycle != nil {
		t.Fatalf("dependsOnCycle reported %v for an acyclic graph", cycle)
	}
}

func TestRegistryRelationEntries(t *testing.T) {
	if got, want := Checks[ClusterCapacityCheck].DependsOn, []string{ServerManifestCheck}; !reflect.DeepEqual(got, want) {
		t.Errorf("%s.DependsOn = %v, want %v", ClusterCapacityCheck, got, want)
	}
	if got, want := Checks[ExternalMysqlInClusterCheck].Covers, []string{ExternalMysqlCheck}; !reflect.DeepEqual(got, want) {
		t.Errorf("%s.Covers = %v, want %v", ExternalMysqlInClusterCheck, got, want)
	}
}

// Both helpers must copy before appending: a base Check whose slice has spare capacity would otherwise have two
// registry entries built from it share, and overwrite, one backing array.
func TestRelationHelpersDoNotAlias(t *testing.T) {
	helpers := []struct {
		name  string
		apply func(Check, ...string) Check
		get   func(Check) []string
		set   func(*Check, []string)
	}{
		{"withDependsOn", withDependsOn, func(c Check) []string { return c.DependsOn }, func(c *Check, s []string) { c.DependsOn = s }},
		{"withCovers", withCovers, func(c Check) []string { return c.Covers }, func(c *Check, s []string) { c.Covers = s }},
	}
	for _, h := range helpers {
		t.Run(h.name, func(t *testing.T) {
			var base Check
			spare := make([]string, 1, 4)
			spare[0] = "base"
			h.set(&base, spare)

			first := h.apply(base, "first")
			second := h.apply(base, "second")

			if got, want := h.get(first), []string{"base", "first"}; !reflect.DeepEqual(got, want) {
				t.Errorf("first = %v, want %v", got, want)
			}
			if got, want := h.get(second), []string{"base", "second"}; !reflect.DeepEqual(got, want) {
				t.Errorf("second = %v, want %v", got, want)
			}
			if got, want := h.get(base), []string{"base"}; !reflect.DeepEqual(got, want) {
				t.Errorf("base = %v, want %v", got, want)
			}
		})
	}
}

// closedPort returns a local port with nothing listening, so a dial is refused immediately.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	_ = l.Close()
	return port
}

// serve accepts connections on a local port and hands each to handle until the test ends.
func serve(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				handle(conn)
			}()
		}
	}()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func TestRunExternalMysqlCheck_FailureKind(t *testing.T) {
	// A MySQL server that rejects the client with ERR 1045 in place of its handshake, as it does for a blocked host.
	rejecting := serve(t, func(conn net.Conn) {
		payload := []byte{0xff}
		payload = binary.LittleEndian.AppendUint16(payload, 1045)
		payload = append(payload, "#28000Access denied for user 'wandb'"...)
		header := []byte{byte(len(payload)), byte(len(payload) >> 8), byte(len(payload) >> 16), 0}
		_, _ = conn.Write(append(header, payload...))
	})

	tests := []struct {
		name string
		port string
		want FailureKind
	}{
		{name: "refused dial is unreachable", port: closedPort(t), want: FailureUnreachable},
		{name: "server error packet is rejected", port: rejecting, want: FailureRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &apiv2.MysqlConnection{
				Host:     apiv2.ValueOrSecret{Value: "127.0.0.1"},
				Port:     apiv2.ValueOrSecret{Value: tt.port},
				Username: apiv2.ValueOrSecret{Value: "wandb"},
				Password: apiv2.ValueOrSecret{Value: "secret"},
			}
			result := RunExternalMysqlCheck(context.Background(), conn, Env{Resolve: literalResolver})
			if result.Outcome != OutcomeFail {
				t.Fatalf("Outcome = %q (%s), want %q", result.Outcome, result.Message, OutcomeFail)
			}
			if result.Failure != tt.want {
				t.Errorf("Failure = %q, want %q (message: %s)", result.Failure, tt.want, result.Message)
			}
			if strings.Contains(result.Message, "secret") {
				t.Errorf("Message leaks the password: %s", result.Message)
			}
		})
	}
}

// fakeRedis answers each RESP command with reply(command name), so a test can script the server's responses.
func fakeRedis(t *testing.T, reply func(cmd string) string) string {
	return serve(t, func(conn net.Conn) {
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
			if !strings.HasPrefix(line, "*") || err != nil {
				return
			}
			var args []string
			for i := 0; i < n; i++ {
				if _, err := r.ReadString('\n'); err != nil { // $<len>
					return
				}
				arg, err := r.ReadString('\n')
				if err != nil {
					return
				}
				args = append(args, strings.TrimSpace(arg))
			}
			if len(args) == 0 {
				return
			}
			if _, err := conn.Write([]byte(reply(strings.ToUpper(args[0])))); err != nil {
				return
			}
		}
	})
}

func redisConn(port, password string) *apiv2.RedisConnection {
	return &apiv2.RedisConnection{
		Host:     apiv2.ValueOrSecret{Value: "127.0.0.1"},
		Port:     apiv2.ValueOrSecret{Value: port},
		Password: apiv2.ValueOrSecret{Value: password},
	}
}

func TestRunExternalRedisCheck_FailureKind(t *testing.T) {
	rejecting := fakeRedis(t, func(string) string {
		return "-WRONGPASS invalid username-password pair or user is disabled.\r\n"
	})

	tests := []struct {
		name string
		port string
		want FailureKind
	}{
		{name: "refused dial is unreachable", port: closedPort(t), want: FailureUnreachable},
		{name: "error reply is rejected", port: rejecting, want: FailureRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RunExternalRedisCheck(context.Background(), redisConn(tt.port, "secret"), Env{Resolve: literalResolver})
			if result.Outcome != OutcomeFail {
				t.Fatalf("Outcome = %q (%s), want %q", result.Outcome, result.Message, OutcomeFail)
			}
			if result.Failure != tt.want {
				t.Errorf("Failure = %q, want %q (message: %s)", result.Failure, tt.want, result.Message)
			}
			if strings.Contains(result.Message, "secret") {
				t.Errorf("Message leaks the password: %s", result.Message)
			}
		})
	}
}

func TestRunExternalRedisCheck_Pass(t *testing.T) {
	const version = "7.2.4"
	info := fmt.Sprintf("# Server\r\nredis_version:%s\r\n", version)
	tests := []struct {
		name        string
		infoReply   string
		wantMessage string
	}{
		{name: "reports the version", infoReply: fmt.Sprintf("$%d\r\n%s\r\n", len(info), info), wantMessage: "redis " + version},
		{name: "INFO disabled still passes", infoReply: "-ERR unknown command 'INFO'\r\n", wantMessage: "connected; server version unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fakeRedis(t, func(cmd string) string {
				switch cmd {
				case "PING":
					return "+PONG\r\n"
				case "INFO":
					return tt.infoReply
				default: // HELLO: answer like a RESP2-only server so the client falls back.
					return "-ERR unknown command '" + cmd + "'\r\n"
				}
			})
			result := RunExternalRedisCheck(context.Background(), redisConn(port, ""), Env{Resolve: literalResolver})
			if result.Outcome != OutcomePass {
				t.Fatalf("Outcome = %q (%s), want %q", result.Outcome, result.Message, OutcomePass)
			}
			if result.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", result.Message, tt.wantMessage)
			}
			if result.Failure != "" {
				t.Errorf("Failure = %q on a pass, want empty", result.Failure)
			}
		})
	}
}
