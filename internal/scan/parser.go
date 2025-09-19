package scan

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yoooby/showtrack/internal/model"
)

// please be enough or i'll kill my self
var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,3})`),                    // S01E02
	regexp.MustCompile(`(?i)(\d{1,2})x(\d{1,3})`),                     // 1x02
	regexp.MustCompile(`(?i)[Ee](\d{1,3})`),                            // E01, E1
	regexp.MustCompile(`(?i)Season[ ._-]?(\d{1,2})[ ._-]?Episode[ ._-]?(\d{1,3})`),
	regexp.MustCompile(`(?i)S(\d{1,2})[.-]?E(\d{1,3})`),               // S01.E02 or S01-E02
}

// incase season is in the parent folder
var folderSeasonRe = regexp.MustCompile(`(?i)season[ ._-]?(\d{1,2})`)

var container = regexp.MustCompile(`\.(mkv|mp4|avi|mov|webm)$`)

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


func extractShowName(filename, parentFolder, grandparentFolder string) string {
    // Step 1: Clean filename and remove season/episode markers
    title := cleanTitle(container.ReplaceAllString(filename, ""))
    for _, p := range episodePatterns {
        title = p.ReplaceAllString(title, "")
    }
    title = strings.TrimSpace(title)

    // Step 2: Fallback to parent folder if filename has no meaningful title
    if title == "" || len(title) <= 2 {
        title = cleanTitle(parentFolder)
    }

    // Step 3: Fallback to grandparent folder if parent is generic (like "S01")
    if title == "" || strings.HasPrefix(strings.ToLower(title), "s") {
        title = cleanTitle(grandparentFolder)
    }

    // Step 4: Final fallback
    if title == "" {
        title = "Unknown Show"
    }

    return title
}
func ParseEpisode(path string, folderSeason int) *model.Episode {
    ep := &model.Episode{}
    filename := filepath.Base(path)
    parent := filepath.Base(filepath.Dir(path))
	grandparent := filepath.Base(filepath.Dir(filepath.Dir(path)))
    // Detect season from parent folder only
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
	ep.Path = path
    return ep
}

// TODO: DO NOT RESCAN THE WHOLE FOLDER ALL THE TIME