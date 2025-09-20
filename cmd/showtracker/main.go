package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/yoooby/showtrack/internal/db"
	"github.com/yoooby/showtrack/internal/model"
	"github.com/yoooby/showtrack/internal/scan"
	"github.com/yoooby/showtrack/internal/vlc"
)

func ParseArgs(args []string, db *db.DB) model.Episode {
	switch len(args) {
	case 0:
		ep, err := db.FindLatestWatchedEpisodeGlobal()
		if err != nil {
			panic(err)
		}
		return *ep
	case 1:
		// only show name
		ep, err := db.FindLatestWatchedEpisode(args[0])
		if err != nil {
			panic(err)
		}
		return *ep
	case 3:

		season, err1 := strconv.Atoi(args[1])
		episode, err2 := strconv.Atoi(args[2])
		if err1 != nil || err2 != nil {
			fmt.Println("Error: season and episode must be integers")
		}

		ep, err := db.GetEpisode(args[0], season, episode)
		if err != nil {
			panic(err)
		}
		return *ep
	default:
		fmt.Println("Usage:")
		fmt.Println("  showtracker")
		fmt.Println("  showtracker \"Show Name\"")
		fmt.Println("  showtracker \"Show Name\" <season> <episode>")
		os.Exit(1)
	}
	return model.Episode{}
}

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
	ep := ParseArgs(os.Args[1:], db)
	player.PlayShow(ep)
	select {}
}
