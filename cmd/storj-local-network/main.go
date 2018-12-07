// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

type Flags struct {
	Directory string

	SatelliteCount   int
	StorageNodeCount int
	Identities       int
}

func main() {
	var flags Flags

	rootCmd := &cobra.Command{
		Use:   "storj-local-network",
		Short: "Storj Local Network",
	}

	rootCmd.PersistentFlags().StringVarP(&flags.Directory, "dir", "", fpath.ApplicationDir("storj", "local-network"), "base project directory")

	rootCmd.PersistentFlags().IntVarP(&flags.SatelliteCount, "satellites", "", 1, "number of satellites to start")
	rootCmd.PersistentFlags().IntVarP(&flags.StorageNodeCount, "storage-nodes", "", 10, "number of storage nodes to start")
	rootCmd.PersistentFlags().IntVarP(&flags.Identities, "identities", "", 10, "number of identities to create")

	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return runProcesses(&flags, args, "run")
			},
		}, &cobra.Command{
			Use:   "setup",
			Short: "setup peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return runProcesses(&flags, args, "setup")
			},
		}, &cobra.Command{
			Use:   "testplanet <command>",
			Short: "run command with a testplanet",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return runTestPlanet(&flags, args)
			},
		},
	)

	process.Exec(rootCmd)
}

func runProcesses(flags *Flags, args []string, command string) error {
	processes, err := NewProcesses(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	return processes.Exec(ctx, command)
}
