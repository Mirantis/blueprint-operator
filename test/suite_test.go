package test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestBoundless runs end to end tests on boundless operator
func TestBoundless(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boundless Operator End-to-End tests")
}
