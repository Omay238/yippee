package download

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/aur"

	"github.com/Jguer/yippee/v12/pkg/db"
	"github.com/Jguer/yippee/v12/pkg/multierror"
	"github.com/Jguer/yippee/v12/pkg/settings/exe"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
)

type httpRequestDoer interface {
	Get(string) (*http.Response, error)
}

type DBSearcher interface {
	SyncPackage(string) db.IPackage
	SyncPackageFromDB(string, string) db.IPackage
}

func downloadGitRepo(ctx context.Context, cmdBuilder exe.GitCmdBuilder,
	pkgURL, pkgName, dest string, force bool, gitArgs ...string,
) (bool, error) {
	finalDir := filepath.Join(dest, pkgName)
	newClone := true

	switch _, err := os.Stat(filepath.Join(finalDir, ".git")); {
	case os.IsNotExist(err) || (err == nil && force):
		if _, errD := os.Stat(finalDir); force && errD == nil {
			if errR := os.RemoveAll(finalDir); errR != nil {
				return false, ErrGetPKGBUILDRepo{inner: errR, pkgName: pkgName, errOut: ""}
			}
		}

		gitArgs = append(gitArgs, pkgURL, pkgName)

		cloneArgs := make([]string, 0, len(gitArgs)+4)
		cloneArgs = append(cloneArgs, "clone", "--no-progress")
		cloneArgs = append(cloneArgs, gitArgs...)
		cmd := cmdBuilder.BuildGitCmd(ctx, dest, cloneArgs...)

		_, stderr, errCapture := cmdBuilder.Capture(cmd)
		if errCapture != nil {
			return false, ErrGetPKGBUILDRepo{inner: errCapture, pkgName: pkgName, errOut: stderr}
		}
	case err != nil:
		return false, ErrGetPKGBUILDRepo{
			inner:   err,
			pkgName: pkgName,
			errOut:  gotext.Get("error reading %s", filepath.Join(dest, pkgName, ".git")),
		}
	default:
		cmd := cmdBuilder.BuildGitCmd(ctx, filepath.Join(dest, pkgName), "pull", "--rebase", "--autostash")

		_, stderr, errCmd := cmdBuilder.Capture(cmd)
		if errCmd != nil {
			return false, ErrGetPKGBUILDRepo{inner: errCmd, pkgName: pkgName, errOut: stderr}
		}

		newClone = false
	}

	return newClone, nil
}

func getURLName(pkg db.IPackage) string {
	name := pkg.Base()
	if name == "" {
		name = pkg.Name()
	}

	return name
}

func PKGBUILDs(dbExecutor DBSearcher, aurClient aur.QueryClient, httpClient *http.Client,
	logger *text.Logger, targets []string, aurURL string, mode parser.TargetMode,
) (map[string][]byte, error) {
	pkgbuilds := make(map[string][]byte, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		// Probably replaceable by something in query.
		dbName, name, isAUR, toSkip := getPackageUsableName(dbExecutor, aurClient, logger, target, mode)
		if toSkip {
			continue
		}

		sem <- 1

		wg.Add(1)

		go func(target, dbName, pkgName string, aur bool) {
			var (
				err      error
				pkgbuild []byte
			)

			if aur {
				pkgbuild, err = AURPKGBUILD(httpClient, pkgName, aurURL)
			} else {
				pkgbuild, err = ABSPKGBUILD(httpClient, dbName, pkgName)
			}

			if err == nil {
				mux.Lock()
				pkgbuilds[target] = pkgbuild
				mux.Unlock()
			} else {
				errs.Add(err)
			}

			<-sem
			wg.Done()
		}(target, dbName, name, isAUR)
	}

	wg.Wait()

	return pkgbuilds, errs.Return()
}

func PKGBUILDRepos(ctx context.Context, dbExecutor DBSearcher, aurClient aur.QueryClient,
	cmdBuilder exe.GitCmdBuilder, logger *text.Logger,
	targets []string, mode parser.TargetMode, aurURL, dest string, force bool,
) (map[string]bool, error) {
	cloned := make(map[string]bool, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		// Probably replaceable by something in query.
		dbName, name, isAUR, toSkip := getPackageUsableName(dbExecutor, aurClient, logger, target, mode)
		if toSkip {
			continue
		}

		sem <- 1

		wg.Add(1)

		go func(target, dbName, pkgName string, aur bool) {
			var (
				err      error
				newClone bool
			)

			if aur {
				newClone, err = AURPKGBUILDRepo(ctx, cmdBuilder, aurURL, pkgName, dest, force)
			} else {
				newClone, err = ABSPKGBUILDRepo(ctx, cmdBuilder, dbName, pkgName, dest, force)
			}

			progress := 0

			if err != nil {
				errs.Add(err)
			} else {
				mux.Lock()
				cloned[target] = newClone
				progress = len(cloned)
				mux.Unlock()
			}

			if aur {
				logger.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
						progress, len(targets), text.Cyan(pkgName)))
			} else {
				logger.OperationInfoln(
					gotext.Get("(%d/%d) Downloaded PKGBUILD from ABS: %s",
						progress, len(targets), text.Cyan(pkgName)))
			}

			<-sem

			wg.Done()
		}(target, dbName, name, isAUR)
	}

	wg.Wait()

	return cloned, errs.Return()
}

// TODO: replace with dep.ResolveTargets.
func getPackageUsableName(dbExecutor DBSearcher, aurClient aur.QueryClient,
	logger *text.Logger, target string, mode parser.TargetMode,
) (dbname, pkgname string, isAUR, toSkip bool) {
	dbName, name := text.SplitDBFromName(target)
	if dbName != "aur" && mode.AtLeastRepo() {
		var pkg db.IPackage
		if dbName != "" {
			pkg = dbExecutor.SyncPackageFromDB(name, dbName)
		} else {
			pkg = dbExecutor.SyncPackage(name)
		}

		if pkg != nil {
			name = getURLName(pkg)
			dbName = pkg.DB().Name()
			return dbName, name, false, false
		}

		// If the package is not found in the database and it was expected to be
		if pkg == nil && dbName != "" {
			return dbName, name, true, true
		}
	}

	if mode == parser.ModeRepo {
		return dbName, name, true, true
	}

	pkgs, err := aurClient.Get(context.Background(), &aur.Query{
		By:       aur.Name,
		Contains: false,
		Needles:  []string{name},
	})
	if err != nil {
		logger.Warnln(err)
		return dbName, name, true, true
	}

	if len(pkgs) == 0 {
		return dbName, name, true, true
	}

	return "aur", name, true, false
}
