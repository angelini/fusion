package cmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type configKey string

var logKey = configKey("log")

func NewCmdRoot() *cobra.Command {
	var level *zapcore.Level

	cmd := &cobra.Command{
		Use: "fusion",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			config := zap.NewDevelopmentConfig()
			config.Level = zap.NewAtomicLevelAt(*level)

			log, err := config.Build()
			if err != nil {
				return fmt.Errorf("cannot build zap logger: %w", err)
			}

			ctx := cmd.Context()
			cmd.SetContext(context.WithValue(ctx, logKey, log))

			return nil
		},
	}

	cmd.AddCommand(NewCmdManager())
	cmd.AddCommand(NewCmdPodProxy())
	cmd.AddCommand(NewCmdSandbox())
	cmd.AddCommand(NewCmdDebug())

	level = zap.LevelFlag("log-level", zap.DebugLevel, "Log level")
	cmd.PersistentFlags().AddGoFlag(flag.CommandLine.Lookup("log-level"))

	return cmd
}

func Execute() error {
	ctx := context.Background()
	return NewCmdRoot().ExecuteContext(ctx)
}
