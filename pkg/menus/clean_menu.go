// Clean Build Menu functions
package menus

import (
	"context"
	"io"
	"os"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yippee/v12/pkg/runtime"
	"github.com/Jguer/yippee/v12/pkg/settings"
	"github.com/Jguer/yippee/v12/pkg/text"
)

func anyExistInCache(pkgbuildDirs map[string]string) bool {
	for _, dir := range pkgbuildDirs {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func CleanFn(ctx context.Context, run *runtime.Runtime, w io.Writer,
	pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
) error {
	if len(pkgbuildDirsByBase) == 0 {
		return nil // no work to do
	}

	if !anyExistInCache(pkgbuildDirsByBase) {
		return nil
	}

	skipFunc := func(pkg string) bool {
		dir := pkgbuildDirsByBase[pkg]
		// TOFIX: new install engine dir will always exist, check if unclean instead
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return true
		}

		return false
	}

	bases := make([]string, 0, len(pkgbuildDirsByBase))
	for pkg := range pkgbuildDirsByBase {
		bases = append(bases, pkg)
	}

	toClean, errClean := selectionMenu(run.Logger, pkgbuildDirsByBase, bases, installed,
		gotext.Get("Packages to cleanBuild?"),
		settings.NoConfirm, run.Cfg.AnswerClean, skipFunc)
	if errClean != nil {
		return errClean
	}

	for i, base := range toClean {
		dir := pkgbuildDirsByBase[base]
		run.Logger.OperationInfoln(gotext.Get("Deleting (%d/%d): %s", i+1, len(toClean), text.Cyan(dir)))

		if err := run.CmdBuilder.Show(run.CmdBuilder.BuildGitCmd(ctx, dir, "reset", "--hard", "origin/HEAD")); err != nil {
			run.Logger.Warnln(gotext.Get("Unable to clean:"), dir)

			return err
		}

		if err := run.CmdBuilder.Show(run.CmdBuilder.BuildGitCmd(ctx, dir, "clean", "-fdx")); err != nil {
			run.Logger.Warnln(gotext.Get("Unable to clean:"), dir)

			return err
		}
	}

	return nil
}
