package plt

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-feat

func lsFeatAction(ctx context.Context, c *cli.Command) error {
	plt, _, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintPalletFeatures(0, os.Stdout, plt)
}

// show-feat

func showFeatAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	return fcli.FprintFeatureInfo(0, os.Stdout, plt, caches.p, c.Args().First())
}
