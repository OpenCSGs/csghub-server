package sender

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/config"

	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/common/types"
)

// lokiClient implements the LogSender interface
type lokiClient struct {
	clientID          types.ClientType
	acceptLabelPrefix string
	lokiClient        loki.Client
	timeLoc           *time.Location
	lineSeparator     string
}

// NewLokiClient creates a new Loki client
func NewLokiClient(url string, clientID types.ClientType, config *config.Config) (LogSender, error) {
	lc, err := loki.NewClient(url)
	if err != nil {
		slog.Error("failed to create loki client", slog.Any("error", err))
	}
	timeLoc, err := time.LoadLocation(config.Database.TimeZone)
	if err != nil {
		slog.Error("failed to create loki client by TimeZone error", slog.Any("error", err))
	}
	return &lokiClient{
		clientID:          clientID,
		acceptLabelPrefix: config.LogCollector.AcceptLabelPrefix,
		lokiClient:        lc,
		timeLoc:           timeLoc,
		lineSeparator:     config.LogCollector.LineSeparator,
	}, err
}

// logEntryToMap Create a unique key for each stream based on labels
// Priority: entry.PodInfo.Labels > entry.Labels > default labels
func (c *lokiClient) logEntryToMap(entry *types.LogEntry) map[string]string {
	labels := map[string]string{
		"client_id":             c.clientID.String(),
		"trace_id":              entry.TraceID,
		"stage":                 string(entry.Stage),
		"step":                  string(entry.Step),
		"category":              entry.Category.String(),
		types.StreamKeyDeployID: entry.DeployID,
	}

	// Add custom labels; if they exist, they will override the default labels
	for k, v := range entry.Labels {
		labels[k] = v
	}

	if entry.PodInfo != nil {
		labels["pod_name"] = entry.PodInfo.PodName
		labels["pod_uid"] = entry.PodInfo.PodUID
		labels["namespace"] = entry.PodInfo.Namespace
		labels["service_name"] = entry.PodInfo.ServiceName
		labels["container_name"] = entry.PodInfo.ContainerName
		for key, value := range entry.PodInfo.Labels {
			if strings.HasPrefix(key, c.acceptLabelPrefix) && value != "" {
				labels[key] = value
			}
		}
	}

	// filter  labels with empty values
	for k, v := range labels {
		if v == "" {
			delete(labels, k)
		}
	}
	return labels
}

// SendLogs sends log entries to Loki
func (c *lokiClient) SendLogs(ctx context.Context, entries []types.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	// Group entries by labels to create streams
	streamMap := make(map[string]*loki.LokiStream)

	for _, entry := range entries {
		// Create stream labels
		labels := c.logEntryToMap(&entry)
		// Create stream key
		streamKey := c.createStreamKey(labels)

		// Get or create stream to merge  logs with the same stream key
		stream, exists := streamMap[streamKey]
		if !exists {
			stream = &loki.LokiStream{
				Stream: labels,
				Values: make([][]string, 0),
			}
			streamMap[streamKey] = stream
		}

		// Add log entry to stream
		timestamp := strconv.FormatInt(entry.Timestamp.UnixNano(), 10)
		stream.Values = append(stream.Values, []string{timestamp, entry.Message})
	}

	// Convert map to slice
	streams := make([]loki.LokiStream, 0, len(streamMap))
	for _, stream := range streamMap {
		streams = append(streams, *stream)
	}

	// Create push request
	pushRequest := &loki.LokiPushRequest{
		Streams: streams,
	}

	err := c.lokiClient.Push(ctx, pushRequest)
	if err != nil {
		return fmt.Errorf("failed to push logs to loki: %w", err)
	}

	podName := ""
	if len(entries) > 0 && entries[0].PodInfo != nil {
		podName = entries[0].PodInfo.PodName
	}
	slog.Debug("Successfully sent logs to Loki",
		slog.Int("entries_count", len(entries)),
		slog.Int("streams_count", len(streams)),
		slog.String("pod_name", podName),
		slog.String("client_id", c.clientID.String()))

	return nil
}

