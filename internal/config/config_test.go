package config

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Config", func() {
	var originalEnv map[string]string

	ginkgo.BeforeEach(func() {
		// Save original environment
		originalEnv = make(map[string]string)
		for _, key := range []string{"NAMESPACE_REGEX", "DRY_RUN", "DEBUG", "TRACE"} {
			if val, exists := os.LookupEnv(key); exists {
				originalEnv[key] = val
			}
			os.Unsetenv(key)
		}
	})

	ginkgo.AfterEach(func() {
		// Restore original environment
		for _, key := range []string{"NAMESPACE_REGEX", "DRY_RUN", "DEBUG", "TRACE"} {
			os.Unsetenv(key)
			if val, exists := originalEnv[key]; exists {
				os.Setenv(key, val)
			}
		}
	})

	ginkgo.Describe("NewConfig", func() {
		ginkgo.It("should create default configuration", func() {
			config := &OperatorConfig{}

			gomega.Expect(config.NamespaceRegex).To(gomega.BeEmpty())
			gomega.Expect(config.DryRun).To(gomega.BeFalse())
			gomega.Expect(config.Debug).To(gomega.BeFalse())
			gomega.Expect(config.Trace).To(gomega.BeFalse())
		})

		ginkgo.It("should parse namespace regex from environment", func() {
			os.Setenv("NAMESPACE_REGEX", "^kube-.*,^default$")

			config := &OperatorConfig{
				NamespaceRegex: []string{"^kube-.*", "^default$"},
			}

			gomega.Expect(config.NamespaceRegex).To(gomega.Equal([]string{"^kube-.*", "^default$"}))
		})

		ginkgo.It("should set dry run from environment", func() {
			os.Setenv("DRY_RUN", "true")

			config := &OperatorConfig{
				DryRun: true,
			}

			gomega.Expect(config.DryRun).To(gomega.BeTrue())
		})

		ginkgo.It("should set debug from environment", func() {
			os.Setenv("DEBUG", "true")

			config := &OperatorConfig{
				Debug: true,
			}

			gomega.Expect(config.Debug).To(gomega.BeTrue())
		})

		ginkgo.It("should set trace from environment and enable debug", func() {
			os.Setenv("TRACE", "true")

			config := &OperatorConfig{
				Trace: true,
				Debug: true,
			}

			gomega.Expect(config.Trace).To(gomega.BeTrue())
			gomega.Expect(config.Debug).To(gomega.BeTrue()) // Trace implies debug
		})
	})

	ginkgo.Describe("LogLevel", func() {
		ginkgo.It("should return normal level by default", func() {
			config := &OperatorConfig{}
			gomega.Expect(config.LogLevel()).To(gomega.Equal(0))
		})

		ginkgo.It("should return verbose level for debug", func() {
			config := &OperatorConfig{Debug: true}
			gomega.Expect(config.LogLevel()).To(gomega.Equal(-1))
		})

		ginkgo.It("should return very verbose level for trace", func() {
			config := &OperatorConfig{Trace: true}
			gomega.Expect(config.LogLevel()).To(gomega.Equal(-2))
		})

		ginkgo.It("should prioritize trace over debug", func() {
			config := &OperatorConfig{Debug: true, Trace: true}
			gomega.Expect(config.LogLevel()).To(gomega.Equal(-2))
		})
	})
})

func TestConfig(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Config Suite")
}
