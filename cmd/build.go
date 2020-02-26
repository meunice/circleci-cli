package cmd

import (
	"github.com/CircleCI-Public/circleci-cli/local"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func addFlagsForDocumentation(flags *pflag.FlagSet) {
	// These options are purely here to retain a mock of the structure of the flags used by `build`.
	// They don't reflect the entire structure or available flags, only those which are public in the original command.
	flags.StringP("config", "c", local.DefaultConfigPath, "config file")
	flags.String("job", "build", "job to be executed")
	flags.Int("node-total", 1, "total number of parallel nodes")
	flags.Int("index", 0, "node index of parallelism")
	flags.Bool("skip-checkout", true, "use local path as-is")
	flags.StringSliceP("volume", "v", nil, "Volume bind-mounting")
	flags.String("checkout-key", "~/.ssh/id_rsa", "Git Checkout key")
	flags.String("revision", "", "Git Revision")
	flags.String("branch", "", "Git branch")
	flags.String("repo-url", "", "Git Url")
	flags.StringArrayP("env", "e", nil, "Set environment variables, e.g. `-e VAR=VAL`")
}

func newLocalExecuteCommand(config *settings.Config) *cobra.Command {
	opts := local.BuildOptions{
		Cfg: config,
	}

	buildCommand := &cobra.Command{
		Use:   "execute",
		Short: "Run a job in a container on the local machine",
		PreRun: func(cmd *cobra.Command, args []string) {
			opts.Args = args
			opts.Help = cmd.Help
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return local.Execute(opts)
		},
		DisableFlagParsing: true,
	}

	// Used as a convenience work-around when DisableFlagParsing is enabled
	// Allows help command to access the combined rollup of flags
	addFlagsForDocumentation(buildCommand.Flags())

	return buildCommand
}

func newBuildCommand(config *settings.Config) *cobra.Command {
	cmd := newLocalExecuteCommand(config)
	cmd.Hidden = true
	cmd.Use = "build"
	return cmd
}

func newLocalCommand(config *settings.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Debug jobs on the local machine",
	}
	cmd.AddCommand(newLocalExecuteCommand(config))
	return cmd
}
