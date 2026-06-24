package agents

type ConfigFile struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Exists   bool   `json:"exists"`
	Selected bool   `json:"selected"`
}

type Context struct {
	GatewayBaseURL string
	ProviderIDs    []string
}

type Manager interface {
	ID() string
	DisplayName() string
	Discover() ([]ConfigFile, error)
	Status(Context) (*Status, error)
	Plan(Context, BindingInput) (*Plan, error)
	Apply(Context, BindingInput, string) error
}

type Status struct {
	AgentID        string          `json:"agent_id"`
	DisplayName    string          `json:"display_name"`
	ConfigFiles    []ConfigFile    `json:"config_files"`
	Bindings       []BindingStatus `json:"bindings"`
	GatewayCapable bool            `json:"gateway_capable"`
	FineGrained    bool            `json:"fine_grained"`
}

type BindingStatus struct {
	ProviderID     string `json:"provider_id"`
	Configured     bool   `json:"configured"`
	Active         bool   `json:"active"`
	GatewayEnabled bool   `json:"gateway_enabled"`
	GatewayBaseURL string `json:"gateway_base_url"`
	LiveBaseURL    string `json:"live_base_url"`
	ConfigPath     string `json:"config_path"`
	ProjectID      string `json:"project_id,omitempty"`
}

type BindingInput struct {
	ProviderID     string `json:"provider_id"`
	Enabled        bool   `json:"enabled"`
	GatewayBaseURL string `json:"gateway_base_url"`
	ConfigPath     string `json:"config_path"`
	ProjectID      string `json:"project_id,omitempty"`
}

type Plan struct {
	BaseChecksum string `json:"base_checksum"`
	Diff         string `json:"diff"`
	Current      string `json:"current"`
	ProposedText string `json:"proposed_text"`
}
