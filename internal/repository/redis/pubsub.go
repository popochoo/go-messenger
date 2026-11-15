package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const eventsChannel = "messenger:events"

type BroadcastEvent struct {
	TargetUserIDs []int64         `json:"target_user_ids"`
	Payload       json.RawMessage `json:"payload"`
}

type EventBroker struct {
	rdb *redis.Client
}

func NewEventBroker(rdb *redis.Client) *EventBroker {
	return &EventBroker{rdb: rdb}
}

func (b *EventBroker) CheckRateLimit(ctx context.Context, userID int64, maxMsgs int64, window time.Duration) (bool, error) {
	key := fmt.Sprintf("rate:msg:%d", userID)
	count, err := b.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		b.rdb.Expire(ctx, key, window)
	}
	return count <= maxMsgs, nil
}

func (b *EventBroker) PublishEvent(ctx context.Context, targetUserIDs []int64, payload []byte) error {
	event := BroadcastEvent{
		TargetUserIDs: targetUserIDs,
		Payload:       payload,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal broadcast event: %w", err)
	}

	return b.rdb.Publish(ctx, eventsChannel, data).Err()
}

func (b *EventBroker) SubcribeEvents(ctx context.Context, handler func(BroadcastEvent)) error {
	sub := b.rdb.Subscribe(ctx, eventsChannel)
	defer sub.Close()

	if _, err := sub.Receive(ctx); err != nil {
		return fmt.Errorf("redis subcribe receive: %w", err)
	}

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:

			slog.Info("received event from redis", "payload", msg.Payload)

			if !ok {
				return nil
			}

			var event BroadcastEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
				handler(event)
			}
		}
	}
}
