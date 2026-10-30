package cache

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"

	"github.com/forklift-run/forklift/exp/caching"
	fpkg "github.com/forklift-run/forklift/exp/packaging"
	fcli "github.com/forklift-run/forklift/internal/app/forklift/cli"
)

// ls-pkg

func lsPkgAction(c *cli.Context) error {
	cache, err := getPalletCache(c.String("workspace"), false)
	if err != nil {
		return err
	}
	if !cache.Exists() {
		return errMissingCache
	}
	mergedCache := caching.NewMergedFSPalletCache(cache)

	// TODO: add a --pattern cli flag for the pattern
	pkgs, err := mergedCache.LoadFSPkgs("**")
	if err != nil {
		return errors.Wrapf(err, "couldn't identify packages")
	}
	slices.SortFunc(pkgs, fpkg.CompareFSPkgs)
	for _, pkg := range pkgs {
		fmt.Printf("%s@%s\n", pkg.Path(), pkg.FSPkgTree.Version)
	}
	return nil
}

// show-pkg

func showPkgAction(c *cli.Context) error {
	cache, err := getPalletCache(c.String("workspace"), false)
	if err != nil {
		return err
	}
	if !cache.Exists() {
		return errMissingCache
	}
	mergedCache := caching.NewMergedFSPalletCache(cache)

	versionedPkgPath := c.Args().First()
	pkgPath, version, ok := strings.Cut(versionedPkgPath, "@")
	if !ok {
		return errors.Errorf(
			"Couldn't parse package query %s as package_path@version", versionedPkgPath,
		)
	}
	pkg, err := mergedCache.LoadFSPkg(pkgPath, version)
	if err != nil {
		return errors.Wrapf(err, "couldn't resolve package query %s@%s", pkgPath, version)
	}
	fcli.FprintPkg(0, os.Stdout, mergedCache, pkg)
	return nil
}
