// Package inspector provides subcommands for inspecting the state of various things
package inspector

import (
	"github.com/urfave/cli/v3"
)

var Cmd = &cli.Command{
	Name:  "inspector",
	Usage: "Inspects the state of various things",
	Commands: []*cli.Command{
		{
			Name:    "resolve-git-repo",
			Aliases: []string{"resolve-git-repository"},
			Usage: "Prints the version/pseudoversion string for the version query on the specified git " +
				"repo",
			ArgsUsage: "git_repo_path@version_query",
			Action:    resolveGitRepoAction,
		},
	},
}
