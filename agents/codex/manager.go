package codex

import (
	"github.com/jachy-h/umbra-gate/agents"
	"github.com/jachy-h/umbra-gate/codexconfig"
)

type Manager struct {
	Path string
}

func (m Manager) ID() string {
	return "codex"
}

func (m Manager) DisplayName() string {
	return "Codex CLI"
}

func (m Manager) Discover() ([]agents.ConfigFile, error) {
	return convertFiles(codexconfig.Discover()), nil
}

func (m Manager) Status(ctx agents.Context) (*agents.Status, error) {
	manager := codexconfig.Manager{Path: m.Path}
	statuses, err := manager.Statuses(providerIDs(ctx.ProviderIDs), ctx.GatewayBaseURL)
	if err != nil {
		return nil, err
	}
	files, err := m.Discover()
	if err != nil {
		return nil, err
	}
	bindings := make([]agents.BindingStatus, 0, len(statuses))
	for _, status := range statuses {
		bindings = append(bindings, agents.BindingStatus{
			ProviderID:     status.ID,
			Configured:     status.Configured,
			Active:         status.Active,
			GatewayEnabled: status.GatewayEnabled,
			GatewayBaseURL: codexconfig.GatewayURL(ctx.GatewayBaseURL, status.ID),
			LiveBaseURL:    status.BaseURL,
			ConfigPath:     selectedPath(files, m.Path),
		})
	}
	return &agents.Status{
		AgentID:        m.ID(),
		DisplayName:    m.DisplayName(),
		ConfigFiles:    files,
		Bindings:       bindings,
		GatewayCapable: true,
		FineGrained:    false,
	}, nil
}

func (m Manager) Plan(ctx agents.Context, input agents.BindingInput) (*agents.Plan, error) {
	manager := codexconfig.Manager{Path: firstNonEmpty(input.ConfigPath, m.Path)}
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
	manager := codexconfig.Manager{Path: firstNonEmpty(input.ConfigPath, m.Path)}
	return manager.Apply(toProviderInput(ctx, input), baseChecksum)
}

func toProviderInput(ctx agents.Context, input agents.BindingInput) codexconfig.ProviderInput {
	gateway := codexconfig.GatewayDisable
	if input.Enabled {
		gateway = codexconfig.GatewayEnable
	}
	return codexconfig.ProviderInput{
		ID:             firstNonEmpty(input.ProviderID, codexconfig.DefaultProviderID),
		Gateway:        gateway,
		GatewayBaseURL: firstNonEmpty(input.GatewayBaseURL, ctx.GatewayBaseURL),
	}
}

func providerIDs(ids []string) []string {
	return []string{codexconfig.DefaultProviderID}
}

func convertFiles(files []codexconfig.ConfigFile) []agents.ConfigFile {
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
