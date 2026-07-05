package component

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/common/types"
)

const clawEvalSummaryHTTPTimeout = 15 * time.Second

func fetchClawEvalSummary(ctx context.Context, resultURL string) (*types.ClawEvalSummary, error) {
	resultURL = strings.TrimSpace(resultURL)
	if resultURL == "" {
		return nil, fmt.Errorf("result url is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create summary request: %w", err)
	}

	client := &http.Client{Timeout: clawEvalSummaryHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch claw-eval summary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch claw-eval summary, status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read claw-eval summary: %w", err)
	}

	summary, err := types.ParseClawEvalSummary(raw)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// attachClawEvalSummary fetches batch_summary.json from ResultURL on each successful GET.
// Failures are logged and omitted; consider persisting summary after job completion if latency matters.
func attachClawEvalSummary(ctx context.Context, res *types.EvaluationRes) {
	if res == nil || res.TaskType != types.TaskTypeClawEval || res.ResultURL == "" {
		return
	}
	if res.Status != string(v1alpha1.WorkflowSucceeded) {
		return
	}

	summary, err := fetchClawEvalSummary(ctx, res.ResultURL)
	if err != nil {
		slog.WarnContext(ctx, "failed to fetch claw-eval summary",
			slog.Int64("evaluation_id", res.ID),
			slog.String("result_url", res.ResultURL),
			slog.Any("error", err),
		)
		return
	}
	res.Summary = summary
}
