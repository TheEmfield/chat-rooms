package postgres

import (
	"context"
	"fmt"

	"github.com/TheEmfield/chat-rooms/backend/repository/entity"
)

func (p *Postgres) UpsertMessage(ctx context.Context, msg entity.Message) error {
	query := `
		INSERT INTO messages (room_id, sender_ip, msg)
		VALUES (:room_id, :sender_ip, :msg)
	`
	_, err := p.db.NamedExecContext(ctx, query, msg)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (p *Postgres) GetHistory(ctx context.Context, roomID string, limit int) ([]*entity.Message, error) {
	query := `
		SELECT id, room_id, sender_ip, msg, created_at
		FROM messages
		WHERE room_id = $1
		ORDER BY id DESC
		LIMIT $2
	`

	var messages []*entity.Message
	err := p.db.SelectContext(ctx, &messages, query, roomID, limit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}
