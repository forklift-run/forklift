package bundling

import (
	_ "embed"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	ffs "github.com/forklift-run/forklift/exp/fs"
	fpkg "github.com/forklift-run/forklift/exp/packaging"
	fplt "github.com/forklift-run/forklift/exp/pallets"
)

// packagesDirName is the name of the directory containing bundled files for each package.
const packagesDirName = "packages"

// FSBundle: Packages

func (b *FSBundle) getPackagesPath() string {
	return path.Join(b.FS.Path(), packagesDirName)
}

func (b *FSBundle) WriteRepoDefFile() error {
	if err := ffs.EnsureExists(b.getPackagesPath()); err != nil {
		return err
	}
	const perm = 0o644
	return os.WriteFile(
		filepath.FromSlash(path.Join(b.getPackagesPath(), "forklift-repository.yml")),
		[]byte(repoDefFileString), perm,
	)
}

//go:embed deprecated.forklift-repository.yml
var repoDefFileString string

// FSBundle: FSPkgLoader

func (b *FSBundle) LoadFSPkg(pkgPath string, version string) (*fpkg.FSPkg, error) {
	if b == nil {
		return nil, errors.New("bundle is nil")
	}

	return b.FSPkgTree.LoadFSPkg(strings.TrimLeft(pkgPath, "/"))
}

func (b *FSBundle) LoadFSPkgs(searchPattern string) ([]*fpkg.FSPkg, error) {
	if b == nil {
		return nil, errors.New("bundle is nil")
	}

	return b.FSPkgTree.LoadFSPkgs(searchPattern)
}

// FSBundle: PkgReqLoader

func (b *FSBundle) LoadPkgReq(pkgPath string) (r fplt.PkgReq, err error) {
	return fplt.PkgReq{
		PkgSubdir: strings.TrimLeft(pkgPath, "/"),
	}, nil
}
