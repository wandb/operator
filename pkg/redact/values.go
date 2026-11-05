package redact

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

// Mask replaces a credential in log output.
const Mask = "***"

// Key names are compared lowercased with separators removed, so these match
// password, MYSQL_PASSWORD and passwordSecret alike. Everything under a
// matching key is masked.
var (
	sensitiveSubstrings = []string{
		"password", "passwd", "secret", "token", "credential", "license",
		"apikey", "accesskey", "privatekey", "authorization",
		"dsn", "connstr", "connectionstring", "passphrase",
	}
	sensitiveSuffixes = []string{"key", "pwd", "connection"}
	// Address-like keys are masked only when the value contains an @, so a
	// plain host stays readable but user:pass@host does not.
	addressSuffixes = []string{"address", "addr", "url", "uri", "endpoint", "host", "store"}
)

var (
	urlScheme = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://`)
	// An OCI digest after a path, as in oci://host/charts/wandb@sha256:<hex>.
	digestRef = regexp.MustCompile(`^[a-z0-9]+(?:[+._-][a-z0-9]+)*:[0-9a-fA-F]{32,}`)
	// password=..., "token": "...", Authorization: Bearer ... in free text
	// such as CLI flags or embedded config files. assignmentValue finds where
	// the value ends.
	inlineAssignment = regexp.MustCompile(`(?i)(?:password|passwd|secret|token|credential|license|api[_-]?key|access[_-]?key|private[_-]?key|authorization)\w*["']?[ \t]*([:=])[ \t]*`)
	authScheme       = regexp.MustCompile(`(?i)^(?:bearer|basic)[ \t]+`)
	jwt              = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
)

// Values returns a JSON-normalized deep copy of v that is safe to log. It
// descends maps and lists, masks values under sensitive keys and the value of
// {name, value} env entries with a sensitive name, and applies String to every
// other string. It fails closed: anything it cannot normalize becomes Mask.
func Values(v interface{}) interface{} {
	data, err := json.Marshal(v)
	if err != nil {
		return Mask
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized interface{}
	if err := decoder.Decode(&normalized); err != nil {
		return Mask
	}
	return mask(normalized, false)
}

func mask(v interface{}, sensitive bool) interface{} {
	switch v := v.(type) {
	case map[string]interface{}:
		envName, _ := v["name"].(string)
		if envName == "" {
			envName, _ = v["key"].(string)
		}
		out := make(map[string]interface{}, len(v))
		for key, child := range v {
			childSensitive := sensitive || isSensitive(key, child) ||
				(key == "value" && envName != "" && isSensitive(envName, child))
			out[key] = mask(child, childSensitive)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, child := range v {
			out[i] = mask(child, sensitive)
		}
		return out
	case string:
		if sensitive && v != "" || hasURLCredential(v) {
			return Mask
		}
		return String(v)
	case json.Number:
		if sensitive {
			return Mask
		}
		return v
	case bool, nil:
		return v
	default:
		return Mask
	}
}

func isSensitive(key string, value interface{}) bool {
	k := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, key)
	for _, s := range sensitiveSubstrings {
		if strings.Contains(k, s) {
			return true
		}
	}
	for _, s := range sensitiveSuffixes {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	// WF_CLICKHOUSE_PASS, SMTP_PASS; bypass is a flag, not a password.
	if strings.HasSuffix(k, "pass") && !strings.HasSuffix(k, "bypass") {
		return true
	}
	if s, ok := value.(string); ok && strings.Contains(s, "@") {
		for _, suffix := range addressSuffixes {
			if strings.HasSuffix(k, suffix) {
				return true
			}
		}
	}
	return false
}

// String masks credentials embedded in free text: URL userinfo, inline
// assignments such as password=..., JWTs, and PEM private keys.
func String(s string) string {
	if strings.Contains(s, "PRIVATE KEY-----") {
		return Mask
	}
	// Assignments go first: password=p@ss would otherwise look like userinfo.
	s = maskAssignments(s)
	s = maskUserinfo(s)
	return jwt.ReplaceAllString(s, Mask)
}

// URL masks the credentials in a value that is a single URL, such as a chart
// repository or OCI reference, from its scheme:// to its last @. Unlike
// String it does not stop at whitespace, so a password containing a space is
// still covered.
func URL(u string) string {
	m := urlScheme.FindStringIndex(u)
	if m == nil {
		return String(u)
	}
	rest := u[m[1]:]
	at := credentialAt(rest)
	if at < 0 {
		return String(u)
	}
	// The rest can still hold credentials, such as ?token=.
	return u[:m[1]] + Mask + String(rest[at:])
}

// maskUserinfo masks, in each word that contains a scheme://, everything from
// the first scheme:// to the last @. Passwords in these values are not
// percent-encoded and can contain any character, so the span is not parsed:
// every userinfo in the word is inside it. An @ that starts a digest is
// skipped, so the span ends at the @ before it, if any. A quote ends the word
// only when the same quote opened the URL.
func maskUserinfo(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		m := urlScheme.FindStringIndex(s[i:])
		if m == nil {
			break
		}
		schemeAt, start := i+m[0], i+m[1]
		var quote byte
		if schemeAt > 0 && (s[schemeAt-1] == '"' || s[schemeAt-1] == '\'') {
			quote = s[schemeAt-1]
		}
		end := len(s)
		if stop := urlEnd(s[start:], quote); stop >= 0 {
			end = start + stop
		}
		rest := s[start:end]
		at := credentialAt(rest)
		out.WriteString(s[i:start])
		if at >= 0 {
			out.WriteString(Mask)
			out.WriteString(rest[at:])
		} else {
			out.WriteString(rest)
		}
		i = end
	}
	out.WriteString(s[i:])
	return out.String()
}

// urlEnd returns the index where a URL ends in s. A quoted URL ends only at
// its unescaped closing quote, because Go and Kubernetes errors quote values
// whose passwords can contain spaces. Otherwise whitespace or an angle
// bracket ends it.
func urlEnd(s string, quote byte) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote && (i == 0 || s[i-1] != '\\') {
				return i
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '<' || c == '>' {
			return i
		}
	}
	return -1
}

