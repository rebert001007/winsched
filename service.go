package main

import (
	"golang.org/x/sys/windows/svc"
)

// winService implements the svc.Handler interface.
type winService struct {
	configPath  string
	interactive bool
	logger      *Logger
	config      *Config
	scheduler   *Scheduler
	apiServer   *APIServer
}

// Execute is the callback from the Service Control Manager.
func (ws *winService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	ws.config = LoadConfig(ws.configPath, ws.logger)
	ws.scheduler = NewScheduler(ws.config, ws.logger)
	ws.scheduler.Start()

	if ws.config.API.Enabled {
		ws.apiServer = NewAPIServer(ws.config, ws.configPath, ws.scheduler, ws.logger)
		ws.apiServer.Start()
	}

	ws.logger.Info("WinSched service started (%d tasks loaded)", len(ws.config.Tasks))

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				ws.logger.Info("WinSched stopping...")
				if ws.apiServer != nil {
					ws.apiServer.Stop()
				}
				ws.scheduler.Stop()
				ws.logger.Info("WinSched stopped")
				ws.logger.Close()
				return false, 0
			default:
				ws.logger.Warn("Unexpected control request: %d", c.Cmd)
			}
		}
	}
}

// runInteractive runs the service in foreground (debug/terminal mode).
func runInteractive(configPath string, logger *Logger) {
	cfg := LoadConfig(configPath, logger)
	sched := NewScheduler(cfg, logger)
	sched.Start()

	if cfg.API.Enabled {
		api := NewAPIServer(cfg, configPath, sched, logger)
		api.Start()
	}

	logger.Info("WinSched running in interactive mode (%d tasks loaded)", len(cfg.Tasks))

	// Block forever. On Ctrl+C, Windows will terminate the process.
	select {}
}
