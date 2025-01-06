package buildkite

import (
    "encoding/json"
    "reflect"
    "testing"
    "time"
)

func TestTransform(t *testing.T) {
    createdAt := time.Now()
    startedAt := createdAt.Add(5 * time.Second)
    finishedAt := startedAt.Add(30 * time.Second)

    input := Payload{
        Event: "build.finished",
        Build: Build{
            ID:         "019439b6-95f9-4326-81fb-25ac99289820",
            GraphQLID:  "QnVpbGQtLS0wMTk0MzliNi05NWY5LTQzMjYtODFmYi0yNWFjOTkyODk4MjA=",
            URL:        "https://api.buildkite.com/v2/organizations/testkite/pipelines/basic-pipeline/builds/697",
            WebURL:     "https://buildkite.com/testkite/basic-pipeline/builds/697",
            Number:     697,
            State:      "failed",
            Message:    "",
            Commit:     "HEAD",
            Branch:     "main",
            Source:     "ui",
            CreatedAt:  createdAt,
            StartedAt:  startedAt,
            FinishedAt: finishedAt,
            Creator: User{
                ID:        "01831b25-7d66-431e-8dcf-6d7ff40c5255",
                Name:      "Test User",
                Email:     "test@example.com",
                AvatarURL: "https://example.com/avatar",
            },
        },
        Pipeline: Pipeline{
            ID:          "0189b873-e493-4675-b964-a085ddc4b927",
            GraphQLID:   "UGlwZWxpbmUtLS0wMTg5Yjg3My1lNDkzLTQ2NzUtYjk2NC1hMDg1ZGRjNGI5Mjc=",
            URL:         "https://api.buildkite.com/v2/organizations/testkite/pipelines/basic-pipeline",
            WebURL:      "https://buildkite.com/testkite/basic-pipeline",
            Name:        "Basic Pipeline",
            Description: "Has no special config just standard steps.",
            Slug:        "basic-pipeline",
            Repository:  "git@github.com:mcncl/pipeline_basic.git",
            Provider: Provider{
                ID:       "github",
                Settings: map[string]interface{}{},
            },
        },
        Sender: User{
            ID:   "01831b25-7d66-431e-8dcf-6d7ff40c5255",
            Name: "Test User",
        },
    }

    want := TransformedPayload{
        EventType: "build.finished",
        Build: BuildInfo{
            ID:           "019439b6-95f9-4326-81fb-25ac99289820",
            URL:          "https://api.buildkite.com/v2/organizations/testkite/pipelines/basic-pipeline/builds/697",
            WebURL:       "https://buildkite.com/testkite/basic-pipeline/builds/697",
            Number:       697,
            State:        "failed",
            Branch:       "main",
            Commit:       "HEAD",
            CreatedAt:    createdAt,
            StartedAt:    startedAt,
            FinishedAt:   finishedAt,
            Pipeline:     "basic-pipeline",
            Organization: "testkite",
        },
        Pipeline: PipelineInfo{
            ID:          "0189b873-e493-4675-b964-a085ddc4b927",
            Name:        "Basic Pipeline",
            Description: "Has no special config just standard steps.",
            Repository:  "git@github.com:mcncl/pipeline_basic.git",
        },
        Sender: User{
            ID:   "01831b25-7d66-431e-8dcf-6d7ff40c5255",
            Name: "Test User",
        },
    }

    got, err := Transform(input)
    if err != nil {
        t.Fatalf("Transform() error = %v", err)
    }

    // Store Raw field for later comparison
    rawField := got.Raw

    // Compare everything except Raw field
    got.Raw = nil
    if !reflect.DeepEqual(got, want) {
        t.Errorf("Transform() = %v, want %v", got, want)
    }

    // Get expected Raw field
    inputJSON, err := json.Marshal(input)
    if err != nil {
        t.Fatalf("Failed to marshal input: %v", err)
    }
    
    var expectedRaw map[string]interface{}
    if err := json.Unmarshal(inputJSON, &expectedRaw); err != nil {
        t.Fatalf("Failed to unmarshal expected raw: %v", err)
    }

    // Compare Raw fields
    if !reflect.DeepEqual(rawField, expectedRaw) {
        t.Errorf("Transform() Raw field mismatch:\ngot  = %v\nwant = %v", rawField, expectedRaw)
    }
}
