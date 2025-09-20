package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/yoooby/showtrack/internal/db"
	"github.com/yoooby/showtrack/internal/model"
	"github.com/yoooby/showtrack/internal/scan"
	"github.com/yoooby/showtrack/internal/vlc"
)

func main() {
	app := &cli.App{
		Name:  "showtracker",
		Usage: "Track and play TV shows with VLC",
		Commands: []*cli.Command{
			{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Configure ShowTracker settings (TV path, database, etc.)",
				Action:  configCommand,
			},
			{
				Name:    "scan",
				Aliases: []string{"s"},
				Usage:   "Rescan TV shows folder (use after adding new episodes)",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "force",
						Aliases: []string{"f"},
						Usage:   "Force full rescan",
					},
				},
				Action: scanCommand,
			},
		},
		Action: defaultAction, // When no command is specified
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func initDB() (*db.DB, error) {
	// Try to get DB path from settings, fallback to default
	dbPath := "db.sqlite3"
	if db, err := db.InitDB(dbPath); err == nil {
		if savedPath := db.GetSetting("db_path"); savedPath != "" {
			dbPath = savedPath
		}
	}
	return db.InitDB(dbPath)
}

func ensureConfigured(db *db.DB) bool {
	scanPath := db.GetSetting("scan_path")
	if scanPath == "" {
		fmt.Println("❌ ShowTracker is not configured yet.")
		fmt.Println("Run 'showtracker config' to set up your TV shows folder.")
		return false
	}

	if db.GetSetting("initial_scan") == "" {
		fmt.Println("❌ Initial scan not completed.")
		fmt.Println("Run 'showtracker scan' to scan your TV shows folder.")
		return false
	}

	return true
}

func configCommand(c *cli.Context) error {
	db, err := initDB()
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== ShowTracker Configuration ===")

	// Configure TV Shows Path
	currentPath := db.GetSetting("scan_path")
	if currentPath != "" {
		fmt.Printf("Current TV shows folder: %s\n", currentPath)
		fmt.Print("Enter new path (or press Enter to keep current): ")
	} else {
		fmt.Print("Enter path to your TV shows folder: ")
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "" { // Only change if user entered something
		// Validate path exists
		if _, err := os.Stat(input); os.IsNotExist(err) {
			fmt.Printf("⚠️  Warning: Path '%s' does not exist!\n", input)
			fmt.Print("Continue anyway? (y/n): ")
			confirm, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
				fmt.Println("Configuration cancelled.")
				return nil
			}
		}

		db.SetSetting("scan_path", input)
		fmt.Printf("✅ TV shows path set to: %s\n", input)

		// Ask if they want to scan now
		fmt.Print("Scan this folder now? (y/n): ")
		scanNow, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(scanNow)) == "y" {
			performScan(input, db)
		} else {
			fmt.Println("Remember to run 'showtracker scan' before playing episodes.")
		}
	} else if currentPath != "" {
		fmt.Printf("✅ Keeping current path: %s\n", currentPath)
	}

	// Configure Database Path
	fmt.Println("\n--- Database Settings ---")
	currentDB := db.GetSetting("db_path")
	if currentDB == "" {
		currentDB = "db.sqlite3"
	}
	fmt.Printf("Current database: %s\n", currentDB)
	fmt.Print("Enter new database path (or press Enter to keep current): ")

	dbInput, _ := reader.ReadString('\n')
	dbInput = strings.TrimSpace(dbInput)

	if dbInput != "" { // Only change if user entered something
		db.SetSetting("db_path", dbInput)
		fmt.Printf("✅ Database path set to: %s\n", dbInput)
		fmt.Println("⚠️  You'll need to restart ShowTracker for database changes to take effect.")
	} else {
		fmt.Printf("✅ Keeping current database: %s\n", currentDB)
	}

	// Configure VLC Settings
	fmt.Println("\n--- VLC Settings ---")

	currentPassword := db.GetSetting("vlc_password")
	if currentPassword == "" {
		currentPassword = "zebi"
	}
	fmt.Printf("Current VLC password: %s\n", currentPassword)
	fmt.Print("Enter new VLC password (or press Enter to keep current): ")

	passInput, _ := reader.ReadString('\n')
	passInput = strings.TrimSpace(passInput)
	if passInput != "" { // Only change if user entered something
		db.SetSetting("vlc_password", passInput)
		fmt.Printf("✅ VLC password set to: %s\n", passInput)
	} else {
		fmt.Printf("✅ Keeping current password: %s\n", currentPassword)
	}

	currentPort := db.GetSetting("vlc_port")
	if currentPort == "" {
		currentPort = "42069"
	}
	fmt.Printf("Current VLC port: %s\n", currentPort)
	fmt.Print("Enter new VLC port (or press Enter to keep current): ")

	portInput, _ := reader.ReadString('\n')
	portInput = strings.TrimSpace(portInput)
	if portInput != "" { // Only change if user entered something
		if _, err := strconv.Atoi(portInput); err != nil {
			fmt.Println("❌ Invalid port number")
		} else {
			db.SetSetting("vlc_port", portInput)
			fmt.Printf("✅ VLC port set to: %s\n", portInput)
		}
	} else {
		fmt.Printf("✅ Keeping current port: %s\n", currentPort)
	}

	fmt.Println("\n🎉 Configuration complete!")
	return nil
}

