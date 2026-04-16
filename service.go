package service

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"goodin/cmd"

	core "goodin/internal"

	"github.com/spf13/cobra"
)

// TODO : implement unit test for service.go
const banner = `
   ____             _
  / __/__ _____  __(_)______
 _\ \/ -_) __/ |/ / / __/ -_)
/___/\__/_/  |___/_/\__/\__/

---------------------------------------------

`

type BaseConfig struct {
	DisableBanner bool
}

type Service struct {
	App            core.App
	RootCmd        *cobra.Command
	TemplateConfig BaseConfig
}

func New(config BaseConfig) *Service {
	l := &Service{
		App: core.NewBaseApp(),
		RootCmd: &cobra.Command{
			Use:   "mox",
			Short: "Zero Downtime Proxy",
			FParseErrWhitelist: cobra.FParseErrWhitelist{
				UnknownFlags: true,
			},
			CompletionOptions: cobra.CompletionOptions{
				DisableDefaultCmd: true,
			},
		},
		TemplateConfig: config,
	}

	return l
}

func (t *Service) ShutdownSignal() {
	// listen for interrupt signal to gracefully shutdown the application
	go func() {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
		<-sigch

		t.App.Stop()
	}()
}

func (t *Service) Shutdown() {
	t.App.OnApplicationStop().Execute(core.CloseEvent{App: t.App})

	t.App.Driver().CloseAllDriver(func(s string, err error) {
		t.App.Logger().Warn("Error closing driver", slog.Any("app", s), slog.Any("err", err))
	})

	t.App.Shutdown()
}

func (t *Service) registerCommand() {
	commands := cmd.NewCommands(t.App)

	for _, command := range commands {
		t.RootCmd.AddCommand(command)
	}
}

func (t *Service) Start() {
	t.App.Bootstrap()
	t.registerCommand()

	if !t.TemplateConfig.DisableBanner {
		fmt.Print(banner)
	}
}
