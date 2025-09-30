package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/kardianos/service"
)

var (
	serviceCommand = flag.String("service", "", "manage the backrest background service (install, uninstall, start, stop, restart, run)")
	serviceUser    = flag.String("service-user", "", "user account to run the service as (platform dependent)")
	serviceArgs    []string
)

func init() {
	flag.Func("service-arg", "additional argument to pass to the backrest service when it runs (repeatable)", func(value string) error {
		if value == "" {
			return errors.New("service-arg cannot be empty")
		}
		serviceArgs = append(serviceArgs, value)
		return nil
	})
}

func handleServiceCommand() bool {
	if *serviceCommand == "" {
		return false
	}

	cfg := &service.Config{
		Name:        "backrest",
		DisplayName: "Backrest",
		Description: "Backrest backup orchestrator",
		Arguments:   append([]string{"--service", "run"}, serviceArgs...),
	}

	if *serviceUser != "" {
		cfg.UserName = *serviceUser
	}

	if runtime.GOOS == "linux" {
		cfg.Option = service.KeyValue{
			"Restart":           "on-failure",
			"RestartSec":        "5",
			"SuccessExitStatus": "0 2",
		}
	}

	svc, err := service.New(newBackrestService(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure service: %v\n", err)
		os.Exit(1)
	}

	switch *serviceCommand {
	case "install":
		err = svc.Install()
	case "uninstall":
		err = svc.Uninstall()
	case "start":
		err = svc.Start()
	case "stop":
		err = svc.Stop()
	case "restart":
		err = svc.Restart()
	case "run":
		err = svc.Run()
	default:
		fmt.Fprintf(os.Stderr, "unknown service command %q\n", *serviceCommand)
		os.Exit(1)
	}

	if err != nil {
		if errors.Is(err, service.ErrNotInstalled) && (*serviceCommand == "stop" || *serviceCommand == "uninstall") {
			err = nil
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "service command %q failed: %v\n", *serviceCommand, err)
		os.Exit(1)
	}

	return true
}

type backrestService struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newBackrestService() *backrestService {
	return &backrestService{}
}

func (s *backrestService) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		runAppWithContext(ctx, false)
	}()
	return nil
}

func (s *backrestService) Stop(_ service.Service) error {
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
	return nil
}
