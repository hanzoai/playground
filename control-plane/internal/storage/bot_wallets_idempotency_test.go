package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractExecutionIDFromDescription(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		expected string
	}{
		{
			name:     "LLM format with exec prefix",
			desc:     "LLM claude-sonnet-4: 150 input + 300 output tokens (exec abc-123-def)",
			expected: "abc-123-def",
		},
		{
			name:     "Execution format",
			desc:     "Execution xyz-789 (1500ms)",
			expected: "xyz-789",
		},
		{
			name:     "No execution ID",
			desc:     "Manual top-up from admin",
			expected: "",
		},
		{
			name:     "Empty description",
			desc:     "",
			expected: "",
		},
		{
			name:     "UUID execution ID",
			desc:     "LLM gpt-4: 100 input + 200 output tokens (exec e3b0c442-98fc-1c14-b39f-4c9a2e6b2d1a)",
			expected: "e3b0c442-98fc-1c14-b39f-4c9a2e6b2d1a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractExecutionIDFromDescription(tt.desc)
			assert.Equal(t, tt.expected, result)
		})
	}
}
