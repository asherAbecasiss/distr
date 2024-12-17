package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glasskube/cloud/internal/migrations"
	"github.com/glasskube/cloud/internal/svc"
	"github.com/glasskube/cloud/internal/util"
	"github.com/spf13/pflag"
)

var cliOptions = struct{ Migrate bool }{
	Migrate: true,
}

func init() {
	pflag.BoolVar(&cliOptions.Migrate, "migrate", cliOptions.Migrate, "run database migrations before starting the server")
	pflag.Parse()
}

func main() {
	ctx := context.Background()
	registry := util.Require(svc.NewDefault(ctx))
	defer func() { util.Must(registry.Shutdown()) }()

	if cliOptions.Migrate {
		util.Must(migrations.Up(registry.GetLogger()))
	}

	server := registry.GetServer()
	go onSigterm(func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		server.Shutdown(ctx)
		cancel()
	})

	util.Must(server.Start(":8080"))
}

func onSigterm(callback func()) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT)
	<-sigint
	callback()
}
