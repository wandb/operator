package redact

import "io"

type Redactor interface {
	Redact(input io.Reader, path string) io.Reader
}
