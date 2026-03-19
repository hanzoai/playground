package orchestrator

// Preset defines a harness configuration template for an agent.
type Preset struct {
	Name           string   `json:"name"`
	DisplayName    string   `json:"display_name"`
	Model          string   `json:"model"`
	ApprovalPolicy string   `json:"approval_policy"` // "never", "on-failure", "untrusted", "on-request"
	SandboxMode    string   `json:"sandbox_mode"`    // "danger-full-access", "workspace-write", "read-only"
	Personality    string   `json:"personality"`      // system prompt
	Emoji          string   `json:"emoji"`
	Color          string   `json:"color"`
	Capabilities   []string `json:"capabilities"`
}

// builtinPresets is the static preset map, built once.
var builtinPresets = map[string]Preset{
	"cto": {
		Name:           "cto",
		DisplayName:    "CTO",
		Model:          "opus",
		ApprovalPolicy: "never",
		SandboxMode:    "danger-full-access",
		Personality:    "Technical leader. Architectural decisions. Ship it.",
		Emoji:          "👔",
		Color:          "#6366f1",
		Capabilities:   []string{"code", "review", "deploy", "architecture"},
	},
	"senior": {
		Name:           "senior",
		DisplayName:    "Senior Engineer",
		Model:          "sonnet",
		ApprovalPolicy: "untrusted",
		SandboxMode:    "workspace-write",
		Personality:    "Production engineer. Clean, tested code.",
		Emoji:          "🔧",
		Color:          "#22c55e",
		Capabilities:   []string{"code", "review", "test", "network"},
	},
	"junior": {
		Name:           "junior",
		DisplayName:    "Junior Engineer",
		Model:          "haiku",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "workspace-write",
		Personality:    "Learning. Ask before destructive actions.",
		Emoji:          "🌱",
		Color:          "#eab308",
		Capabilities:   []string{"code", "test"},
	},
	"intern": {
		Name:           "intern",
		DisplayName:    "Intern",
		Model:          "haiku",
		ApprovalPolicy: "on-request",
		SandboxMode:    "read-only",
		Personality:    "Observer. Suggest improvements. Never modify without approval.",
		Emoji:          "📚",
		Color:          "#94a3b8",
		Capabilities:   []string{"review"},
	},
	"vision": {
		Name:           "vision",
		DisplayName:    "Product Visionary",
		Model:          "opus",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "read-only",
		Personality:    "Product visionary. Strategy and roadmap.",
		Emoji:          "🔭",
		Color:          "#a855f7",
		Capabilities:   []string{"review", "network"},
	},
	"marketing": {
		Name:           "marketing",
		DisplayName:    "Marketing Engineer",
		Model:          "sonnet",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "workspace-write",
		Personality:    "Growth engineer. Content, campaigns, analytics.",
		Emoji:          "📈",
		Color:          "#ec4899",
		Capabilities:   []string{"code", "network"},
	},
	"sales": {
		Name:           "sales",
		DisplayName:    "Sales Engineer",
		Model:          "sonnet",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "read-only",
		Personality:    "Revenue. Pipeline, outreach, deal analysis.",
		Emoji:          "💰",
		Color:          "#f97316",
		Capabilities:   []string{"review", "network"},
	},
	"design": {
		Name:           "design",
		DisplayName:    "Designer",
		Model:          "sonnet",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "workspace-write",
		Personality:    "UI/UX designer. Figma-to-code. Design systems.",
		Emoji:          "🎨",
		Color:          "#06b6d4",
		Capabilities:   []string{"code", "review"},
	},
	"devops": {
		Name:           "devops",
		DisplayName:    "DevOps Engineer",
		Model:          "sonnet",
		ApprovalPolicy: "untrusted",
		SandboxMode:    "danger-full-access",
		Personality:    "Infrastructure. CI/CD, K8s, monitoring, deploys.",
		Emoji:          "🚀",
		Color:          "#14b8a6",
		Capabilities:   []string{"code", "deploy", "network"},
	},
	"security": {
		Name:           "security",
		DisplayName:    "Security Engineer",
		Model:          "opus",
		ApprovalPolicy: "on-failure",
		SandboxMode:    "read-only",
		Personality:    "Red team. Audit, pentest, vulnerability analysis.",
		Emoji:          "🛡️",
		Color:          "#ef4444",
		Capabilities:   []string{"review", "network"},
	},
}

// BuiltinPresets returns a copy of the hardcoded preset map.
func BuiltinPresets() map[string]Preset {
	out := make(map[string]Preset, len(builtinPresets))
	for k, v := range builtinPresets {
		out[k] = v
	}
	return out
}

// GetPreset returns a preset by name, falling back to "junior" for unknown names.
func GetPreset(name string) Preset {
	if p, ok := builtinPresets[name]; ok {
		return p
	}
	return builtinPresets["junior"]
}

// ApplyPreset merges a preset with explicit overrides from SpawnOpts.
// Explicit fields in opts take precedence over preset defaults.
func ApplyPreset(presetName string, opts SpawnOpts) SpawnOpts {
	p := GetPreset(presetName)

	if opts.Model == "" {
		opts.Model = p.Model
	}
	if opts.ApprovalPolicy == "" {
		opts.ApprovalPolicy = p.ApprovalPolicy
	}
	if opts.SandboxMode == "" {
		opts.SandboxMode = p.SandboxMode
	}
	if opts.Personality == "" {
		opts.Personality = p.Personality
	}
	if opts.Emoji == "" {
		opts.Emoji = p.Emoji
	}
	if opts.Color == "" {
		opts.Color = p.Color
	}
	if len(opts.Capabilities) == 0 {
		opts.Capabilities = make([]string, len(p.Capabilities))
		copy(opts.Capabilities, p.Capabilities)
	}
	if opts.Name == "" {
		opts.Name = p.DisplayName
	}

	return opts
}
