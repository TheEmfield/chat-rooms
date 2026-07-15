package entity

import "time"

type Room struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Capacity  int       `db:"capacity" json:"capacity"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Message struct {
	ID        int64     `db:"id" json:"id"`
	RoomID    string    `db:"room_id" json:"room_id"`
	SenderIP  string    `db:"sender_ip" json:"sender_ip"`
	Msg       string    `db:"msg" json:"msg"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
