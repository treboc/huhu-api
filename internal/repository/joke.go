package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/treboc/huhu-api/internal/model"
)

var (
	ErrJokeNotFound = errors.New("joke not found")
	ErrNoJokes      = errors.New("no jokes available")
)

type JokeRepository interface {
	GetJoke(ctx context.Context, id int64) (*model.Joke, error)
	GetRandomJoke(ctx context.Context) (*model.Joke, error)
	ListJokes(ctx context.Context, offset, limit int) ([]*model.Joke, error)
	CreateJoke(ctx context.Context, joke *model.Joke) (int64, error)
	UpdateJoke(ctx context.Context, joke *model.Joke) error
	DeleteJoke(ctx context.Context, id int64) error
	CountJokes(ctx context.Context) (int, error)
	Close() error
}

type SQLiteJokeRepository struct {
	db *sql.DB
}

func NewSQLiteJokeRepository(dbPath string) (*SQLiteJokeRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jokes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		text TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating jokes table: %w", err)
	}

	return &SQLiteJokeRepository{db: db}, nil
}

func (r *SQLiteJokeRepository) GetJoke(ctx context.Context, id int64) (*model.Joke, error) {
	query := `
		SELECT id, text, created_at, updated_at
		FROM jokes
		WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id)
	joke := &model.Joke{}

	err := row.Scan(&joke.ID, &joke.Text, &joke.CreatedAt, &joke.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("joke not found: %w", err)
		}
		return nil, fmt.Errorf("error getting joke: %w", err)
	}

	return joke, nil
}

func (r *SQLiteJokeRepository) GetRandomJoke(ctx context.Context) (*model.Joke, error) {
	query := `
		SELECT id, text, created_at, updated_at
		FROM jokes
		ORDER BY RANDOM()
		LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query)
	joke := &model.Joke{}

	err := row.Scan(&joke.ID, &joke.Text, &joke.CreatedAt, &joke.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("joke not found: %w", err)
		}
		return nil, fmt.Errorf("error getting joke: %w", err)
	}

	return joke, nil
}

func (r *SQLiteJokeRepository) ListJokes(ctx context.Context, limit, offset int) ([]*model.Joke, error) {
	query := `
		SELECT id, text, created_at, updated_at
		FROM jokes
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listing jokes: %w", err)
	}
	defer rows.Close()

	jokes := make([]*model.Joke, 0)
	for rows.Next() {
		joke := &model.Joke{}
		if err := rows.Scan(&joke.ID, &joke.Text, &joke.CreatedAt, &joke.UpdatedAt); err != nil {
			return nil, fmt.Errorf("error scanning joke: %w", err)
		}
		jokes = append(jokes, joke)
	}

	return jokes, nil
}

func (r *SQLiteJokeRepository) CreateJoke(ctx context.Context, joke *model.Joke) (int64, error) {
	query := `
		INSERT INTO jokes (text, created_at, updated_at)
		VALUES (?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query, joke.Text, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("error creating joke: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting last insert ID: %w", err)
	}

	return id, nil
}

func (r *SQLiteJokeRepository) UpdateJoke(ctx context.Context, joke *model.Joke) error {
	query := `
		UPDATE jokes
		SET text = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now().UTC()
	result, err := r.db.ExecContext(
		ctx,
		query,
		joke.Text,
		now,
		joke.ID,
	)

	if err != nil {
		return fmt.Errorf("error updating joke: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("joke not found")
	}

	return nil
}

func (r *SQLiteJokeRepository) DeleteJoke(ctx context.Context, id int64) error {
	query := `
		DELETE FROM jokes
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting joke: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("joke not found")
	}

	return nil
}

func (r *SQLiteJokeRepository) CountJokes(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM jokes
	`

	row := r.db.QueryRowContext(ctx, query)
	var count int

	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("error counting jokes: %w", err)
	}

	return count, nil
}

func (r *SQLiteJokeRepository) Close() error {
	return r.db.Close()
}
