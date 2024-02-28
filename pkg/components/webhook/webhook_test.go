package webhook

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_renderTemplate(t *testing.T) {
	tests := []struct {
		name   string
		source string
		cfg    webhookConfig
	}{
		{
			name:   "Test 1",
			source: "This is a test for {{.Image}}",
			cfg: webhookConfig{
				Image: "webhook-image",
			},
		},
		{
			name:   "Test 1",
			source: webhookTemplate,
			cfg: webhookConfig{
				Image: "operator-image:latest",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := renderTemplate(test.source, test.cfg)
			assert.NoError(t, err)
			assert.True(t, strings.Contains(string(got), test.cfg.Image), "Expected replacement not found in rendered template: %s", string(got))
		})
	}
}
