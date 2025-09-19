package vlc

import (
	"fmt"
	"os/exec"
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
			"--http-port=" + p.VLC.Host,
			"--http-password=" + p.VLC.Password,
			fmt.Sprintf("--start-time=%d", progress),
			ep.Path,
		)
        done := make(chan struct{}) // signal channel for this episode

        go p.trackProgress(ep, done) // pass channel

        cmd.Start()
        cmd.Wait()      // blocks until VLC exits
        close(done)     // signal trackProgress to stop

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