// credentialAt returns the index of the @ that ends the userinfo in rest, the
// text after a scheme://, or -1. An @ that starts a digest after a path is
// skipped, so the userinfo ends at the @ before it, if any.
func credentialAt(rest string) int {
	at := strings.LastIndexByte(rest, '@')
	if at >= 0 && strings.Contains(rest[:at], "/") && digestRef.MatchString(rest[at+1:]) {
		at = strings.LastIndexByte(rest[:at], '@')
	}
	return at
}

// hasURLCredential reports whether a spec value holds URL credentials
// anywhere after its first scheme://. Such values are masked whole: their
// passwords are not percent-encoded and can contain any character.
func hasURLCredential(s string) bool {
	m := urlScheme.FindStringIndex(s)
	return m != nil && credentialAt(s[m[1]:]) >= 0
}

func maskAssignments(s string) string {
	var out strings.Builder
	last := 0
	for _, m := range inlineAssignment.FindAllStringSubmatchIndex(s, -1) {
		if m[1] < last {
			// The match is inside a value that is already masked.
			continue
		}
		from, to := assignmentValue(s[m[1]:], s[m[2]])
		if from == to {
			continue
		}
		out.WriteString(s[last : m[1]+from])
		out.WriteString(Mask)
		last = m[1] + to
	}
	out.WriteString(s[last:])
	return out.String()
}

// assignmentValue returns the span of the value at the start of v. A quoted
// value runs to its closing quote, or to the end of v if there is none. An
// unquoted value runs to the end of the line after a colon, as in YAML, and to
// whitespace or a quote after =, as in flags and query strings.
func assignmentValue(v string, sep byte) (int, int) {
	if v == "" {
		return 0, 0
	}
	if q := v[0]; q == '"' || q == '\'' {
		for i := 1; i < len(v); i++ {
			switch {
			case q == '"' && v[i] == '\\':
				i++
			case v[i] == q && q == '\'' && i+1 < len(v) && v[i+1] == q:
				// YAML escapes a single quote by doubling it.
				i++
			case v[i] == q:
				return 1, i
			}
		}
		return 1, len(v)
	}
	if sep == ':' {
		if eol := strings.IndexAny(v, "\r\n"); eol >= 0 {
			return 0, eol
		}
		return 0, len(v)
	}
	skip := 0
	if m := authScheme.FindStringIndex(v); m != nil {
		skip = m[1]
	}
	if end := strings.IndexFunc(v[skip:], func(r rune) bool {
		return unicode.IsSpace(r) || r == '"' || r == '\''
	}); end >= 0 {
		return 0, skip + end
	}
	return 0, len(v)
}

// Error returns err with String applied to its message, for logging errors
// that may quote a spec value such as a chart URL.
func Error(err error) error {
	if err == nil {
		return nil
	}
	return redactedError{err}
}

type redactedError struct{ err error }

func (e redactedError) Error() string { return String(e.err.Error()) }

func (e redactedError) Unwrap() error { return e.err }