func performScan(path string, db *db.DB) {
	fmt.Printf("🔍 Scanning folder: %s\n", path)

	episodes, err := scan.ScanFolder(path, db)
	if err != nil {
		fmt.Printf("❌ Error scanning folder: %v\n", err)
		return
	}

	fmt.Printf("📺 Found %d episodes\n", len(episodes))

	err = db.SaveEpisdoes(episodes)
	if err != nil {
		fmt.Printf("❌ Error saving episodes: %v\n", err)
		return
	}

	db.SetSetting("initial_scan", "completed")
	fmt.Println("✅ Scan completed successfully!")
}

func scanCommand(c *cli.Context) error {
	db, err := initDB()
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	scanPath := db.GetSetting("scan_path")
	if scanPath == "" {
		fmt.Println("❌ No TV shows path configured.")
		fmt.Println("Run 'showtracker config' first to set your TV shows folder.")
		return nil
	}

	if !c.Bool("force") && db.GetSetting("initial_scan") != "" {
		fmt.Println("🔄 Performing scan...")
	} else {
		fmt.Println("🔄 Performing full scan...")
		db.Conn.Exec("DELETE FROM folder_hashes")
		db.Conn.Exec("DELETE FROM episodes")
	}

	performScan(scanPath, db)
	return nil
}

func defaultAction(c *cli.Context) error {
	db, err := initDB()
	if err != nil {
		return fmt.Errorf("database error: %v", err)
	}

	// Check if configured
	if !ensureConfigured(db) {
		return nil
	}

	// Parse arguments for episode selection
	var episode model.Episode
	args := c.Args().Slice()

	switch len(args) {
	case 0:
		// Play latest watched episode globally
		ep, err := db.FindLatestWatchedEpisodeGlobal()
		if err != nil {
			fmt.Println("❌ No episodes found. Try scanning your TV folder:")
			fmt.Println("  showtracker scan")
			return nil
		}
		episode = *ep
	case 1:
		// Play latest episode of specific show
		ep, err := db.FindLatestWatchedEpisode(args[0])
		if err != nil {
			return fmt.Errorf("show not found: %v", err)
		}
		episode = *ep
	case 3:
		// Play specific episode
		season, err1 := strconv.Atoi(args[1])
		episodeNum, err2 := strconv.Atoi(args[2])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("season and episode must be integers")
		}

		ep, err := db.GetEpisode(args[0], season, episodeNum)
		if err != nil {
			return fmt.Errorf("episode not found: %v", err)
		}
		episode = *ep
	default:
		fmt.Println("Usage:")
		fmt.Println("  showtracker                           # Play latest watched episode")
		fmt.Println("  showtracker \"Show Name\"               # Play latest episode of show")
		fmt.Println("  showtracker \"Show Name\" <season> <episode>  # Play specific episode")
		fmt.Println("  showtracker config                    # Configure settings")
		fmt.Println("  showtracker scan                      # Rescan TV folder")
		return nil
	}

	fmt.Printf("▶️  Playing: %s S%02dE%02d\n", episode.Title, episode.Season, episode.Episode)

	// Get VLC settings from database
	password := db.GetSetting("vlc_password")
	if password == "" {
		password = "zebi"
	}

	portStr := db.GetSetting("vlc_port")
	port := 42069
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	player := vlc.NewPlayer(password, port, *db)
	player.PlayShow(episode)

	// Keep program running
	select {}
}
