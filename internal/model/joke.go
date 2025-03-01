package model

import "time"

type Joke struct {
	ID        int64     `json:"id"`
	Text      string    `json:"joke"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
