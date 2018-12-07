// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/processgroup"
)

// Processes contains list of processes
type Processes struct {
	List []*Process
}

// NewProcesses creates a process-set with satellites and storage nodes
func NewProcesses(dir string, satelliteCount, storageNodeCount int) (*Processes, error) {
	processes := &Processes{}

	for i := 0; i < satelliteCount; i++ {
		name := fmt.Sprintf("satellite/%d", i)

		dir := filepath.Join(dir, "satellite", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process := NewProcess(name, "satellite", dir)
		processes.List = append(processes.List, process)

		process.Arguments["run"] = []string{"run", "--base-path", "."}
		process.Arguments["setup"] = []string{"--base-path", ".", "--overwrite"}
	}

	for i := 0; i < storageNodeCount; i++ {
		name := fmt.Sprintf("storage/%d", i)

		dir := filepath.Join(dir, "storagenode", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process := NewProcess(name, "storagenode", dir)
		processes.List = append(processes.List, process)

		process.Arguments["run"] = []string{"run", "--base-path", "."}
		process.Arguments["setup"] = []string{"--base-path", ".", "--overwrite"}
	}

	return processes, nil
}

// Exec executes a command on all processes
func (processes *Processes) Exec(ctx context.Context, command string) error {
	var group errgroup.Group
	for _, p := range processes.List {
		process := p

		process.Stdout.Hook(os.Stdout)
		process.Stderr.Hook(os.Stderr)

		group.Go(func() error {
			return process.Exec(ctx, command)
		})
	}

	return group.Wait()
}

// Process is a type for monitoring the process
type Process struct {
	Name       string
	Directory  string
	Executable string

	Arguments map[string][]string

	Stdout Buffer
	Stderr Buffer
}

// NewProcess creates a process which can be run in the specified directory
func NewProcess(name, executable, directory string) *Process {
	return &Process{
		Name:       name,
		Directory:  directory,
		Executable: executable,

		Arguments: map[string][]string{},
	}
}

// Exec runs the process using the arguments for a given command
func (process *Process) Exec(ctx context.Context, command string) error {
	cmd := exec.Command(process.Executable, process.Arguments[command]...)
	cmd.Dir = process.Directory
	cmd.Stdout, cmd.Stderr = &process.Stdout, &process.Stderr

	processgroup.Setup(cmd)

	err := cmd.Run()
	return err
}
