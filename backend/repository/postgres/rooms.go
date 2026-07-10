package postgres

import (
	"context"
	"fmt"

	"github.com/TheEmfield/chat-rooms/backend/repository/entity"
)

func (p *Postgres) UpsertRoom(ctx context.Context, room entity.Room) error {
	query := `
		INSERT INTO rooms (id, name, capacity)
		VALUES (:id, :name, :capacity)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
		    capacity = EXCLUDED.capacity
	`
	_, err := p.db.NamedExecContext(ctx, query, room)
	if err != nil {
		return fmt.Errorf("upsert room: %w", err)
	}
	return nil
}

func (p *Postgres) GetRoom(ctx context.Context, id string) (*entity.Room, error) {
	query := `SELECT id, name, capacity FROM rooms WHERE id = $1`

	room := &entity.Room{}
	err := p.db.GetContext(ctx, room, query, id)
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}
	return room, nil
}

func (p *Postgres) GetAllRooms(ctx context.Context) ([]*entity.Room, error) {
	query := `SELECT id, name, capacity FROM rooms ORDER BY id`

	var rooms []*entity.Room
	err := p.db.SelectContext(ctx, &rooms, query)
	if err != nil {
		return nil, fmt.Errorf("get all rooms: %w", err)
	}
	return rooms, nil
}
