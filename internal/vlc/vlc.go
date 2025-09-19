package vlc

import (
	"fmt"
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

	p.CurrentEP = &ep

	p.Queue = p.db.GetNextEpisodes(ep.Title, ep.Season, ep.Episode, 2)

	p.mu.Unlock()

	go p.playLoop()
}

func (p *Player) playLoop() {
    for {
        p.mu.Lock()
        if p.CurrentEP == nil {
            p.mu.Unlock()
            break
        }

        ep := p.CurrentEP
        p.mu.Unlock()

        progress := p.db.GetProgress(ep.Title)
cmd := exec.Command("vlc",
    "--extraintf", "http",
    "--http-host", p.VLC.Host,
    "--http-port", strconv.Itoa(p.VLC.Port),
    "--http-password", p.VLC.Password,
    fmt.Sprintf("--start-time=%d", progress),
    ep.Path,
)

        done := make(chan struct{})
        go p.trackProgress(ep, done)

        if err := cmd.Start(); err != nil {
            log.Printf("failed to start VLC: %v", err)
            close(done)
            return
        }

        if err := cmd.Wait(); err != nil {
            log.Printf("VLC exited with error: %v", err)
        }
        close(done)

        // move to next episode in queue
        p.mu.Lock()
        if len(p.Queue) > 0 {
            p.CurrentEP = p.Queue[0]
            p.Queue = p.Queue[1:]
        } else {
            p.CurrentEP = nil
        }
        p.mu.Unlock()
    }
}



func (p *Player) trackProgress(ep *model.Episode, done chan struct{}) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-done:
            return // stop tracking when episode finished
        case <-ticker.C:
            status, err := p.VLC.Status()
            if err != nil {
                return
            }
            t := int(status["time"].(float64))
            p.db.SaveProgress(ep.Title, ep.Season, ep.Episode, t)
        }
    }
}
