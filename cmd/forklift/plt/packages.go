package plt

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-pkg

func lsPkgAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintPalletPkgs(0, os.Stdout, plt, caches.pmp)
}

// locate-pkg

func locatePkgAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintPkgLocation(os.Stdout, plt, caches.pmp, c.Args().First())
}

// show-pkg

func showPkgAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintPkgInfo(0, os.Stdout, plt, caches.pmp, c.Args().First())
}
