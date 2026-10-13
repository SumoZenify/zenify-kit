package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ZenifyAIContactCenter/zenify-kit/internal/dbread"
	"github.com/ZenifyAIContactCenter/zenify-kit/internal/exitcode"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// newDBReadCmd builds `zenify db-read <setup|collections|tables|doc|count|eval|sql> [arg]`
// — the portable port of ~/.local/bin/db_read (FR-033).
func newDBReadCmd() *cobra.Command {
	return newDBReadCmdWithSetup(promptDBReadSetup)
}

func newDBReadCmdWithSetup(prompt dbReadSetupPrompt) *cobra.Command {
	profile := "staging"
	cmd := &cobra.Command{
		Use:           "db-read <setup|collections|tables|doc|count|eval|sql> [arg]",
		Short:         "Đọc database dùng chung ở chế độ chỉ đọc, không bao giờ ghi và không in secret", //znf:allow-lang
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "setup" {
				if len(args) != 1 {
					return exitcode.New(exitcode.BadArgs, errors.New("zenify db-read setup takes no argument"))
				}
				input, err := prompt(profile, !cmd.Flags().Changed("profile"))
				if err != nil {
					if errors.Is(err, huh.ErrUserAborted) {
						return exitcode.New(exitcode.Cancelled, err)
					}
					return exitcode.New(exitcode.BadArgs, err)
				}
				settingsPath := workspaceSettingsPath()
				configPath, err := dbread.SaveProfile(settingsPath, input)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "db-read profile %q saved\nmetadata: %s\ncredentials: %s\n", input.Name, configPath, settingsPath)
				return nil
			}
			arg := ""
			if len(args) > 1 {
				arg = args[1]
			}
			o := &dbread.Options{
				Cmd:          args[0],
				Arg:          arg,
				Profile:      profile,
				Stdout:       cmd.OutOrStdout(),
				Stderr:       cmd.ErrOrStderr(),
				Env:          os.Getenv,
				SettingsPath: workspaceSettingsPath(),
			}
			o.SetRun(func(name string, args []string, extraEnv []string, _ string) error {
				c := exec.Command(name, args...) //nolint:gosec // G204 -- name is always "mongosh" or "mysql" from this package's own callers, args are internally-computed subcommands, not attacker-controlled shell input
				c.Env = mergeEnv(os.Environ(), extraEnv)
				c.Stdout = o.Stdout
				var childStderr bytes.Buffer
				c.Stderr = &childStderr
				err := c.Run()
				_, _ = o.Stderr.Write(redactEnvValues(childStderr.Bytes(), extraEnv))
				return err
			})
			return dbread.Run(o)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "staging", "database profile: staging, local, develop, custom, or production")
	return cmd
}

func redactEnvValues(output []byte, env []string) []byte {
	redacted := append([]byte(nil), output...)
	for _, assignment := range env {
		_, value, ok := strings.Cut(assignment, "=")
		if ok && value != "" {
			redacted = bytes.ReplaceAll(redacted, []byte(value), []byte("[REDACTED]"))
		}
	}
	return redacted
}

func mergeEnv(base, overrides []string) []string {
	values := make(map[string]string, len(overrides))
	order := make([]string, 0, len(overrides))
	for _, assignment := range overrides {
		key, _, ok := strings.Cut(assignment, "=")
		if !ok {
			continue
		}
		if _, seen := values[key]; !seen {
			order = append(order, key)
		}
		values[key] = assignment
	}
	merged := make([]string, 0, len(base)+len(order))
	for _, assignment := range base {
		key, _, ok := strings.Cut(assignment, "=")
		if ok {
			if _, replaced := values[key]; replaced {
				continue
			}
		}
		merged = append(merged, assignment)
	}
	for _, key := range order {
		merged = append(merged, values[key])
	}
	return merged
}
