package buildkite

import (
	"encoding/json"
	"strings"
)

func Transform(payload Payload) (TransformedPayload, error) {
	// Extract organization from pipeline URL
	// URL format: https://api.buildkite.com/v2/organizations/ORGNAME/pipelines/...
	orgName := ""
	urlParts := strings.Split(payload.Pipeline.URL, "/")
	for i, part := range urlParts {
		if part == "organizations" && i+1 < len(urlParts) {
			orgName = urlParts[i+1]
			break
		}
	}

	transformed := TransformedPayload{
		EventType: payload.Event,
		Build: BuildInfo{
			ID:           payload.Build.ID,
			URL:          payload.Build.URL,
			WebURL:       payload.Build.WebURL,
			Number:       payload.Build.Number,
			State:        payload.Build.State,
			Branch:       payload.Build.Branch,
			Commit:       payload.Build.Commit,
			CreatedAt:    payload.Build.CreatedAt,
			StartedAt:    payload.Build.StartedAt,
			FinishedAt:   payload.Build.FinishedAt,
			Pipeline:     payload.Pipeline.Slug,
			Organization: orgName,
		},
		Pipeline: PipelineInfo{
			ID:          payload.Pipeline.ID,
			Name:        payload.Pipeline.Name,
			Description: payload.Pipeline.Description,
			Repository:  payload.Pipeline.Repository,
		},
		Sender: payload.Sender,
	}

	// Convert payload to map for raw storage
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		return TransformedPayload{}, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return TransformedPayload{}, err
	}

	transformed.Raw = raw
	return transformed, nil
}
