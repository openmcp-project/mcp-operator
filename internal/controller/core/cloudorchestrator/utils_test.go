package cloudorchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyMapEntries(t *testing.T) {
	tests := []struct {
		description       string
		sourceMap         map[string]string
		targetMap         map[string]string
		copyKeys          []string
		expectedTargetMap map[string]string
	}{
		{
			description:       "doesn't run into error if the source or target map is nil",
			sourceMap:         nil,
			targetMap:         nil,
			copyKeys:          []string{"a"},
			expectedTargetMap: nil,
		},
		{
			description: "doesn't modify target map if no keys are given",
			sourceMap: map[string]string{
				"d": "d",
			},
			targetMap: map[string]string{
				"a": "a",
				"b": "b",
				"c": "c",
			},
			expectedTargetMap: map[string]string{
				"a": "a",
				"b": "b",
				"c": "c",
			},
		},
		{
			description: "copies map entries successfully into another map",
			sourceMap: map[string]string{
				"d": "d",
			},
			targetMap: map[string]string{
				"a": "a",
				"b": "b",
				"c": "c",
			},
			copyKeys: []string{"d"},
			expectedTargetMap: map[string]string{
				"a": "a",
				"b": "b",
				"c": "c",
				"d": "d",
			},
		},
		{
			description: "overwrites map entries if already available",
			sourceMap: map[string]string{
				"d": "b",
			},
			targetMap: map[string]string{
				"d": "d",
			},
			copyKeys: []string{"d"},
			expectedTargetMap: map[string]string{
				"d": "b",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			copyMapEntries(test.targetMap, test.sourceMap, test.copyKeys...)

			assert.Equal(t, test.expectedTargetMap, test.targetMap)
		})
	}
}
