package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Jguer/aur"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yippee/v12/pkg/download"
	"github.com/Jguer/yippee/v12/pkg/runtime"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
)

// yippee -Gp.
func printPkgbuilds(dbExecutor download.DBSearcher, aurClient aur.QueryClient,
	httpClient *http.Client, logger *text.Logger, targets []string,
	mode parser.TargetMode, aurURL string,
) error {
	pkgbuilds, err := download.PKGBUILDs(dbExecutor, aurClient, httpClient, logger, targets, aurURL, mode)
	if err != nil {
		logger.Errorln(err)
	}

	for target, pkgbuild := range pkgbuilds {
		logger.Printf("\n\n# %s\n\n%s", target, string(pkgbuild))
	}

	if len(pkgbuilds) != len(targets) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := pkgbuilds[target]; !ok {
				missing = append(missing, target)
			}
		}

		logger.Warnln(gotext.Get("Unable to find the following packages:"), " ", strings.Join(missing, ", "))

		return fmt.Errorf("")
	}

	return nil
}

// yippee -G.
func getPkgbuilds(ctx context.Context, dbExecutor download.DBSearcher, aurClient aur.QueryClient,
	run *runtime.Runtime, targets []string, force bool,
) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cloned, errD := download.PKGBUILDRepos(ctx, dbExecutor, aurClient,
		run.CmdBuilder, run.Logger, targets, run.Cfg.Mode, run.Cfg.AURURL, wd, force)
	if errD != nil {
		run.Logger.Errorln(errD)
	}

	if len(targets) != len(cloned) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := cloned[target]; !ok {
				missing = append(missing, target)
			}
		}

		run.Logger.Warnln(gotext.Get("Unable to find the following packages:"), " ", strings.Join(missing, ", "))

		err = fmt.Errorf("")
	}

	return err
}
