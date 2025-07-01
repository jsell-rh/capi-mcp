package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Command represents a command execution utility with context support
type Command struct {
	cmd    *exec.Cmd
	name   string
	args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// NewCommand creates a new command with the specified name and arguments
func NewCommand(name string, args ...string) *Command {
	return &Command{
		name:   name,
		args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// RunWithContext executes the command with a context for cancellation
func (c *Command) RunWithContext(ctx context.Context) error {
	c.cmd = exec.CommandContext(ctx, c.name, c.args...)
	c.setupCmd()

	fmt.Printf("Executing: %s %s\n", c.name, strings.Join(c.args, " "))

	if err := c.cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}

	return nil
}

// Run executes the command without context (with default timeout)
func (c *Command) Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return c.RunWithContext(ctx)
}

// StartBackground starts the command in the background and returns immediately
func (c *Command) StartBackground() error {
	c.cmd = exec.Command(c.name, c.args...)
	c.setupCmd()

	fmt.Printf("Starting background: %s %s\n", c.name, strings.Join(c.args, " "))

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background command: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}

	return nil
}

// Wait waits for a background command to complete
func (c *Command) Wait() error {
	if c.cmd == nil {
		return fmt.Errorf("command not started")
	}

	if err := c.cmd.Wait(); err != nil {
		return fmt.Errorf("background command failed: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}

	return nil
}

// Kill terminates a background command
func (c *Command) Kill() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	if err := c.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill command: %w", err)
	}

	return nil
}

// RunWithOutput executes the command and returns the output as a string
func (c *Command) RunWithOutput(ctx context.Context) (string, error) {
	c.cmd = exec.CommandContext(ctx, c.name, c.args...)
	c.setupCmd()

	// Override stdout to capture output
	c.cmd.Stdout = nil

	fmt.Printf("Executing (with output): %s %s\n", c.name, strings.Join(c.args, " "))

	output, err := c.cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command failed: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}

	return strings.TrimSpace(string(output)), nil
}

// setupCmd configures the underlying exec.Cmd with the Command's settings
func (c *Command) setupCmd() {
	if c.cmd == nil {
		return
	}

	if c.Env != nil {
		c.cmd.Env = c.Env
	}

	if c.Dir != "" {
		c.cmd.Dir = c.Dir
	}

	if c.Stdin != nil {
		c.cmd.Stdin = c.Stdin
	}

	if c.Stdout != nil {
		c.cmd.Stdout = c.Stdout
	}

	if c.Stderr != nil {
		c.cmd.Stderr = c.Stderr
	}
}
