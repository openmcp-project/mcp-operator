package utils

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPrefixWithNamespace(t *testing.T) {
	tests := []struct {
		description string
		inputNS     string
		inputName   string
		expected    string
	}{
		{
			description: "automatically assumes 'default' as namespace if no namespace is given",
			inputName:   "test",
			expected:    "default--test",
		},
		{
			description: "should not modify the namespace if 'default' is given",
			inputNS:     "default",
			inputName:   "test",
			expected:    "default--test",
		},
		{
			description: "should work with namespaces other than 'default'",
			inputNS:     "test",
			inputName:   "test",
			expected:    "test--test",
		},
		{
			description: "should shorten long control-plane-names",
			inputNS:     "my-control-plane-ns",
			inputName:   "my-way-too-long-control-plane-name-0123456789",
			expected:    "my-control-plane-ns--my-way-too-long-control-plane-na--536b070d",
		},
		{
			description: "should work with long namespace names #1",
			inputNS:     strings.Repeat("a", 253), // max length of kubernetes namespace names
			inputName:   "test1",
			expected:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa--20950d95",
		},
		{
			description: "should work with long namespace names #2",
			inputNS:     strings.Repeat("a", 253), // max length of kubernetes namespace names
			inputName:   "test2",
			expected:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa--1d9508dc",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			uut := PrefixWithNamespace(test.inputNS, test.inputName)

			assert.LessOrEqual(t, len(uut), 63)
			assert.Equal(t, test.expected, uut)
		})
	}
}

func TestShorten(t *testing.T) {
	tests := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "SHORTEN string which is longer than 63 characters",
			input:       strings.Repeat("a", maxLength+1),
			expected:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa--d96f0f85",
		},
		{
			description: "NOP for empty string",
		},
		{
			description: "NOP if string length smaller than or equal to 63",
			input:       strings.Repeat("a", maxLength),
			expected:    strings.Repeat("a", maxLength),
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			uut := shorten(test.input, maxLength)

			assert.LessOrEqual(t, len(uut), 63)
			assert.Equal(t, test.expected, uut)
		})
	}
}

var _ = Describe("Utils", func() {
	Context("K8sNameHash", func() {
		It("should return a valid k8s resource name", func() {
			result1 := K8sNameHash("test", "name")
			Expect(result1).To(MatchRegexp("^[a-z2-7]+$"))

			result2 := K8sNameHash("test", "name")
			Expect(result2).To(Equal(result1))
		})
	})

	Context("ScopeToControlPlane", func() {
		It("should return a valid k8s resource name", func() {
			meta := &metav1.ObjectMeta{
				Namespace: "test-namespace",
				Name:      "test-name",
			}
			result := ScopeToControlPlane(meta, "additional", "ids")
			Expect(result).To(MatchRegexp("^[a-z2-7]+$"))
		})
	})

	Context("IsNil", func() {
		It("should return true for nil values", func() {
			Expect(IsNil(map[string]string(nil))).To(BeTrue())
			Expect(IsNil(nil)).To(BeTrue())
			Expect(IsNil((*int)(nil))).To(BeTrue())
			Expect(IsNil([]string(nil))).To(BeTrue())
			Expect(IsNil((chan int)(nil))).To(BeTrue())
			var c client.Client
			Expect(IsNil(c)).To(BeTrue())
			var s *struct{}
			Expect(IsNil(s)).To(BeTrue())
			Expect(IsNil((*client.Client)(nil))).To(BeTrue())
		})

		It("should return false for non-nil values", func() {
			nonNilMap := map[string]string{"key": "value"}
			Expect(IsNil(nonNilMap)).To(BeFalse())
			nonNilSlice := []string{"value"}
			Expect(IsNil(nonNilSlice)).To(BeFalse())
			nonNilPointer := new(int)
			Expect(IsNil(nonNilPointer)).To(BeFalse())
			nonNilStruct := struct{}{}
			Expect(IsNil(nonNilStruct)).To(BeFalse())
			nonNilChannel := make(chan int)
			Expect(IsNil(nonNilChannel)).To(BeFalse())
		})
	})

	Context("InitializeControllerLogger", func() {
		It("should panic if context doesn't contain a logger", func() {
			Expect(func() { InitializeControllerLogger(context.Background(), "test") }).To(Panic())
		})

		It("should return a logger and context containing it", func() {
			log, err := logging.GetLogger()
			Expect(err).ToNot(HaveOccurred())
			ctx := logging.NewContext(context.Background(), log)
			_, ctxWithLogger := InitializeControllerLogger(ctx, "test")
			Expect(logging.FromContext(ctxWithLogger)).ToNot(BeNil())
		})
	})
})

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Test Suite")
}