// GetLastReportedTimestamp queries Loki for the last timestamp for this client
func (c *lokiClient) GetLastReportedTimestamp(ctx context.Context) (time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if c.clientID == "" {
		return time.Time{}, fmt.Errorf("no client ID provided") // No client ID, nothing to query
	}

	query := fmt.Sprintf(`{client_id="%s"}`, c.clientID)
	// Search over the last 30 days. Adjust if logs can be older.
	start := time.Now().Add(-30 * 24 * time.Hour)

	queryRangeParams := loki.QueryRangeParams{
		Query:     query,
		Limit:     1,
		Start:     start,
		Direction: "backward",
	}
	queryResponse, err := c.lokiClient.QueryRange(ctx, queryRangeParams)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to query loki: %w", err)
	}

	if queryResponse.Data.ResultType == "streams" && len(queryResponse.Data.Result) > 0 {
		streams := queryResponse.Data.Result
		if len(streams) > 0 && len(streams[0].Values) > 0 {
			// The first value is the latest one because of direction=backward
			lastTimestampStr := streams[0].Values[0][0]
			lastTimestampNano, err := strconv.ParseInt(lastTimestampStr, 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("failed to parse timestamp from Loki: %w", err)
			}
			// Add one nanosecond to avoid fetching the same log entry again
			lastTime := time.Unix(0, lastTimestampNano).Add(time.Nanosecond)
			slog.Info("Found last reported timestamp from Loki", "client_id", c.clientID, "timestamp", lastTime)
			return lastTime, nil
		}
	}

	slog.Info("No previous logs found for this client_id, will start from the beginning.", "client_id", c.clientID)
	return time.Time{}, nil // No logs found
}

// Health checks Loki health
func (c *lokiClient) Health(ctx context.Context) error {
	return c.lokiClient.Ready(ctx)
}

// createStreamKey creates a unique key for a stream based on labels
func (c *lokiClient) createStreamKey(labels map[string]string) string {
	// Create a deterministic key from labels
	var keyBuilder strings.Builder
	for k, v := range labels {
		if len(v) == 0 {
			continue
		}
		keyBuilder.WriteString(k)
		keyBuilder.WriteString("=")
		keyBuilder.WriteString(v)
		keyBuilder.WriteString(",")
	}
	return keyBuilder.String()
}

func (c *lokiClient) formatPodIdentifier(streamMap map[string]string) string {
	category := types.LogCategrory(streamMap["category"])
	if category == types.LogCategoryPlatform {
		return types.LogCategoryPlatform.String()
	}
	podName := streamMap["pod_name"]
	if podName == "" {
		return types.LogCategoryContainer.String()
	}
	parts := strings.Split(podName, "-")
	var podIdentifier string
	if len(parts) > 2 {
		podIdentifier = strings.Join(parts[len(parts)-2:], "-")
	} else {
		podIdentifier = podName
	}
	return podIdentifier
}

func (c *lokiClient) formatLokiLog(lokiLog *loki.LokiPushRequest, timeLoc *time.Location) string {
	if nil == timeLoc {
		timeLoc = c.timeLoc
	}
	var bulkLog strings.Builder
	for _, stream := range lokiLog.Streams {
		podIdentifier := c.formatPodIdentifier(stream.Stream)
		for _, valuePair := range stream.Values {
			loglist := strings.Split(valuePair[1], "\n")
			for _, log := range loglist {
				if log == "" {
					continue
				}

				lineParts := strings.SplitN(log, " ", 2)
				if len(lineParts) < 2 {
					formattedLog := fmt.Sprintf("%s | %s", podIdentifier, log)
					bulkLog.WriteString(formattedLog)
					bulkLog.WriteString(c.lineSeparator)
					continue
				}

				var formattedTime string
				t, err := time.Parse(time.RFC3339Nano, lineParts[0])
				if err != nil {
					// if parse failed, use original timestamp
					formattedTime = lineParts[0]
				} else {
					formattedTime = t.In(timeLoc).Format(time.DateOnly)
				}

				formattedLog := fmt.Sprintf("%s | %s %s", podIdentifier, formattedTime, lineParts[1])
				bulkLog.WriteString(formattedLog)
				bulkLog.WriteString(c.lineSeparator)
			}
		}
	}
	return strings.TrimSuffix(bulkLog.String(), c.lineSeparator)
}

func (c *lokiClient) StreamAllLogs(
	ctx context.Context,
	id string,
	start time.Time,
	lables map[string]string,
	timeLoc *time.Location) (chan string, error) {
	lables[types.StreamKeyDeployID] = id

	var queryBuilder strings.Builder
	queryBuilder.WriteString("{")
	first := true
	for k, v := range lables {
		if !first {
			queryBuilder.WriteString(",")
		}
		fmt.Fprintf(&queryBuilder, `%s="%s"`, k, v)
		first = false
	}
	queryBuilder.WriteString("}")
	query := queryBuilder.String()

	lokiCh, err := c.lokiClient.Tail(ctx, query, start)
	if err != nil {
		return nil, fmt.Errorf("failed to tail logs from loki: %w", err)
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				slog.Info("StreamAllLogs context canceled, closing connection")
				return
			case lokiLog, ok := <-lokiCh:
				if !ok {
					return
				}
				formattedLogs := c.formatLokiLog(lokiLog, timeLoc)
				if formattedLogs != "" {
					select {
					case ch <- formattedLogs:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return ch, nil
}

func (c *lokiClient) QueryRange(ctx context.Context, params loki.QueryRangeParams) (*loki.LokiQueryResponse, error) {
	return c.lokiClient.QueryRange(ctx, params)
}
