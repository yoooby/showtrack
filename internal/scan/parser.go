package scan

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/razsteinmetz/go-ptn"
	"github.com/yoooby/showtrack/internal/model"
)

// incase season is in the parent folder
var folderSeasonRe = regexp.MustCompile(`(?i)season[ ._-]?(\d{1,2})`)
// OfflineEpisodeID generates a deterministic hash for offline tracking
func OfflineEpisodeID(title string, season, episode int) string {
	h := sha1.New()
	h.Write([]byte(strings.ToLower(title)))
	h.Write([]byte{byte(season), byte(episode)})
	return hex.EncodeToString(h.Sum(nil))
}
func detectSeasonFromFolder(folder string) int {
	match := folderSeasonRe.FindStringSubmatch(folder)
	if match != nil && len(match) > 1 {
		return atoi(match[1])
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func cleanTitle(title string) string {
	title = strings.Trim(title, " -_.^/\\(){}[]")
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "  ", " ")
	return strings.TrimSpace(title)
}



func ParseEpisode(path string, folderSeason int) *model.Episode {
    ep := &model.Episode{}
    filename := filepath.Base(path)
    parent := filepath.Base(filepath.Dir(path))
	grandparent := filepath.Base(filepath.Dir(filepath.Dir(path)))
   /*  // Detect season from parent folder only
    season := detectSeasonFromFolder(parent)
    if season == 0 {
        season = folderSeason
    }

    ep.Season = season

    // Parse episode number from filename
    for _, p := range episodePatterns {
        if match := p.FindStringSubmatch(filename); match != nil {
            if len(match) >= 3 {
                ep.Season = atoi(match[1])
                ep.Episode = atoi(match[2])
            } else if len(match) == 2 {
                ep.Episode = atoi(match[1])
            }
            break
        }
    }

    // Determine show title
    ep.Title = extractShowName(filename, parent, grandparent)
    ep.Id = OfflineEpisodeID(ep.Title, ep.Season, ep.Episode)
	ep.Path = path */


    torrent, err := ptn.Parse(filename)
    if err != nil {
        panic(err)
    }
    
    
}

// TODO: DO NOT RESCAN THE WHOLE FOLDER ALL THE TIME