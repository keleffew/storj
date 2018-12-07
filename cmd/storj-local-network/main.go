// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

type Config struct {
	Directory string

	SatelliteCount   int
	StorageNodeCount int

	Exec string
}

func main() {
	var config Config

	rootCmd := &cobra.Command{
		Use:   "storj-local-network",
		Short: "Storj Local Network",
	}

	rootCmd.PersistentFlags().StringVarP(&config.Directory, "dir", "", fpath.ApplicationDir("storj", "local-network"), "base project directory")

	rootCmd.PersistentFlags().IntVarP(&config.SatelliteCount, "satellites", "", 1, "number of satellites to start")
	rootCmd.PersistentFlags().IntVarP(&config.StorageNodeCount, "storage-nodes", "", 100, "number of storage nodes to start")

	rootCmd.PersistentFlags().StringVarP(&config.Exec, "exec", "", "", "execute the given program when all the peers have started")

	exec := func(cmd *cobra.Command, args []string, command string) error {
		processes, err := NewProcesses(config.Directory, 1, 100)
		if err != nil {
			return err
		}

		ctx, cleanup := NewCLIContext(context.Background())
		defer cleanup()

		return processes.Exec(ctx, command)
	}

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return exec(cmd, args, "run")
			},
		}, &cobra.Command{
			Use:   "setup",
			Short: "setup peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return exec(cmd, args, "setup")
			},
		},
	)

	process.Exec(rootCmd)
}
