package cache

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/forklift-run/forklift/exp/caching"
	fplt "github.com/forklift-run/forklift/exp/pallets"
	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-plt

func lsPltAction(ctx context.Context, c *cli.Command) error {
	cache, err := getPalletCache(c.String("workspace"), false)
	if err != nil {
		return err
	}
	if !cache.Exists() {
		return errMissingCache
	}

	// TODO: add a --pattern cli flag for the pattern
	return lsGitRepo("pallet", "**", cache.LoadFSPallets, func(r, s *fplt.FSPallet) int {
		return fplt.ComparePallets(r.Pallet, s.Pallet)
	})
}

// show-plt

func showPltAction(ctx context.Context, c *cli.Command) error {
	cache, err := getPalletCache(c.String("workspace"), false)
	if err != nil {
		return err
	}
	if !cache.Exists() {
		return errMissingCache
	}
	mergedCache := caching.NewMergedFSPalletCache(cache)

	return showGitRepo(
		os.Stdout, mergedCache, c.Args().First(), mergedCache.LoadFSPallet,
		fcli.FprintCachedPallet, true,
	)
}
