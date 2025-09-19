package vlc

import (
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/yoooby/showtrack/internal/db"
	"github.com/yoooby/showtrack/internal/model"
)




type Player struct {
	VLC       *VLC
	CurrentEP *model.Episode
	Queue     []*model.Episode
	mu        sync.Mutex
	isRunning bool
	vlcCmd *exec.Cmd
	db        *db.DB
}

func NewPlayer(password string, port int, db db.DB) *Player {
	vlc := VLC{
		Host: "127.0.0.1",
		Port: port,
		Password: password,
	}

	return &Player{
		VLC: &vlc,
		db: &db,
	}
}


func (p *Player) PlayShow(ep model.Episode) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    p.CurrentEP = &ep
    p.Queue = p.db.GetNextEpisodes(ep.Title, ep.Season, ep.Episode, 2)
    
    if !p.isRunning {
        go p.startVLC()
    }
    
    // Clear current playlist and add episodes
    p.setupInitialQueue()
    
    go p.monitorPlayback()
}


func (p *Player) startVLC() {
    // Start VLC process
    p.vlcCmd = exec.Command("vlc", 
        "--extraintf", "http",
        "--http-host", p.VLC.Host,
        "--http-port", strconv.Itoa(p.VLC.Port),
        "--http-password", p.VLC.Password,
        "--fullscreen",  // Opens in fullscreen video mode
    )
    
    if err := p.vlcCmd.Start(); err != nil {  // <-- VLC process starts here
        log.Printf("Failed to start VLC: %v", err)
        return
    }
    
    p.isRunning = true
    time.Sleep(2 * time.Second)  // Wait for VLC to initialize
}



func (p *Player) monitorPlayback() {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    var lastPlaylistLength int
    
    for p.isRunning {
        select {
        case <-ticker.C:
            status, err := p.VLC.Status()
            if err != nil {
                log.Printf("Failed to get VLC status: %v", err)
                continue
            }
            
            // Track progress for current episode
            if p.CurrentEP != nil && status["state"] == "playing" {
                currentTime := int(status["time"].(float64))
                p.db.SaveProgress(p.CurrentEP.Title, p.CurrentEP.Season, p.CurrentEP.Episode, currentTime)
            }
            
            // Check if episode changed (playlist position changed)
            playlistLength := int(status["length"].(float64))
            
            // If playlist got shorter, an episode finished
            if playlistLength < lastPlaylistLength {
                p.onEpisodeFinished()
            }
            
            lastPlaylistLength = playlistLength
            
            // Maintain 3 episodes in VLC queue
            p.maintainQueue(playlistLength)
        }
    }
}

func (p *Player) setupInitialQueue() {
    // Clear playlist
    if err := p.VLC.ClearPlaylist(); err != nil {
        log.Printf("Failed to clear playlist: %v", err)
        return
    }
    
    // Add current episode
    if p.CurrentEP != nil {
        progress := p.db.GetProgress(p.CurrentEP.Title)
        if err := p.VLC.AddToPlaylist(p.CurrentEP.Path); err != nil {
            log.Printf("Failed to add current episode: %v", err)
            return
        }
        
        // Start playing and seek to saved position
        p.VLC.Play()
        if progress > 0 {
            p.VLC.Seek(progress)
        }
    }
    
    // Add queue episodes to VLC playlist
    for _, ep := range p.Queue {
        if err := p.VLC.AddToPlaylist(ep.Path); err != nil {
            log.Printf("Failed to add episode to playlist: %v", err)
        }
    }
}

func (p *Player) onEpisodeFinished() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // Move to next episode in our internal tracking
    if len(p.Queue) > 0 {
        p.CurrentEP = p.Queue[0]
        p.Queue = p.Queue[1:]
    } else {
        p.CurrentEP = nil
    }
}

func (p *Player) maintainQueue(currentPlaylistLength int) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // If playlist has less than 3 items, add more
    if currentPlaylistLength < 3 && p.CurrentEP != nil {
        // Get more episodes from database
        moreEpisodes := p.db.GetNextEpisodes(p.CurrentEP.Title, p.CurrentEP.Season, p.CurrentEP.Episode, 5-len(p.Queue))
        
        // Add new episodes to internal queue
        for _, ep := range moreEpisodes {
            // Skip if already in queue
            if !p.isInQueue(ep) {
                p.Queue = append(p.Queue, ep)
                
                // Add to VLC playlist
                if err := p.VLC.AddToPlaylist(ep.Path); err != nil {
                    log.Printf("Failed to add episode to VLC playlist: %v", err)
                }
            }
        }
    }
}

func (p *Player) isInQueue(ep *model.Episode) bool {
    for _, queueEp := range p.Queue {
        if queueEp.Title == ep.Title && queueEp.Season == ep.Season && queueEp.Episode == ep.Episode {
            return true
        }
    }
    return false
}

func (p *Player) Stop() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    p.isRunning = false
    if p.vlcCmd != nil && p.vlcCmd.Process != nil {
        p.vlcCmd.Process.Kill()
    }
}