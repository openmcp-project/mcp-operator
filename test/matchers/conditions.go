package matchers

import (
	"fmt"

	"github.com/onsi/gomega/types"

	corev1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// MatchComponentCondition returns a Gomega matcher that checks if a ComponentCondition is equal to the expected one.
// If the passed in 'actual' is not a ComponentCondition, the matcher will fail.
// All fields which are set to their zero value in the expected condition will be ignored.
func MatchComponentCondition(con corev1alpha1.ComponentCondition) types.GomegaMatcher {
	return &conditionMatcher{expected: con}
}

type conditionMatcher struct {
	expected corev1alpha1.ComponentCondition
}

var _ types.GomegaMatcher = &conditionMatcher{}

// Match implements types.GomegaMatcher.
func (c *conditionMatcher) Match(actualRaw interface{}) (success bool, err error) {
	actual, ok := actualRaw.(corev1alpha1.ComponentCondition)
	if !ok {
		return false, fmt.Errorf("expected actual to be of type ComponentCondition, got %T", actualRaw)
	}
	if c.expected.Type != "" && c.expected.Type != actual.Type {
		return false, nil
	}
	if c.expected.Status != "" && c.expected.Status != actual.Status {
		return false, nil
	}
	if c.expected.Reason != "" && c.expected.Reason != actual.Reason {
		return false, nil
	}
	if c.expected.Message != "" && c.expected.Message != actual.Message {
		return false, nil
	}
	if !c.expected.LastTransitionTime.IsZero() && !c.expected.LastTransitionTime.Equal(&actual.LastTransitionTime) {
		return false, nil
	}
	return true, nil
}

// FailureMessage implements types.GomegaMatcher.
func (c *conditionMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto equal \n\t%#v", actual, c.expected)
}

// NegatedFailureMessage implements types.GomegaMatcher.
func (c *conditionMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto not equal \n\t%#v", actual, c.expected)
}
