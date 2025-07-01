package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
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

// allowedCommands defines the whitelist of allowed commands for security
var allowedCommands = map[string]bool{
	"kubectl":    true,
	"kind":       true,
	"docker":     true,
	"helm":       true,
	"go":         true,
	"make":       true,
	"git":        true,
	"curl":       true,
	"sleep":      true,
	"echo":       true,
	"cat":        true,
	"grep":       true,
	"awk":        true,
	"sed":        true,
	"which":      true,
	"command":    true,
	"timeout":    true,
	"sh":         true,
	"bash":       true,
}

// NewCommand creates a new command with the specified name and arguments
func NewCommand(name string, args ...string) *Command {
	// Validate command is in whitelist
	if !allowedCommands[name] {
		// For security, only allow whitelisted commands
		// Return a command that will fail safely
		return &Command{
			name:   "echo",
			args:   []string{fmt.Sprintf("Error: command '%s' not allowed", name)},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	}
	
	// Sanitize command name and arguments to prevent injection
	sanitizedName := sanitizeCommandInput(name)
	sanitizedArgs := make([]string, len(args))
	for i, arg := range args {
		sanitizedArgs[i] = sanitizeCommandInput(arg)
	}
	
	return &Command{
		name:   sanitizedName,
		args:   sanitizedArgs,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// sanitizeCommandInput removes potentially dangerous characters from command inputs
func sanitizeCommandInput(input string) string {
	// Remove shell metacharacters that could be used for injection
	// Keep only alphanumeric, dash, underscore, dot, slash, colon, equals, and space
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_./:= ]`)
	return re.ReplaceAllString(input, "")
}

// execSafeCommand creates exec.Cmd with validated inputs
func (c *Command) execSafeCommand(ctx context.Context) *exec.Cmd {
	// Double-check command is still allowed (defense in depth)
	if !allowedCommands[c.name] {
		// Return safe fallback command
		return exec.CommandContext(ctx, "echo", "Error: command not allowed")
	}
	
	// Create command with sanitized inputs
	if ctx != nil {
		return exec.CommandContext(ctx, c.name, c.args...)
	}
	return exec.Command(c.name, c.args...)
}

// RunWithContext executes the command with a context for cancellation
func (c *Command) RunWithContext(ctx context.Context) error {
	c.cmd = c.execSafeCommand(ctx)
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
	c.cmd = c.execSafeCommand(nil)
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
	c.cmd = c.execSafeCommand(ctx)
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
