// Package cache provides subcommands for the local cache
package cache

import (
	"github.com/urfave/cli/v2"
)

const (
	categoryQuery  = "Query the cache"
	categoryModify = "Modify the cache"
)

var Cmd = &cli.Command{
	Name:  "cache",
	Usage: "Manages the local cache of pallets and downloads",
	Subcommands: []*cli.Command{
		{
			Name:     "ls-plt",
			Aliases:  []string{"list-pallets"},
			Category: categoryQuery,
			Usage:    "Lists cached pallets",
			Action:   lsPltAction,
		},
		{
			Name:      "show-plt",
			Aliases:   []string{"show-pallet"},
			Category:  categoryQuery,
			Usage:     "Describes a cached pallet",
			ArgsUsage: "pallet_path@version",
			Action:    showPltAction,
		},
		{
			Name:     "ls-pkg",
			Aliases:  []string{"list-packages"},
			Category: categoryQuery,
			Usage:    "Lists packages offered by cached pallets",
			// TODO: allow only listing packages matching a glob pattern
			Action: lsPkgAction,
		},
		{
			Name:      "show-pkg",
			Aliases:   []string{"show-package"},
			Category:  categoryQuery,
			Usage:     "Describes a cached package",
			ArgsUsage: "package_path@version",
			Action:    showPkgAction,
		},
		{
			Name:     "ls-dl",
			Aliases:  []string{"list-downloads"},
			Category: categoryQuery,
			Usage:    "Lists cached file downloads",
			Action:   lsDlAction,
		},
		{
			Name:     "ls-img",
			Aliases:  []string{"list-images"},
			Category: categoryQuery,
			Usage:    "Lists Docker container images in the local cache",
			// TODO: allow only listing images matching a glob pattern
			Action: lsImgAction,
		},
		{
			Name:      "show-img",
			Aliases:   []string{"show-image"},
			Category:  categoryQuery,
			Usage:     "Describes a cached Docker container image",
			ArgsUsage: "image_sha",
			Action:    showImgAction,
		},
		{
			Name:      "add-plt",
			Aliases:   []string{"add-pallets"},
			Category:  categoryModify,
			Usage:     "Downloads local copies of pallets from remote releases",
			ArgsUsage: "[pallet_path@release]...",
			Action:    addGitRepoAction(getPalletCache),
		},
		{
			Name:     "del-all",
			Aliases:  []string{"delete-all"},
			Category: categoryModify,
			Usage:    "Removes all cached resources",
			Action:   delAllAction,
		},
		{
			Name:     "del-mir",
			Aliases:  []string{"delete-mirrors"},
			Category: categoryModify,
			Usage:    "Removes local mirrors of git repositories",
			// TODO: allow only removing mirrors matching a glob pattern
			Action: delGitRepoAction("mirror", getMirrorCache),
		},
		{
			Name:     "del-plt",
			Aliases:  []string{"delete-pallets"},
			Category: categoryModify,
			Usage:    "Removes locally-cached pallets",
			// TODO: allow only removing pallets matching a glob pattern
			Action: delGitRepoAction("pallet", getPalletCache),
		},
		{
			Name:     "del-dl",
			Aliases:  []string{"delete-downloads"},
			Category: categoryModify,
			Usage:    "Removes locally-cached file downloads",
			Action:   delDlAction,
		},
		{
			Name:     "del-img",
			Aliases:  []string{"delete-images"},
			Category: categoryModify,
			Usage:    "Removes unused Docker container images",
			Action:   delImgAction,
		},
	},
}
