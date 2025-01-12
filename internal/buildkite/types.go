package buildkite

import "time"

// Payload represents the incoming webhook payload from Buildkite
type Payload struct {
	Event    string   `json:"event"`
	Build    Build    `json:"build"`
	Pipeline Pipeline `json:"pipeline"`
	Sender   User     `json:"sender"`
}

type Build struct {
	ID          string                 `json:"id"`
	GraphQLID   string                 `json:"graphql_id"`
	URL         string                 `json:"url"`
	WebURL      string                 `json:"web_url"`
	Number      int                    `json:"number"`
	State       string                 `json:"state"`
	Message     string                 `json:"message"`
	Commit      string                 `json:"commit"`
	Branch      string                 `json:"branch"`
	Tag         *string                `json:"tag"`
	Source      string                 `json:"source"`
	Creator     User                   `json:"creator"`
	CreatedAt   time.Time              `json:"created_at"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	StartedAt   time.Time              `json:"started_at"`
	FinishedAt  time.Time              `json:"finished_at"`
	MetaData    map[string]interface{} `json:"meta_data"`
	ClusterID   string                 `json:"cluster_id"`
}

type Pipeline struct {
	ID          string    `json:"id"`
	GraphQLID   string    `json:"graphql_id"`
	URL         string    `json:"url"`
	WebURL      string    `json:"web_url"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Slug        string    `json:"slug"`
	Repository  string    `json:"repository"`
	Provider    Provider  `json:"provider"`
	CreatedAt   time.Time `json:"created_at"`
}

type Provider struct {
	ID       string                 `json:"id"`
	Settings map[string]interface{} `json:"settings"`
}

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// TransformedPayload represents our standardized message format
type TransformedPayload struct {
	EventType string                 `json:"event_type"`
	Build     BuildInfo              `json:"build"`
	Pipeline  PipelineInfo           `json:"pipeline"`
	Sender    User                   `json:"sender"`
	Raw       map[string]interface{} `json:"raw_payload"`
}

type BuildInfo struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	WebURL       string    `json:"web_url"`
	Number       int       `json:"number"`
	State        string    `json:"state"`
	Branch       string    `json:"branch"`
	Commit       string    `json:"commit"`
	CreatedAt    time.Time `json:"created_at"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	Pipeline     string    `json:"pipeline"`
	Organization string    `json:"organization"`
}

type PipelineInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Repository  string `json:"repository"`
}
