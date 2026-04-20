package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// consumeLoop periodically consumes cron result messages from the broker.
// On first call it acks all pre-existing messages to avoid re-delivering
// results from before this process started.
func (s *Scheduler) consumeLoop(ctx context.Context) {
	defer s.wg.Done()

	s.ackAllExisting(ctx)

	ticker := time.NewTicker(consumeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.consumeOnce(ctx)
		}
	}
}

// ackAllExisting consumes and acks all pending messages without notifying.
// This prevents re-delivery of cron results from previous process lifetimes.
// It consumes messages for ALL channels (channelID="") because on restart the
// new sessionID has no prior consume state — any unacked messages from the old
// session would otherwise be re-delivered. In single-instance deployments this
// is always safe; in multi-instance deployments, each instance acks on behalf
// of its own previous session.
func (s *Scheduler) ackAllExisting(ctx context.Context) {
	if s.broker == nil {
		return
	}
	for {
		msgs, err := s.broker.Consume(ctx, cobot.MessageTypeCronResult, "", s.sessionID, 100)
		if err != nil || len(msgs) == 0 {
			return
		}
		// Batch ack all messages in the fetched batch.
		ids := make([]string, 0, len(msgs))
		for _, msg := range msgs {
			ids = append(ids, msg.ID)
		}
		if err := s.broker.AckAll(ctx, ids, s.sessionID); err != nil {
			slog.Warn("batch ack failed", "error", err, "count", len(ids))
		}
	}
}

// consumeOnce consumes unacknowledged cron result messages and delivers them locally.
func (s *Scheduler) consumeOnce(ctx context.Context) {
	if s.broker == nil || s.notifier == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("consumeOnce recovered from panic", "error", r)
		}
	}()
	// sessionID is used as the consume session identity (separate from leader lease holderID).
	msgs, err := s.broker.Consume(ctx, cobot.MessageTypeCronResult, "", s.sessionID, 50)
	if err != nil {
		slog.Warn("failed to consume cron results", "error", err)
		return
	}
	for _, msg := range msgs {
		payload, err := DecodeCronResult(msg)
		if err != nil {
			slog.Warn("failed to decode cron result", "msg_id", msg.ID, "error", err)
			_ = s.broker.Ack(ctx, msg.ID, s.sessionID)
			continue
		}
		if msg.ChannelID == "" {
			_ = s.broker.Ack(ctx, msg.ID, s.sessionID)
			continue
		}
		notifyCtx, cancel := context.WithTimeout(ctx, brokerOpTimeout)
		content := formatCronResult(payload.JobName, payload.Result, payload.Error)
		s.notifier.Notify(notifyCtx, msg.ChannelID, cobot.ChannelMessage{
			Type:    cobot.MessageTypeCronResult,
			Title:   fmt.Sprintf("Cron job %q completed", payload.JobName),
			Content: content,
		})
		cancel()
		if ackErr := s.broker.Ack(ctx, msg.ID, s.sessionID); ackErr != nil {
			slog.Warn("failed to ack cron result", "msg_id", msg.ID, "error", ackErr)
		}
	}
}

// formatCronResult formats a cron job execution result for display.
func formatCronResult(jobName, result, runErr string) string {
	if runErr != "" {
		return fmt.Sprintf("❌ Job %s failed: %s", jobName, runErr)
	}
	return fmt.Sprintf("✅ Job %s result:\n%s", jobName, result)
}

// publishJobResult publishes the job result via the broker so followers can consume it.
func (s *Scheduler) publishJobResult(job *Job, result string, runErr error, duration time.Duration) {
	if s.broker == nil {
		return
	}
	payload := &CronResultPayload{
		JobID:    job.ID,
		JobName:  job.Name,
		Result:   result,
		RunAt:    time.Now(),
		Duration: duration.Milliseconds(),
	}
	if runErr != nil {
		payload.Error = runErr.Error()
	}
	msg, err := NewCronResultMessage(job.ChannelID, payload)
	if err != nil {
		slog.Warn("failed to marshal cron result", "job_id", job.ID, "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), brokerOpTimeout)
	defer cancel()
	if err := s.broker.Publish(ctx, msg); err != nil {
		slog.Warn("failed to publish cron result", "job_id", job.ID, "error", err)
	}
}
