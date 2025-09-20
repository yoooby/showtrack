package db

import (
	"database/sql"
	"log"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3" // <-- needed for SQLite driver
	"github.com/yoooby/showtrack/internal/model"
)

type DB struct {
	Conn *sql.DB
}

func InitDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// create tables
	_, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS episodes (
            id TEXT PRIMARY KEY,
            show_title TEXT,
            season INTEGER,
            episode INTEGER,
            file_path TEXT
        )
    `)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS progress (
            show_title TEXT PRIMARY KEY,
            last_watched_season INTEGER,
            last_watched_episode INTEGER,
            progress INTEGER
        )
    `)
	if err != nil {
		return nil, err
	}

	return &DB{Conn: conn}, nil

}

func (db *DB) SaveEpisdoes(eps []model.Episode) error {
	tx, err := db.Conn.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
        INSERT INTO episodes (id, show_title, season, episode, file_path)
        VALUES (?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            file_path=excluded.file_path
    `)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, ep := range eps {
		_, err := stmt.Exec(ep.Id, ep.Title, ep.Season, ep.Episode, ep.Path)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) SaveProgress(show string, season, episode, progress int) error {
	_, err := db.Conn.Exec(`
        INSERT INTO progress (show_title, last_watched_season, last_watched_episode, progress)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(show_title) DO UPDATE SET
            last_watched_season = excluded.last_watched_season,
            last_watched_episode = excluded.last_watched_episode,
            progress = excluded.progress
    `, show, season, episode, progress)

	return err
}

func (db *DB) GetNextEpisodes(title string, season int, episode int, count int) []*model.Episode {
	rows, err := db.Conn.Query(`
        SELECT id, show_title, season, episode, file_path
        FROM episodes
        WHERE show_title = ?
        AND (season > ? OR (season = ? AND episode > ?))
        ORDER BY season ASC, episode ASC
        LIMIT ?
    `, title, season, season, episode, count)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var episodes []*model.Episode
	for rows.Next() {
		ep := &model.Episode{}
		if err := rows.Scan(&ep.Id, &ep.Title, &ep.Season, &ep.Episode, &ep.Path); err != nil {
			panic(err)
		}
		episodes = append(episodes, ep)
	}
	return episodes
}

func (db *DB) GetProgress(title string) int {
	var ts int
	err := db.Conn.QueryRow(`
        SELECT progress
        FROM progress
        WHERE show_title = ?
    `, title).Scan(&ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0 // no progress saved yet
		}
		panic(err)
	}
	return ts
}

// TestGetRandomEpisode returns a random episode from the DB
func (db *DB) TestGetRandomEpisode() *model.Episode {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Count total episodes
	var count int
	err := db.Conn.QueryRow("SELECT COUNT(*) FROM episodes").Scan(&count)
	if err != nil {
		log.Fatal("Failed to count episodes:", err)
	}

	if count == 0 {
		return nil
	}

	// Pick a random offset
	offset := rand.Intn(count)

	// Fetch one episode with offset
	ep := &model.Episode{}
	err = db.Conn.QueryRow(`
		SELECT id, show_title, season, episode, file_path
		FROM episodes
		LIMIT 1 OFFSET ?
	`, offset).Scan(&ep.Id, &ep.Title, &ep.Season, &ep.Episode, &ep.Path)
	if err != nil {
		log.Fatal("Failed to fetch random episode:", err)
	}

	return ep
}
