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

// checkCABundle fails an entry with no currently valid certificate. Expired or not-yet-valid certificates alongside
// a valid one are only reported, since bundles commonly carry retired cross-signs. Messages never include block
// contents: a pasted private key must not land in status.
func checkCABundle(data string, now time.Time) (valid int, outOfDate []string, problem string) {
	rest := []byte(data)
	found := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		found++
		if block.Type != "CERTIFICATE" {
			return valid, outOfDate, fmt.Sprintf("contains a %q block; only CERTIFICATE blocks belong in a CA bundle", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return valid, outOfDate, fmt.Sprintf("certificate %d does not parse", found)
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
	switch {
	case found == 0:
		return 0, nil, "contains no PEM certificate"
	case valid == 0:
		return 0, nil, "has no currently valid certificate: " + strings.Join(outOfDate, ", ")
	}
	return valid, outOfDate, ""
}
