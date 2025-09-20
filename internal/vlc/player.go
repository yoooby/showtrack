package vlc

import (
	"fmt"
	"log"
	"os"
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
	vlcCmd    *exec.Cmd
	db        *db.DB
}

func NewPlayer(password string, port int, db db.DB) *Player {
	vlc := VLC{
		Host:     "127.0.0.1",
		Port:     port,
		Password: password,
	}

	return &Player{
		VLC: &vlc,
		db:  &db,
	}
}

func (p *Player) PlayShow(ep model.Episode) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.CurrentEP = &ep
	p.Queue = p.db.GetNextEpisodes(ep.Title, ep.Season, ep.Episode, 2)
	log.Println("Queue length:", len(p.Queue))
	for _, e := range p.Queue {
		log.Println(e.Title, e.Season, e.Episode, e.Path)
	}
	if !p.isRunning {
		p.startVLC()
	}

	p.setupInitialQueue()

	go p.monitorPlayback()
}

func (p *Player) startVLC() {
	p.vlcCmd = exec.Command("vlc",
		"--extraintf", "http",
		"--http-host", p.VLC.Host,
		"--http-port", strconv.Itoa(p.VLC.Port),
		"--http-password", p.VLC.Password,
		"--fullscreen", // Opens in fullscreen video mode
	)

	log.Println("started http")
	if err := p.vlcCmd.Start(); err != nil {
		log.Printf("Failed to start VLC: %v", err)
		return
	}

	p.isRunning = true
	go func() {
		err := p.vlcCmd.Wait() // This blocks until VLC exits
		log.Printf("VLC process exited: %v", err)
		log.Println("VLC closed, exiting program...")
		os.Exit(9)
	}()
	time.Sleep(2 * time.Second)
}
func (p *Player) monitorPlayback() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastPlaylistLength int
	var lastCurrentPos int = -1

	for p.isRunning {
		select {
		case <-ticker.C:
			status, err := p.VLC.Status()
			if err != nil {
				log.Printf("Failed to get VLC status: %v", err)
				continue
			}
			if p.CurrentEP != nil && status["state"] == "playing" {
				currentTime := int(status["time"].(float64))
				println("called saveprogress")
				err := p.db.SaveProgress(p.CurrentEP.Title, p.CurrentEP.Season, p.CurrentEP.Episode, currentTime)
				if err != nil {
					fmt.Println("Error", err.Error())
				}
			}

			currentPos := int(status["currentplid"].(float64))
			playlistLength := int(status["length"].(float64))

			if currentPos != lastCurrentPos && lastCurrentPos != -1 {
				p.onEpisodeFinished()
				log.Printf("Episode changed: position %d -> %d", lastCurrentPos, currentPos)
			}

			if playlistLength < lastPlaylistLength {
				p.onEpisodeFinished()
				log.Printf("Playlist shortened: %d -> %d", lastPlaylistLength, playlistLength)
			}

			lastCurrentPos = currentPos
			lastPlaylistLength = playlistLength

			p.maintainQueue(playlistLength)
		}
	}
}

func (p *Player) setupInitialQueue() {
	if err := p.VLC.ClearPlaylist(); err != nil {
		log.Printf("Failed to clear playlist: %v", err)
		return
	}

	if p.CurrentEP != nil {
		progress := p.db.GetProgress(p.CurrentEP.Title)
		if err := p.VLC.AddToPlaylist(p.CurrentEP.Path); err != nil {
			log.Printf("Failed to add current episode: %v", err)
			return
		}

		p.VLC.Play()
		if progress > 0 {
			p.VLC.Seek(progress)
		}
	}

	for _, ep := range p.Queue {
		if err := p.VLC.AddToPlaylist(ep.Path); err != nil {
			log.Printf("Failed to add episode to playlist: %v", err)
		}
	}
}

func (p *Player) onEpisodeFinished() {
	p.mu.Lock()
	defer p.mu.Unlock()

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

	if len(p.Queue) < 3 && p.CurrentEP != nil {
		moreEpisodes := p.db.GetNextEpisodes(p.CurrentEP.Title, p.CurrentEP.Season, p.CurrentEP.Episode, 5-len(p.Queue))

		for _, ep := range moreEpisodes {
			if !p.isInQueue(ep) {
				p.Queue = append(p.Queue, ep)

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
