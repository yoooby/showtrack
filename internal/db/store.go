package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3" // <-- needed for SQLite driver
	"github.com/yoooby/showtrack/internal/model"
)

type DB struct {
	Conn *sql.DB
}

func (db *DB) GetSetting(key string) string {
	var value string
	err := db.Conn.QueryRow(`
		SELECT value FROM settings WHERE key = ?
	`, key).Scan(&value)

	if err != nil {
		if err == sql.ErrNoRows {
			return "" // Setting doesn't exist
		}
		// Log error but don't panic in production
		log.Printf("Error getting setting %s: %v", key, err)
		return ""
	}

	return value
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Conn.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)

	if err != nil {
		log.Printf("Error setting %s: %v", key, err)
	}

	return err
}

func (db *DB) FindLatestWatchedEpisodeGlobal() (*model.Episode, error) {
	var title string
	var seasom, episode int

	err := db.Conn.QueryRow(`
		SELECT show_title, last_watched_season, last_watched_episode
		FROM progress
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&title, &seasom, &episode)

	if err != nil {
		if err != sql.ErrNoRows {
			return nil, fmt.Errorf("no watched episodes found")
		}
		return nil, err
	}
	return db.GetEpisode(title, seasom, episode)
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
            progress INTEGER,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS folder_hashes (
		path TEXT PRIMARY KEY,
		hash TEXT
	)
	`)
	return &DB{Conn: conn}, err
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
		_, err := stmt.Exec(ep.Id, strings.ToLower(ep.Title), ep.Season, ep.Episode, ep.Path)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	tx.Commit()

	return err
}

func (db *DB) SaveProgress(show string, season, episode, progress int) error {
	_, err := db.Conn.Exec(`
        INSERT INTO progress (show_title, last_watched_season, last_watched_episode, progress, updated_at)
        VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(show_title) DO UPDATE SET
            last_watched_season = excluded.last_watched_season,
            last_watched_episode = excluded.last_watched_episode,
            progress = excluded.progress,
			updated_at = CURRENT_TIMESTAMP
    `, show, season, episode, progress)
	if err != nil {
		return err
	}

	return nil
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

func (db *DB) FindLatestWatchedEpisode(query string) (*model.Episode, error) {
	var bestMatch string
	err := db.Conn.QueryRow(`
		SELECT show_title
		FROM progress_fts
		WHERE progress_fts MATCH ?
		LIMIT 1
	`, query).Scan(&bestMatch)

	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		// fallback: not in progress, use query directly as title
		bestMatch = query
	}

	var season, episode int
	err = db.Conn.QueryRow(`
		SELECT last_watched_season, last_watched_episode
		FROM progress
		WHERE show_title = ?
	`, bestMatch).Scan(&season, &episode)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var ep model.Episode
	if err == nil {
		return db.GetEpisode(query, season, episode)
		if err == nil {
			return &ep, nil
		}
	}

	err = db.Conn.QueryRow(`
		SELECT id, show_title, season, episode, file_path
		FROM episodes
		WHERE show_title = ?
		ORDER BY season ASC, episode ASC
		LIMIT 1
	`, bestMatch).Scan(&ep.Id, &ep.Title, &ep.Season, &ep.Episode, &ep.Path)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("show not found: %s", bestMatch)
		}
		return nil, err
	}

	return &ep, nil
}

func (db *DB) GetEpisode(title string, season int, episode int) (*model.Episode, error) {
	var ep model.Episode
	err := db.Conn.QueryRow(`
		SELECT id, show_title, season, episode, file_path
		FROM episodes
		WHERE show_title = ? AND season = ? AND episode = ?
	`, title, season, episode).Scan(&ep.Id, &ep.Title, &ep.Season, &ep.Episode, &ep.Path)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("episode not found: %s S%dE%d", title, season, episode)
		}
		return nil, err
	}

	return &ep, nil
}
