package apphome

import (
	"testing"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapability_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cap     Capability
		wantErr bool
	}{
		{
			name: "valid capability",
			cap: Capability{
				ID:             "test_cap",
				Name:           "Test Capability",
				Icon:           ":test:",
				Description:    "A test capability",
				Category:       "test",
				PromptTemplate: "Test prompt: {{.input}}",
				Enabled:        true,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			cap: Capability{
				Name:           "Test",
				PromptTemplate: "Test",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			cap: Capability{
				ID:             "test",
				PromptTemplate: "Test",
			},
			wantErr: true,
		},
		{
			name: "missing prompt template",
			cap: Capability{
				ID:   "test",
				Name: "Test",
			},
			wantErr: true,
		},
		{
			name: "invalid parameter type",
			cap: Capability{
				ID:             "test",
				Name:           "Test",
				PromptTemplate: "Test",
				Parameters: []Parameter{
					{ID: "p1", Label: "P1", Type: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "select without options",
			cap: Capability{
				ID:             "test",
				Name:           "Test",
				PromptTemplate: "Test",
				Parameters: []Parameter{
					{ID: "p1", Label: "P1", Type: "select"},
				},
			},
			wantErr: true,
		},
		{
			name: "select with options",
			cap: Capability{
				ID:             "test",
				Name:           "Test",
				PromptTemplate: "Test: {{.p1}}",
				Parameters: []Parameter{
					{ID: "p1", Label: "P1", Type: "select", Options: []string{"a", "b"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cap.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegistry_LoadFromBytes(t *testing.T) {
	yamlData := `
capabilities:
  - id: test_cap
    name: Test Capability
    icon: ":test:"
    description: A test
    category: test
    enabled: true
    prompt_template: "Test: {{.input}}"
    parameters:
      - id: input
        label: Input
        type: text
        required: true
  - id: disabled_cap
    name: Disabled
    icon: ":x:"
    description: Disabled
    category: test
    enabled: false
    prompt_template: "Disabled"
`

	registry := NewRegistry()
	err := registry.LoadFromBytes([]byte(yamlData))
	require.NoError(t, err)

	// Should have 1 capability (disabled one skipped)
	assert.Equal(t, 1, registry.Count())

	cap, ok := registry.Get("test_cap")
	require.True(t, ok)
	assert.Equal(t, "Test Capability", cap.Name)
	assert.Len(t, cap.Parameters, 1)
}

func TestRegistry_GetByCategory(t *testing.T) {
	registry := NewRegistry()

	// Register test capabilities
	require.NoError(t, registry.Register(Capability{
		ID:             "code1",
		Name:           "Code 1",
		Category:       "code",
		PromptTemplate: "Test",
	}))
	require.NoError(t, registry.Register(Capability{
		ID:             "code2",
		Name:           "Code 2",
		Category:       "code",
		PromptTemplate: "Test",
	}))
	require.NoError(t, registry.Register(Capability{
		ID:             "debug1",
		Name:           "Debug 1",
		Category:       "debug",
		PromptTemplate: "Test",
	}))

	codeCaps := registry.GetByCategory("code")
	assert.Len(t, codeCaps, 2)

	debugCaps := registry.GetByCategory("debug")
	assert.Len(t, debugCaps, 1)

	gitCaps := registry.GetByCategory("git")
	assert.Len(t, gitCaps, 0)
}

func TestFormBuilder_BuildModal(t *testing.T) {
	fb := NewFormBuilder()
	cap := Capability{
		ID:             "test",
		Name:           "Test Capability",
		Description:    "Test description",
		PromptTemplate: "Test: {{.input}}",
		Parameters: []Parameter{
			{
				ID:          "input",
				Label:       "Input",
				Type:        "text",
				Required:    true,
				Placeholder: "Enter input",
			},
			{
				ID:          "select_field",
				Label:       "Select",
				Type:        "select",
				Options:     []string{"a", "b", "c"},
				Placeholder: "Choose",
			},
		},
	}

	modal := fb.BuildModal(cap)
	require.NotNil(t, modal)
	assert.Equal(t, slack.VTModal, modal.Type)
	assert.Equal(t, "test", modal.PrivateMetadata)
	assert.NotEmpty(t, modal.Blocks.BlockSet)
}

func TestFormBuilder_ValidateParams(t *testing.T) {
	fb := NewFormBuilder()
	cap := Capability{
		ID:             "test",
		Name:           "Test",
		PromptTemplate: "Test",
		Parameters: []Parameter{
			{ID: "required_field", Label: "Required", Type: "text", Required: true},
			{ID: "optional_field", Label: "Optional", Type: "text", Required: false},
		},
	}

	tests := []struct {
		name   string
		params map[string]string
		errors int
	}{
		{
			name:   "all required provided",
			params: map[string]string{"required_field": "value"},
			errors: 0,
		},
		{
			name:   "missing required",
			params: map[string]string{},
			errors: 1,
		},
		{
			name:   "all provided",
			params: map[string]string{"required_field": "value", "optional_field": "opt"},
			errors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := fb.ValidateParams(cap, tt.params)
			assert.Len(t, errors, tt.errors)
		})
	}
}

func TestBuilder_BuildFullHomeView(t *testing.T) {
	registry := NewRegistry()
	require.NoError(t, registry.Register(Capability{
		ID:             "code1",
		Name:           "Code Review",
		Icon:           ":mag:",
		Description:    "Review code",
		Category:       "code",
		PromptTemplate: "Review: {{.code}}",
	}))

	builder := NewBuilder(registry)
	view := builder.BuildFullHomeView()

	require.NotNil(t, view)
	assert.Equal(t, slack.VTHomeTab, view.Type)
	assert.NotEmpty(t, view.Blocks.BlockSet)
}
