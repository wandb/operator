package mysql

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMysql(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MySQL Suite")
}
