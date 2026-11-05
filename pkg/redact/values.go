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
		"dsn", "connstr", "connectionstring",
	}
	sensitiveSuffixes = []string{"key", "pwd", "connection"}
	// Address-like keys are masked only when the value contains an @, so a
	// plain host stays readable but user:pass@host does not.
	addressSuffixes = []string{"address", "addr", "url", "uri", "endpoint", "host"}
)

var (
	// Greedy up to the last @ in the token, so an unescaped @ or / in a
	// password is still covered.
	urlUserinfo = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://[^\s"'<>]*@`)
	// password=..., "token": "...", Authorization: Bearer ... in free text
	// such as CLI flags or embedded config files.
	inlineAssignment = regexp.MustCompile(`(?i)((?:password|passwd|secret|token|credential|license|api[_-]?key|access[_-]?key|private[_-]?key|authorization)\w*["']?[ \t]*[:=][ \t]*["']?)(?:(?:bearer|basic)[ \t]+)?[^\s"'&,;]+`)
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
		if sensitive && v != "" {
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
	s = urlUserinfo.ReplaceAllStringFunc(s, maskUserinfo)
	s = inlineAssignment.ReplaceAllString(s, "${1}"+Mask)
	return jwt.ReplaceAllString(s, Mask)
}

func maskUserinfo(match string) string {
	start := strings.Index(match, "://") + len("://")
	userinfo := match[start : len(match)-1]
	if slash := strings.IndexByte(userinfo, '/'); slash >= 0 {
		authority := userinfo[:slash]
		_, port, hasColon := strings.Cut(authority, ":")
		if !strings.Contains(authority, "@") && (!hasColon || isPort(port)) {
			// host[:port]/path@ref: the @ is in the path, e.g. a digest.
			return match
		}
	}
	return match[:start] + Mask + "@"
}

func isPort(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
