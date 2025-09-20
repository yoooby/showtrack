package main

import (
	"fmt"

	"github.com/yoooby/showtrack/internal/db"
	"github.com/yoooby/showtrack/internal/scan"
	"github.com/yoooby/showtrack/internal/vlc"
)

func main() {
	// init db

	db, err := db.InitDB("db.sqlite3")
	if err != nil {
		panic("ERROR DB " + err.Error())
	}

	episodes, err := scan.ScanFolder("/Users/ayoubidgoufkir/Downloads/tv", db)
	if err != nil {
		panic(err)
	}
	err = db.SaveEpisdoes(episodes)
	if err != nil {
		panic("error rebek" + err.Error())
	}
	player := vlc.NewPlayer("zebi", 42069, *db)
	ep := db.TestGetRandomEpisode()
	fmt.Println(ep.Title, ep.Season, ep.Episode, ep.Path)
	player.PlayShow(*ep)
	select {}
}
