package inspector

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v3"

	ffs "github.com/forklift-run/forklift/exp/fs"
	fws "github.com/forklift-run/forklift/exp/workspaces"
	"github.com/forklift-run/forklift/internal/app/forklift"
	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// show-repo-version

func showRepoVersionAction(ctx context.Context, c *cli.Command) error {
	localPath := c.Args().First()
	lock, err := forklift.ResolveVersionQueryUsingRepo(localPath, "")
	if err != nil {
		return errors.Wrapf(err, "couldn't determine version of %s", localPath)
	}
	fmt.Println(lock.Version)
	return nil
}

// resolve-repo

func resolveRepoAction(ctx context.Context, c *cli.Command) error {
	workspace, err := ensureWorkspace(c.String("workspace"))
	if err != nil {
		return err
	}

	query := c.Args().First()
	resolved, err := fcli.ResolveQueriesUsingLocalMirrors(
		0, workspace.GetMirrorCachePath(), []string{query}, true,
	)
	if err != nil {
		return errors.Wrapf(err, "couldn't resolve query %s", query)
	}
	fmt.Println(resolved[query].VersionLock.Version)
	return nil
}

func ensureWorkspace(wpath string) (*fws.FSWorkspace, error) {
	if !ffs.DirExists(wpath) {
		fmt.Fprintf(os.Stderr, "Making a new workspace at %s...", wpath)
	}
	if err := ffs.EnsureExists(wpath); err != nil {
		return nil, errors.Wrapf(err, "couldn't make new workspace at %s", wpath)
	}
	workspace, err := fws.LoadWorkspace(wpath)
	if err != nil {
		return nil, err
	}
	if err = ffs.EnsureExists(workspace.GetDataPath()); err != nil {
		return nil, errors.Wrapf(err, "couldn't ensure the existence of %s", workspace.GetDataPath())
	}
	return workspace, nil
}
