package opencode

import (
	"sort"

	"github.com/jachy-h/umbra-gate/agents"
	"github.com/jachy-h/umbra-gate/opencodeconfig"
)

type Manager struct {
	Path string
}

func (m Manager) ID() string {
	return "opencode"
}

func (m Manager) DisplayName() string {
	return "OpenCode"
}

func (m Manager) Discover() ([]agents.ConfigFile, error) {
	if m.Path != "" {
		return convertFiles(opencodeconfig.DiscoverFile(m.Path)), nil
	}
	return convertFiles(opencodeconfig.Discover(opencodeconfig.DefaultDir())), nil
}

func (m Manager) Status(ctx agents.Context) (*agents.Status, error) {
	files, err := m.Discover()
	if err != nil {
		return nil, err
	}
	configPath := selectedPath(files, m.Path)
	manager := opencodeconfig.Manager{Path: configPath}
	cfg, err := manager.MaskedConfig()
	if err != nil {
		return nil, err
	}
	providers, _ := cfg["provider"].(map[string]any)
	ids := providerIDs(ctx.ProviderIDs, providers)
	bindings := make([]agents.BindingStatus, 0, len(ids))
	for _, id := range ids {
		provider, _ := providers[id].(map[string]any)
		options, _ := provider["options"].(map[string]any)
		baseURL := ""
		if options != nil {
			baseURL, _ = options["baseURL"].(string)
		}
		bindings = append(bindings, agents.BindingStatus{
			ProviderID:     id,
			Configured:     provider != nil,
			Active:         false,
			GatewayEnabled: opencodeconfig.GatewayURLMatches(baseURL, ctx.GatewayBaseURL, id),
			GatewayBaseURL: opencodeconfig.GatewayURL(ctx.GatewayBaseURL, id),
			LiveBaseURL:    baseURL,
			ConfigPath:     configPath,
		})
	}
	return &agents.Status{
		AgentID:        m.ID(),
		DisplayName:    m.DisplayName(),
		ConfigFiles:    files,
		Bindings:       bindings,
		GatewayCapable: true,
		FineGrained:    true,
		ProxyMethod:    "Config File (provider.options.baseURL)",
	}, nil
}

func (m Manager) Plan(ctx agents.Context, input agents.BindingInput) (*agents.Plan, error) {
	manager := opencodeconfig.Manager{Path: firstNonEmpty(input.ConfigPath, m.Path)}
	plan, err := manager.Plan(toProviderInput(ctx, input))
	if err != nil {
		return nil, err
	}
	return &agents.Plan{
		BaseChecksum: plan.BaseChecksum,
		Diff:         plan.Diff,
		Current:      plan.Current,
		ProposedText: plan.ProposedText,
	}, nil
}

func (m Manager) Apply(ctx agents.Context, input agents.BindingInput, baseChecksum string) error {
	manager := opencodeconfig.Manager{Path: firstNonEmpty(input.ConfigPath, m.Path)}
	return manager.Apply(toProviderInput(ctx, input), baseChecksum)
}

func toProviderInput(ctx agents.Context, input agents.BindingInput) opencodeconfig.ProviderInput {
	gateway := opencodeconfig.GatewayDisable
	if input.Enabled {
		gateway = opencodeconfig.GatewayEnable
	}
	return opencodeconfig.ProviderInput{
		ID:             input.ProviderID,
		Gateway:        gateway,
		GatewayBaseURL: firstNonEmpty(input.GatewayBaseURL, ctx.GatewayBaseURL),
	}
}

func providerIDs(configIDs []string, providers map[string]any) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, id := range configIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range providers {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func convertFiles(files []opencodeconfig.ConfigFile) []agents.ConfigFile {
	out := make([]agents.ConfigFile, 0, len(files))
	for _, file := range files {
		out = append(out, agents.ConfigFile{
			Path:     file.Path,
			Label:    file.Label,
			Exists:   file.Exists,
			Selected: file.Selected,
		})
	}
	return out
}

func selectedPath(files []agents.ConfigFile, fallback string) string {
	if fallback != "" {
		return fallback
	}
	for _, file := range files {
		if file.Selected {
			return file.Path
		}
	}
	if len(files) > 0 {
		return files[0].Path
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
