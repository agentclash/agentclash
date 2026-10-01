//go:build awsplatform

package temporalutil

import (
	"context"
	"os"
	"testing"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func platformWorkflow(ctx workflow.Context) (string, error) {
	if err := workflow.Sleep(ctx, 35*time.Second); err != nil {
		return "", err
	}
	return "recovered", nil
}

// The harness seeds durable timers, replaces the production server with a new
// membership IP/certificate and later restores all three databases into a second
// cluster. Each phase uses the repository's actual TLS loader and pinned SDK.
func TestAWSPlatformTemporal(t *testing.T) {
	if os.Getenv("PLATFORM_TEMPORAL_ADDRESS") == "" {
		t.Skip("isolated harness required")
	}
	t.Setenv("TEMPORAL_TLS_ENABLED", "true")
	t.Setenv("TEMPORAL_API_KEY", "")
	t.Setenv("TEMPORAL_TLS_CA_FILE", os.Getenv("PLATFORM_CA"))
	t.Setenv("TEMPORAL_TLS_SERVER_NAME", "temporal")
	t.Setenv("TEMPORAL_TLS_CERT_FILE", os.Getenv("PLATFORM_CERT"))
	t.Setenv("TEMPORAL_TLS_KEY_FILE", os.Getenv("PLATFORM_KEY"))
	connection, err := LoadConnectionConfigFromEnv("production")
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(os.Getenv("PLATFORM_TEMPORAL_ADDRESS"), "agentclash-prod", connection)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	phase := os.Getenv("PLATFORM_PHASE")
	for _, queue := range []string{"execution", "scoring", "background"} {
		w := worker.New(c, queue, worker.Options{MaxConcurrentWorkflowTaskExecutionSize: 2})
		w.RegisterWorkflow(platformWorkflow)
		if err := w.Start(); err != nil {
			t.Fatal(err)
		}
		defer w.Stop()
		id := "platform-recovery-" + queue
		if phase == "seed" {
			if _, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: id, TaskQueue: queue}, platformWorkflow); err != nil {
				t.Fatal(err)
			}
			// Ensure the workflow timer is durable before the first worker disappears.
			it := c.GetWorkflowHistory(ctx, id, "", true, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			seen := false
			for it.HasNext() {
				event, err := it.Next()
				if err != nil {
					t.Fatal(err)
				}
				if event.GetEventType() == enums.EVENT_TYPE_TIMER_STARTED {
					seen = true
					break
				}
			}
			if !seen {
				t.Fatal("durable timer not observed")
			}
		} else {
			var result string
			if err := c.GetWorkflow(ctx, id, "").Get(ctx, &result); err != nil {
				t.Fatal(err)
			}
			if result != "recovered" {
				t.Fatal("unexpected workflow result")
			}
		}
	}
	if phase != "seed" {
		deadline := time.Now().Add(15 * time.Second)
		for {
			response, err := c.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{Namespace: "agentclash-prod", Query: "ExecutionStatus = 'Completed'"})
			if err == nil && len(response.Executions) >= 3 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("completed workflows missing from PostgreSQL visibility")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	bad := connection
	bad.tlsConfig = connection.tlsConfig.Clone()
	bad.tlsConfig.ServerName = "incorrect.invalid"
	if unexpected, err := NewClient(os.Getenv("PLATFORM_TEMPORAL_ADDRESS"), "agentclash-prod", bad); err == nil {
		unexpected.Close()
		t.Fatal("wrong server name accepted")
	}
	t.Setenv("TEMPORAL_TLS_CA_FILE", os.Getenv("PLATFORM_UNTRUSTED_CA"))
	bad, err = LoadConnectionConfigFromEnv("production")
	if err != nil {
		t.Fatal(err)
	}
	if unexpected, err := NewClient(os.Getenv("PLATFORM_TEMPORAL_ADDRESS"), "agentclash-prod", bad); err == nil {
		unexpected.Close()
		t.Fatal("untrusted server CA accepted")
	}
}
