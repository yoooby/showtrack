# ShowTracker

A command-line TV show tracker that automatically parses and manages episode playback with VLC media player.
Keep track of your watching progress and seamlessly continue where you left off.

## Why ?
well this is for my use, i torrent alot and the way i used to keep track of where i left off was by literally chaning the file name of the current episode 
i was watching.
Why not just use jellyfin ? 
well this is more lightweight plus i don't need have to have a server running on my local pc,

## Installation
```bash
go get github.com/yoooby/showtrack
go build -o showtrack
```
Also make sure VLC is installed and vlc is added to path

## Usage


## Usage

```bash
# Play latest watched episode globally
showtrack
# Play latest watched episode of specific show  
showtrack "Show Name"
# Play specific episode (show, season, episode)
showtrack "Lost" 2 10
# Configure settings (TV folder, VLC settings, etc)
showtrack config
# Rescan TV folder for new episodes
showtrack scan
# Force full rescan (clears cache)
showtrack scan --force
```


## Dependencies
- Go 1.21+
- VLC Media Player (Duh)


## Roadmap
- [ ] Fuzzy searching for show names because i can't godamn type
- [ ] better parsing
- [ ] CLI interface


