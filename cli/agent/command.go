// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andydunstall/pico/agent"
	"github.com/andydunstall/pico/agent/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [flags]",
		Short: "start the pico agent",
		Long: `Start the Pico agent.

The Pico agent is a CLI that runs alongside your upstream service that
registers one or more listeners.

The agent will connect to a Pico server, register the configured listeners,
then forwards incoming requests to your upstream service.

Such as if you have a service running at 'localhost:3000', you can register
endpoint 'my-endpoint' that forwards requests to that local service.

Examples:
  # Register a listener with endpoint ID 'my-endpoing-123' that forwards
  # requests to 'localhost:3000'.
  pico agent --listener my-endpoint-123/localhost:3000

  # Register multiple listeners.
  pico agent --listener my-endpoint-123/localhost:3000 \
      --listener my-endpoint-xyz/localhost:6000

  # Specify the Pico server address.
  pico agent --listener my-endpoint-123/localhost:3000 \
      --server.url https://pico.example.com
`,
	}

	var conf config.Config

	cmd.Flags().StringSliceVar(&conf.Listeners, "listeners", nil, "command separated listeners to register, with format '<endpoint ID>/<forward addr>'")

	cmd.Flags().StringVar(&conf.Log.Level, "log.level", "info", "log level")
	cmd.Flags().StringSliceVar(&conf.Log.Subsystems, "log.subsystems", nil, "enable debug logs for logs the the given subsystems")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		logger, err := log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		run(&conf, logger)
	}

	return cmd
}

func run(conf *config.Config, logger *log.Logger) {
	logger.Info("starting pico agent", zap.Any("conf", conf))

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	g, ctx := errgroup.WithContext(ctx)

	for _, l := range conf.Listeners {
		// Already verified format in Config.Validate.
		elems := strings.Split(l, "/")
		endpointID := elems[0]
		forwardAddr := elems[1]

		listener := agent.NewListener(endpointID, forwardAddr, logger)
		g.Go(func() error {
			return listener.Run(ctx)
		})
	}

	sig := <-c
	logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	cancel()

	if err := g.Wait(); err != nil {
		logger.Error("failed to run agent", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
