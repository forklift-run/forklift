package plt

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-imp

func lsImpAction(ctx context.Context, c *cli.Command) error {
	plt, err := getShallowPallet(c.String("cwd"))
	if err != nil {
		return err
	}

	return fcli.FprintPalletImports(0, os.Stdout, plt)
}

// show-imp

func showImpAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(ctx, c, processingOptions{
		requirePalletCache: true,
		enableOverrides:    true,
	})
	if err != nil {
		return err
	}

	importName := c.Args().First()
	return fcli.FprintImportInfo(0, os.Stdout, plt, caches.p, importName)
}
