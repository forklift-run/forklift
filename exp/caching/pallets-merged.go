package caching

import (
	"fmt"
	"maps"
	"path"
	"strings"

	"github.com/pkg/errors"

	ffs "github.com/forklift-run/forklift/exp/fs"
	fpkg "github.com/forklift-run/forklift/exp/packaging"
	fplt "github.com/forklift-run/forklift/exp/pallets"
	"github.com/forklift-run/forklift/exp/structures"
)

// MergedFSPalletCache is a [PathedPalletCache] implementation with copies of pallets stored in a
// [fpkg.PathedFS] filesystem.
type MergedFSPalletCache struct {
	shallowCache  PathedPalletCache
	mergedPallets map[string]*fplt.FSPallet
	mergedPkgTree *fpkg.FSPkgTree
}

// MergedFSPalletCache

func NewMergedFSPalletCache(shallowCache PathedPalletCache) *MergedFSPalletCache {
	return &MergedFSPalletCache{
		shallowCache:  shallowCache,
		mergedPallets: make(map[string]*fplt.FSPallet),
		mergedPkgTree: &fpkg.FSPkgTree{
			// FIXME: initialize an FS whose children correspond to the keys of mergedPallets
		},
	}
}

// MergedFSPalletCache: Pather

// Path returns the path of the cache's filesystem.
func (c *MergedFSPalletCache) Path() string {
	return c.shallowCache.Path()
}

// MergedFSPalletCache: FSPalletLoader

// LoadFSPallet loads the FSPallet with the specified path and version.
// The loaded FSPallet instance is fully initialized and merged.
func (c *MergedFSPalletCache) LoadFSPallet(pltPath string, version string) (*fplt.FSPallet, error) {
	if c == nil {
		return nil, errors.New("cache is nil")
	}

	versionedPath := fmt.Sprintf("%s@%s", pltPath, version)
	if mergedPlt, ok := c.mergedPallets[versionedPath]; ok {
		return mergedPlt, nil
	}

	shallowPlt, err := c.shallowCache.LoadFSPallet(pltPath, version)
	if err != nil {
		return nil, errors.Wrapf(
			err, "couldn't load pre-merge pallet %s@%s from cache", pltPath, version,
		)
	}
	prohibited := make(structures.Set[string])
	mergedPlt, mergedPlts, err := fplt.MergeFSPallet(shallowPlt, c.shallowCache, prohibited)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't merge pallet %s@%s", pltPath, version)
	}
	maps.Copy(c.mergedPallets, mergedPlts)
	c.mergedPallets[versionedPath] = mergedPlt
	return mergedPlt, nil
}

// LoadFSPallets loads all FSPallets from the cache matching the specified search pattern.
// The search pattern should be a [doublestar] pattern, such as `**`, matching pallet directories to
// search for.
// The loaded FSPallet instances are fully initialized and merged.
func (c *MergedFSPalletCache) LoadFSPallets(searchPattern string) ([]*fplt.FSPallet, error) {
	if c == nil {
		return nil, nil
	}

	shallowPlts, err := c.shallowCache.LoadFSPallets(searchPattern)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't load pre-merge pallets from cache")
	}

	mergedPlts := make([]*fplt.FSPallet, 0, len(shallowPlts))
	for _, shallowPlt := range shallowPlts {
		mergedPlt, err := c.LoadFSPallet(shallowPlt.Path(), shallowPlt.Version)
		if err != nil {
			return nil, errors.Wrapf(
				err, "couldn't load merged pallet %s@%s", shallowPlt.Path(), shallowPlt.Version,
			)
		}
		mergedPlts = append(mergedPlts, mergedPlt)
	}

	return mergedPlts, nil
}

// MergedFSPalletCache: FSPkgLoader

// LoadFSPkg loads the FSPkg with the specified path and version.
// The loaded FSPkg instance is fully initialized.
func (c *MergedFSPalletCache) LoadFSPkg(pkgPath string, version string) (*fpkg.FSPkg, error) {
	if c == nil {
		return nil, errors.New("cache is nil")
	}

	// Search for the package by starting with the shortest possible package subdirectory path and the
	// longest possible pkg tree path, and shifting path components from the pkg tree path to the package
	// subdirectory path until we successfully load the package.
	palletPath := path.Dir(pkgPath)
	pkgSubdir := path.Base(pkgPath)
	for palletPath != "." && palletPath != "/" {
		pallet, err := c.LoadFSPallet(palletPath, version)
		if err != nil {
			pkgSubdir = path.Join(path.Base(palletPath), pkgSubdir)
			palletPath = path.Dir(palletPath)
			continue
		}

		pkg, err := pallet.LoadFSPkg(pkgSubdir)
		if err != nil {
			return nil, errors.Wrapf(
				err, "couldn't load package %s from merged pallet %s at version %s",
				pkgPath, palletPath, version,
			)
		}
		return pkg, nil
	}
	return nil, errors.Errorf("no cached packages were found matching %s@%s", pkgPath, version)
}

// LoadFSPkgs loads all FSPkgs from the cache matching the specified search pattern.
// The search pattern should be a [doublestar] pattern, such as `**`, matching package directories
// to search for.
// The loaded FSPkg instances are fully initialized.
func (c *MergedFSPalletCache) LoadFSPkgs(searchPattern string) ([]*fpkg.FSPkg, error) {
	if c == nil {
		return nil, nil
	}

	if _, err := c.LoadFSPallets("**"); err != nil {
		return nil, errors.Wrap(err, "couldn't load all merged pallets to search for packages in them")
	}

	pkgs, err := c.mergedPkgTree.LoadFSPkgs(searchPattern)
	if err != nil {
		return nil, err
	}

	for _, pkg := range pkgs {
		pallet, err := c.loadFSPalletContaining(ffs.GetSubdirPath(c, pkg.FS.Path()))
		if err != nil {
			return nil, errors.Wrapf(
				err, "couldn't find the cached pallet providing the cached package at %s", pkg.FS.Path(),
			)
		}
		if err = pkg.AttachFSPkgTree(pallet.FSPkgTree); err != nil {
			return nil, errors.Wrap(err, "couldn't attach cached pallet to cached package")
		}
	}
	return pkgs, nil
}

// loadFSPalletContaining finds, loads, and merges the FSPallet which contains the provided
// subdirectory path.
func (c *MergedFSPalletCache) loadFSPalletContaining(
	subdirPath string,
) (pallet *fplt.FSPallet, err error) {
	if c == nil {
		return nil, errors.New("cache is nil")
	}

	if pallet, err = loadFSPalletContaining(c.mergedPkgTree.FS, subdirPath); err != nil {
		return nil, errors.Wrapf(err, "couldn't find any pallet containing %s", subdirPath)
	}
	var palletPath string
	var ok bool
	if palletPath, pallet.Version, ok = strings.Cut(ffs.GetSubdirPath(c, pallet.FS.Path()), "@"); !ok {
		return nil, errors.Wrapf(
			err, "couldn't parse path of cached pallet configured at %s as pallet_path@version",
			pallet.FS.Path(),
		)
	}
	pallet.FSPkgTree.Version = pallet.Version
	if palletPath != pallet.Path() {
		return nil, errors.Errorf(
			"cached pallet %s is in cache at %s@%s instead of %s@%s",
			pallet.Path(), palletPath, pallet.Version, pallet.Path(), pallet.Version,
		)
	}
	return pallet, nil
}
