package broker

import (
	"encoding/json"
	"time"

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
func NewCronResultMessage(channelID string, payload *CronResultPayload) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		ID:        uuid.NewString(),
		Topic:     "cron_result",
		ChannelID: channelID,
		Payload:   data,
		CreatedAt: time.Now(),
	}, nil
}

// DecodeCronResult decodes Message.Payload into a CronResultPayload.
func DecodeCronResult(msg *Message) (*CronResultPayload, error) {
	var p CronResultPayload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
