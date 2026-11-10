package plt

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/forklift-run/forklift/internal/app/forklift"
	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-img

func lsImgAction(ctx context.Context, c *cli.Command) error {
	plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
		mergePallet: true,
	})
	if err != nil {
		return err
	}

	images, err := forklift.ListRequiredImages(plt, caches.pmp, c.Bool("include-disabled"))
	if err != nil {
		return err
	}
	for _, image := range images {
		fmt.Println(image)
	}
	return nil
}

// cache-img

func cacheImgAction(versions Versions) cli.ActionFunc {
	return func(ctx context.Context, c *cli.Command) error {
		plt, caches, err := processFullBaseArgs(c.String("workspace"), processingOptions{
			requirePalletCache: true,
			mergePallet:        true,
		})
		if err != nil {
			return err
		}
		if err = fcli.CheckDeepCompat(
			plt, caches.p, versions.Core(), c.Bool("ignore-tool-version"),
		); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "Downloading Docker container images specified by the local pallet...")
		if err := fcli.DownloadImages(
			ctx, 0, plt, caches.pmp,
			c.String("platform"), c.Bool("include-disabled"), c.Bool("parallel"), true,
		); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Done!")
		return nil
	}
}
