package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"unicode"

	_ "github.com/mattn/go-sqlite3" // <-- needed for SQLite driver
	"github.com/yoooby/showtrack/internal/model"
)

type DB struct {
	Conn *sql.DB
}

// Fuzzy search utilities
func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	rows, cols := len(r1)+1, len(r2)+1

	d := make([][]int, rows)
	for i := range d {
		d[i] = make([]int, cols)
	}

	for i := 1; i < rows; i++ {
		d[i][0] = i
	}
	for j := 1; j < cols; j++ {
		d[0][j] = j
	}

	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}

	return d[rows-1][cols-1]
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func normalizeString(s string) string {
	// Convert to lowercase and remove non-alphanumeric characters
	var result strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

func stringSimilarity(s1, s2 string) float64 {
	s1, s2 = normalizeString(s1), normalizeString(s2)
	if s1 == s2 {
		return 1.0
	}

	maxLen := max(len(s1), len(s2))
	if maxLen == 0 {
		return 0.0
	}

	distance := levenshteinDistance(s1, s2)
	return 1.0 - float64(distance)/float64(maxLen)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// findBestShowMatch finds the best matching show title using fuzzy search
func (db *DB) findBestShowMatch(query string) (string, error) {
	// First try to find exact match in progress table
	var exactMatch string
	err := db.Conn.QueryRow(`
		SELECT show_title FROM progress WHERE show_title = ?
	`, strings.ToLower(query)).Scan(&exactMatch)
	if err == nil {
		return exactMatch, nil
	}

	// If no exact match, get all show titles and find best fuzzy match
	rows, err := db.Conn.Query(`
		SELECT DISTINCT show_title FROM progress
		UNION
		SELECT DISTINCT show_title FROM episodes
	`)
	if err != nil {
		return "", fmt.Errorf("failed to query show titles: %w", err)
	}
	defer rows.Close()

	var bestMatch string
	var bestScore float64
	const minSimilarity = 0.6 // Minimum similarity threshold

	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			continue
		}

		similarity := stringSimilarity(query, title)
		if similarity > bestScore && similarity >= minSimilarity {
			bestScore = similarity
			bestMatch = title
		}
	}

	if bestMatch == "" {
		return "", fmt.Errorf("no similar show found for: %s", query)
	}

	return bestMatch, nil
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
	var season, episode int

	err := db.Conn.QueryRow(`
		SELECT show_title, last_watched_season, last_watched_episode
		FROM progress
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&title, &season, &episode)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no watched episodes found")
		}
		return nil, fmt.Errorf("failed to query latest watched episode: %w", err)
	}
	return db.GetEpisode(title, season, episode)
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

func (db *DB) SaveEpisodes(eps []model.Episode) error {
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

func (db *DB) GetNextEpisodes(title string, season int, episode int, count int) ([]*model.Episode, error) {
	rows, err := db.Conn.Query(`
        SELECT id, show_title, season, episode, file_path
        FROM episodes
        WHERE show_title = ?
        AND (season > ? OR (season = ? AND episode > ?))
        ORDER BY season ASC, episode ASC
        LIMIT ?
    `, title, season, season, episode, count)
	if err != nil {
		return nil, fmt.Errorf("failed to query next episodes: %w", err)
	}
	defer rows.Close()

	var episodes []*model.Episode
	for rows.Next() {
		ep := &model.Episode{}
		if err := rows.Scan(&ep.Id, &ep.Title, &ep.Season, &ep.Episode, &ep.Path); err != nil {
			return nil, fmt.Errorf("failed to scan episode: %w", err)
		}
		episodes = append(episodes, ep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return episodes, nil
}

func (db *DB) GetProgress(title string) (int, error) {
	var ts int
	err := db.Conn.QueryRow(`
        SELECT progress
        FROM progress
        WHERE show_title = ?
    `, title).Scan(&ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil // no progress saved yet
		}
		return 0, fmt.Errorf("failed to get progress for %s: %w", title, err)
	}
	return ts, nil
}

func (db *DB) FindLatestWatchedEpisode(query string) (*model.Episode, error) {
	var bestMatch string
	err := db.Conn.QueryRow(`
		SELECT show_title FROM progress WHERE show_title = ?
	`, query).Scan(&bestMatch)

	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		// No exact match, use fuzzy search
		bestMatch, err = db.findBestShowMatch(query)
		if err != nil {
			return nil, err
		}
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
