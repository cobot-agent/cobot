package cron

import (
	"encoding/json"
	"time"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/cobot-agent/cobot/pkg/broker"
	"github.com/google/uuid"
)

// CronResultPayload is the message payload for cron task execution results.
type CronResultPayload struct {
	JobID    string    `json:"job_id"`
	JobName  string    `json:"job_name"`
	Result   string    `json:"result"`
	Error    string    `json:"error,omitempty"`
	RunAt    time.Time `json:"run_at"`
	Duration int64     `json:"duration_ms"`
}

// NewCronResultMessage builds a cron result message.
func NewCronResultMessage(channelID string, payload *CronResultPayload) (*broker.Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &broker.Message{
		ID:        uuid.NewString(),
		Topic:     cobot.MessageTypeCronResult,
		ChannelID: channelID,
		Payload:   data,
		CreatedAt: time.Now(),
	}, nil
}

// DecodeCronResult decodes Message.Payload into a CronResultPayload.
func DecodeCronResult(msg *broker.Message) (*CronResultPayload, error) {
	var p CronResultPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
