package scan

import (
	"os"
	"path/filepath"
	"strings"

	"crypto/md5"
	"encoding/hex"
	"io/fs"
	"sort"
	"strconv"

	"github.com/razsteinmetz/go-ptn"
	"github.com/yoooby/showtrack/internal/db"
	"github.com/yoooby/showtrack/internal/model"
)

// hashImmediateFiles computes a hash of all immediate files in a folder.
func hashImmediateFiles(folder string) string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return ""
	}

	var files []fs.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e)
		}
	}

	// Sort filenames to get consistent hash
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })

	hash := md5.New()
	for _, f := range files {
		info, _ := f.Info()
		hash.Write([]byte(f.Name()))
		hash.Write([]byte(strconv.FormatInt(info.Size(), 10)))
		hash.Write([]byte(info.ModTime().String()))
	}

	return hex.EncodeToString(hash.Sum(nil))
}

var videoExts = map[string]bool{
    ".mp4":  true,
    ".mkv":  true,
    ".avi":  true,
    ".mov":  true,
    ".wmv":  true,
}

// ScanFolder recursively scans folders, subfolders, etc.
func ScanFolder(root string, db *db.DB) ([]model.Episode, error) {
    var episodes []model.Episode

    // Walk recursively
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if info.IsDir() {
            // Hash only immediate files in this folder
            hash := hashImmediateFiles(path)

            var oldHash string
            err := db.Conn.QueryRow("SELECT hash FROM folder_hashes WHERE path = ?", path).Scan(&oldHash)
            if err == nil && oldHash == hash {
                // Folder unchanged → skip scanning files inside
                return filepath.SkipDir
            }

            // Update DB with new hash
            _, _ = db.Conn.Exec(`
                INSERT INTO folder_hashes(path, hash)
                VALUES (?, ?)
                ON CONFLICT(path) DO UPDATE SET hash=excluded.hash
            `, path, hash)

            return nil
        }
		       // Skip non-video files
        ext := strings.ToLower(filepath.Ext(info.Name()))
        if !videoExts[ext] {
            return nil
        }


/*         folderSeason := 0
        parent := filepath.Base(filepath.Dir(path))
        folderSeason = detectSeasonFromFolder(parent)
        if folderSeason == 0 {
            grandparent := filepath.Base(filepath.Dir(filepath.Dir(path)))
            folderSeason = detectSeasonFromFolder(grandparent)
        } */

        //ep := ParseEpisode(info.Name(), folderSeason)
        torrent, err := ptn.Parse(info.Name())
        if err != nil {
            return err
        }
        if torrent.IsMovie {
            return nil
        }
        ep := model.Episode{
            Id: OfflineEpisodeID(torrent.Title, torrent.Season, torrent.Episode),
            Title: torrent.Title,
            Episode: torrent.Episode,
            Season: torrent.Season,
            Path: path,
        }
        episodes = append(episodes, ep)
        return nil
    })

    return episodes, err
}
