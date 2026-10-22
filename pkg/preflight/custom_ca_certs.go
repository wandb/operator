package preflight

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// RunCustomCACertsCheck parses spec.global.customCACerts, which nothing else validates before it reaches the app's
// trust store. Each entry becomes one .crt file and may hold a bundle.
func RunCustomCACertsCheck(_ context.Context, certs *[]string, _ Env) Result {
	now := time.Now()
	var problems, notes []string
	total := 0
	for i, entry := range *certs {
		n, expired, problem := checkCABundle(entry, now)
		total += n
		if problem != "" {
			problems = append(problems, fmt.Sprintf("customCACerts[%d] %s", i, problem))
		}
		for _, e := range expired {
			notes = append(notes, fmt.Sprintf("customCACerts[%d] %s", i, e))
		}
	}
	if len(problems) > 0 {
		return Result{Check: CustomCACertsCheck, Outcome: OutcomeFail, Message: strings.Join(problems, "; ")}
	}
	msg := fmt.Sprintf("%d certificates", total)
	if len(notes) > 0 {
		msg += "; ignored: " + strings.Join(notes, "; ")
	}
	return Result{Check: CustomCACertsCheck, Outcome: OutcomePass, Message: msg}
}

// knownPEMLabels are standard labels safe to echo. Any other label is user-controlled text that could hold a
// secret, so it's never copied into the result.
var knownPEMLabels = map[string]bool{
	"PRIVATE KEY": true, "RSA PRIVATE KEY": true, "EC PRIVATE KEY": true, "ENCRYPTED PRIVATE KEY": true,
	"PUBLIC KEY": true, "CERTIFICATE REQUEST": true, "X509 CRL": true, "TRUSTED CERTIFICATE": true,
}

// checkCABundle fails an entry with a malformed or non-CA block, or with no currently valid CA. Expired or
// not-yet-valid CAs alongside a valid one are only reported, since bundles commonly carry retired cross-signs.
// Messages never include block contents: a pasted private key must not land in status.
//
// Plain text between blocks is allowed, as OpenSSL, Go and update-ca-certificates all ignore it and real bundles
// carry comments. Every BEGIN marker must decode into a complete block, though: pem.Decode alone silently skips a
// truncated block, or a fragment after the last complete one.
func checkCABundle(data string, now time.Time) (valid int, outOfDate []string, problem string) {
	leading, _, _ := strings.Cut(data, "-----BEGIN")
	if strings.Contains(leading, "-----END") {
		return 0, nil, "has a PEM END marker without a BEGIN"
	}
	segments := pemSegments(data)
	if len(segments) == 0 {
		return 0, nil, "contains no PEM certificate"
	}
	for i, segment := range segments {
		n := i + 1
		// Each segment holds exactly one BEGIN marker, so Decode can't skip ahead to a later block.
		block, rest := pem.Decode([]byte(segment))
		if block == nil || strings.Contains(string(rest), "-----END") {
			return 0, nil, fmt.Sprintf("block %d is malformed or incomplete PEM", n)
		}
		if block.Type != "CERTIFICATE" {
			label := "an unrecognized"
			if knownPEMLabels[block.Type] {
				label = fmt.Sprintf("a %q", block.Type)
			}
			return 0, nil, fmt.Sprintf("block %d is %s block; only CERTIFICATE blocks belong in a CA bundle", n, label)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return 0, nil, fmt.Sprintf("certificate %d does not parse", n)
		}
		// Parsing only proves syntax; a leaf certificate parses too but can't issue the certificates being trusted.
		if !cert.BasicConstraintsValid || !cert.IsCA {
			return 0, nil, fmt.Sprintf("certificate %d (%q) is not a CA certificate", n, cert.Subject.CommonName)
		}
		switch {
		case now.After(cert.NotAfter):
			outOfDate = append(outOfDate, fmt.Sprintf("%q expired on %s", cert.Subject.CommonName, cert.NotAfter.Format(time.DateOnly)))
		case now.Before(cert.NotBefore):
			outOfDate = append(outOfDate, fmt.Sprintf("%q is not valid until %s", cert.Subject.CommonName, cert.NotBefore.Format(time.DateOnly)))
		default:
			valid++
		}
	}
	if valid == 0 {
		return 0, nil, "has no currently valid CA certificate: " + strings.Join(outOfDate, ", ")
	}
	return valid, outOfDate, ""
}

// pemSegments splits data at each BEGIN marker; text before the first marker is dropped.
func pemSegments(data string) []string {
	const begin = "-----BEGIN"
	var segments []string
	for {
		start := strings.Index(data, begin)
		if start < 0 {
			return segments
		}
		data = data[start:]
		next := strings.Index(data[len(begin):], begin)
		if next < 0 {
			return append(segments, data)
		}
		segments = append(segments, data[:len(begin)+next])
		data = data[len(begin)+next:]
	}
}
