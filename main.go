package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
)

const defaultConfigDir = `C:\ProgramData\winsched`
const defaultConfigFile = defaultConfigDir + `\config.yaml`

func main() {
	action := "run"
	configPath := defaultConfigFile

	if len(os.Args) >= 2 {
		action = os.Args[1]
	}
	if len(os.Args) >= 3 {
		configPath = os.Args[2]
	}

	switch action {
	case "install":
		if err := installService(true); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Install complete. Use 'sc start winsched' or restart to begin.")

	case "uninstall":
		if err := uninstallService(true); err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
			os.Exit(1)
		}

	case "run":
		run(configPath)

	default:
		fmt.Fprintf(os.Stderr, "Usage: %s [install|uninstall|run] [config.yaml]\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}
}

func run(configPath string) {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot detect session type: %v\n", err)
		os.Exit(1)
	}

	// In interactive session, log to console + file (no event log typically available).
	// In service session, log to file + event log.
	if isInteractive {
		// Create config dir if it doesn't exist (for development convenience).
		os.MkdirAll(filepath.Dir(configPath), 0755)
	}

	logger, err := NewLogger(InfoLevel, defaultConfigDir+`\service.log`, isInteractive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create logger: %v\n", err)
		os.Exit(1)
	}

	ws := &winService{configPath: configPath, interactive: isInteractive, logger: logger}

	if isInteractive {
		runInteractive(configPath, logger)
	} else {
		if err := svc.Run("winsched", ws); err != nil {
			logger.Error("Service failed: %v", err)
			logger.Close()
			os.Exit(1)
		}
	}
}

func initStorage(configDir string) {
	SetLogsDir(filepath.Join(configDir, "logs"))
	InitExecID()
}
