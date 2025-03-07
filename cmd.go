package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yippee/v12/pkg/completion"
	"github.com/Jguer/yippee/v12/pkg/db"
	"github.com/Jguer/yippee/v12/pkg/download"
	"github.com/Jguer/yippee/v12/pkg/intrange"
	"github.com/Jguer/yippee/v12/pkg/news"
	"github.com/Jguer/yippee/v12/pkg/query"
	"github.com/Jguer/yippee/v12/pkg/runtime"
	"github.com/Jguer/yippee/v12/pkg/settings"
	"github.com/Jguer/yippee/v12/pkg/settings/exe"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
	"github.com/Jguer/yippee/v12/pkg/upgrade"
	"github.com/Jguer/yippee/v12/pkg/vcs"
)

func usage(logger *text.Logger) {
	logger.Println(`Usage:
    yippee
    yippee <operation> [...]
    yippee <package(s)>

operations:
    yippee {-h --help}
    yippee {-V --version}
    yippee {-D --database}    <options> <package(s)>
    yippee {-F --files}       [options] [package(s)]
    yippee {-Q --query}       [options] [package(s)]
    yippee {-R --remove}      [options] <package(s)>
    yippee {-S --sync}        [options] [package(s)]
    yippee {-T --deptest}     [options] [package(s)]
    yippee {-U --upgrade}     [options] <file(s)>

New operations:
    yippee {-B --build}       [options] [dir]
    yippee {-G --getpkgbuild} [options] [package(s)]
    yippee {-P --show}        [options]
    yippee {-W --web}         [options] [package(s)]
    yippee {-Y --yippee}         [options] [package(s)]

If no operation is specified 'yippee -Syu' will be performed
If no operation is specified and targets are provided -Y will be assumed

New options:
       --repo             Assume targets are from the repositories
    -a --aur              Assume targets are from the AUR

Permanent configuration options:
    --save                Causes the following options to be saved back to the
                          config file when used

    --aururl      <url>   Set an alternative AUR URL
    --aurrpcurl   <url>   Set an alternative URL for the AUR /rpc endpoint
    --builddir    <dir>   Directory used to download and run PKGBUILDS
    --editor      <file>  Editor to use when editing PKGBUILDs
    --editorflags <flags> Pass arguments to editor
    --makepkg     <file>  makepkg command to use
    --mflags      <flags> Pass arguments to makepkg
    --pacman      <file>  pacman command to use
    --git         <file>  git command to use
    --gitflags    <flags> Pass arguments to git
    --gpg         <file>  gpg command to use
    --gpgflags    <flags> Pass arguments to gpg
    --config      <file>  pacman.conf file to use
    --makepkgconf <file>  makepkg.conf file to use
    --nomakepkgconf       Use the default makepkg.conf

    --requestsplitn <n>   Max amount of packages to query per AUR request
    --completioninterval  <n> Time in days to refresh completion cache
    --sortby    <field>   Sort AUR results by a specific field during search
    --searchby  <field>   Search for packages using a specified field
    --answerclean   <a>   Set a predetermined answer for the clean build menu
    --answerdiff    <a>   Set a predetermined answer for the diff menu
    --answeredit    <a>   Set a predetermined answer for the edit pkgbuild menu
    --answerupgrade <a>   Set a predetermined answer for the upgrade menu
    --noanswerclean       Unset the answer for the clean build menu
    --noanswerdiff        Unset the answer for the edit diff menu
    --noansweredit        Unset the answer for the edit pkgbuild menu
    --noanswerupgrade     Unset the answer for the upgrade menu
    --cleanmenu           Give the option to clean build PKGBUILDS
    --diffmenu            Give the option to show diffs for build files
    --editmenu            Give the option to edit/view PKGBUILDS
    --askremovemake       Ask to remove makedepends after install
    --askyesremovemake    Ask to remove makedepends after install("Y" as default)
    --removemake          Remove makedepends after install
    --noremovemake        Don't remove makedepends after install

    --cleanafter          Remove package sources after successful install
    --keepsrc             Keep pkg/ and src/ after building packages
    --bottomup            Shows AUR's packages first and then repository's
    --topdown             Shows repository's packages first and then AUR's
    --singlelineresults   List each search result on its own line
    --doublelineresults   List each search result on two lines, like pacman

    --devel               Check development packages during sysupgrade
    --rebuild             Always build target packages
    --rebuildall          Always build all AUR packages
    --norebuild           Skip package build if in cache and up to date
    --rebuildtree         Always build all AUR packages even if installed
    --redownload          Always download pkgbuilds of targets
    --noredownload        Skip pkgbuild download if in cache and up to date
    --redownloadall       Always download pkgbuilds of all AUR packages
    --provides            Look for matching providers when searching for packages
    --pgpfetch            Prompt to import PGP keys from PKGBUILDs
    --useask              Automatically resolve conflicts using pacman's ask flag

    --sudo                <file>  sudo command to use
    --sudoflags           <flags> Pass arguments to sudo
    --sudoloop            Loop sudo calls in the background to avoid timeout

    --timeupdate          Check packages' AUR page for changes during sysupgrade

show specific options:
    -c --complete         Used for completions
    -d --defaultconfig    Print default yippee configuration
    -g --currentconfig    Print current yippee configuration
    -s --stats            Display system package statistics
    -w --news             Print arch news

yippee specific options:
    -c --clean            Remove unneeded dependencies
       --gendb            Generates development package DB used for updating

getpkgbuild specific options:
    -f --force            Force download for existing ABS packages
    -p --print            Print pkgbuild of packages`)
}

func handleCmd(ctx context.Context, run *runtime.Runtime,
	cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	if cmdArgs.ExistsArg("h", "help") {
		return handleHelp(ctx, run, cmdArgs)
	}

	if run.Cfg.SudoLoop && cmdArgs.NeedRoot(run.Cfg.Mode) {
		run.CmdBuilder.SudoLoop()
	}

	switch cmdArgs.Op {
	case "V", "version":
		handleVersion(run.Logger)
		return nil
	case "D", "database":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "F", "files":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "Q", "query":
		return handleQuery(ctx, run, cmdArgs, dbExecutor)
	case "R", "remove":
		return handleRemove(ctx, run, cmdArgs, run.VCSStore)
	case "S", "sync":
		return handleSync(ctx, run, cmdArgs, dbExecutor)
	case "T", "deptest":
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case "U", "upgrade":
		return handleUpgrade(ctx, run, cmdArgs)
	case "B", "build":
		return handleBuild(ctx, run, dbExecutor, cmdArgs)
	case "G", "getpkgbuild":
		return handleGetpkgbuild(ctx, run, cmdArgs, dbExecutor)
	case "P", "show":
		return handlePrint(ctx, run, cmdArgs, dbExecutor)
	case "Y", "yippee":
		return handleYippee(ctx, run, cmdArgs, run.CmdBuilder,
			dbExecutor, run.QueryBuilder)
	case "W", "web":
		return handleWeb(ctx, run, cmdArgs)
	}

	return errors.New(gotext.Get("unhandled operation"))
}

// getFilter returns filter function which can keep packages which were only
// explicitly installed or ones installed as dependencies for showing available
// updates or their count.
func getFilter(cmdArgs *parser.Arguments) (upgrade.Filter, error) {
	deps, explicit := cmdArgs.ExistsArg("d", "deps"), cmdArgs.ExistsArg("e", "explicit")

	switch {
	case deps && explicit:
		return nil, errors.New(gotext.Get("invalid option: '--deps' and '--explicit' may not be used together"))
	case deps:
		return func(pkg *upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonDepend
		}, nil
	case explicit:
		return func(pkg *upgrade.Upgrade) bool {
			return pkg.Reason == alpm.PkgReasonExplicit
		}, nil
	}

	return func(pkg *upgrade.Upgrade) bool {
		return true
	}, nil
}

func handleQuery(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	if cmdArgs.ExistsArg("u", "upgrades") {
		filter, err := getFilter(cmdArgs)
		if err != nil {
			return err
		}

		return printUpdateList(ctx, run, cmdArgs, dbExecutor,
			cmdArgs.ExistsDouble("u", "sysupgrade"), filter)
	}

	if err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm)); err != nil {
		if str := err.Error(); strings.Contains(str, "exit status") {
			// yippee -Qdt should not output anything in case of error
			return fmt.Errorf("")
		}

		return err
	}

	return nil
}

func handleHelp(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments) error {
	usage(run.Logger)
	switch cmdArgs.Op {
	case "Y", "yippee", "G", "getpkgbuild", "P", "show", "W", "web", "B", "build":
		return nil
	}

	run.Logger.Println("\npacman operation specific options:")
	return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
}

func handleVersion(logger *text.Logger) {
	logger.Printf("yippee v%s - libalpm v%s\n", yippeeVersion, alpm.Version())
}

func handlePrint(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	switch {
	case cmdArgs.ExistsArg("d", "defaultconfig"):
		tmpConfig := settings.DefaultConfig(yippeeVersion)
		run.Logger.Printf("%v", tmpConfig)

		return nil
	case cmdArgs.ExistsArg("g", "currentconfig"):
		run.Logger.Printf("%v", run.Cfg)

		return nil
	case cmdArgs.ExistsArg("w", "news"):
		double := cmdArgs.ExistsDouble("w", "news")
		quiet := cmdArgs.ExistsArg("q", "quiet")

		return news.PrintNewsFeed(ctx, run.HTTPClient, run.Logger,
			dbExecutor.LastBuildTime(), run.Cfg.BottomUp, double, quiet)
	case cmdArgs.ExistsArg("c", "complete"):
		return completion.Show(ctx, run.HTTPClient, dbExecutor,
			run.Cfg.AURURL, run.Cfg.CompletionPath, run.Cfg.CompletionInterval, cmdArgs.ExistsDouble("c", "complete"))
	case cmdArgs.ExistsArg("s", "stats"):
		return localStatistics(ctx, run, dbExecutor)
	}

	return nil
}

func handleYippee(ctx context.Context, run *runtime.Runtime,
	cmdArgs *parser.Arguments, cmdBuilder exe.ICmdBuilder,
	dbExecutor db.Executor, queryBuilder query.Builder,
) error {
	switch {
	case cmdArgs.ExistsArg("gendb"):
		return createDevelDB(ctx, run, dbExecutor)
	case cmdArgs.ExistsDouble("c"):
		return cleanDependencies(ctx, run.Cfg, cmdBuilder, cmdArgs, dbExecutor, true)
	case cmdArgs.ExistsArg("c", "clean"):
		return cleanDependencies(ctx, run.Cfg, cmdBuilder, cmdArgs, dbExecutor, false)
	case len(cmdArgs.Targets) > 0:
		return displayNumberMenu(ctx, run, cmdArgs.Targets, dbExecutor, queryBuilder, cmdArgs)
	}

	return nil
}

func handleWeb(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments) error {
	switch {
	case cmdArgs.ExistsArg("v", "vote"):
		return handlePackageVote(ctx, cmdArgs.Targets, run.AURClient, run.Logger,
			run.VoteClient, true)
	case cmdArgs.ExistsArg("u", "unvote"):
		return handlePackageVote(ctx, cmdArgs.Targets, run.AURClient, run.Logger,
			run.VoteClient, false)
	}

	return nil
}

func handleGetpkgbuild(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor download.DBSearcher) error {
	if cmdArgs.ExistsArg("p", "print") {
		return printPkgbuilds(dbExecutor, run.AURClient,
			run.HTTPClient, run.Logger, cmdArgs.Targets, run.Cfg.Mode, run.Cfg.AURURL)
	}

	return getPkgbuilds(ctx, dbExecutor, run.AURClient, run,
		cmdArgs.Targets, cmdArgs.ExistsArg("f", "force"))
}

func handleUpgrade(ctx context.Context,
	run *runtime.Runtime, cmdArgs *parser.Arguments,
) error {
	return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
}

// -B* options
func handleBuild(ctx context.Context,
	run *runtime.Runtime, dbExecutor db.Executor, cmdArgs *parser.Arguments,
) error {
	if cmdArgs.ExistsArg("i", "install") {
		return installLocalPKGBUILD(ctx, run, cmdArgs, dbExecutor)
	}

	return nil
}

func handleSync(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, dbExecutor db.Executor) error {
	targets := cmdArgs.Targets

	switch {
	case cmdArgs.ExistsArg("s", "search"):
		return syncSearch(ctx, targets, dbExecutor, run.QueryBuilder, !cmdArgs.ExistsArg("q", "quiet"))
	case cmdArgs.ExistsArg("p", "print", "print-format"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("c", "clean"):
		return syncClean(ctx, run, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("l", "list"):
		return syncList(ctx, run, run.HTTPClient, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("g", "groups"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	case cmdArgs.ExistsArg("i", "info"):
		return syncInfo(ctx, run, cmdArgs, targets, dbExecutor)
	case cmdArgs.ExistsArg("u", "sysupgrade") || len(cmdArgs.Targets) > 0:
		return syncInstall(ctx, run, cmdArgs, dbExecutor)
	case cmdArgs.ExistsArg("y", "refresh"):
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	}

	return nil
}

func handleRemove(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments, localCache vcs.Store) error {
	err := run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
		cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	if err == nil {
		localCache.RemovePackages(cmdArgs.Targets)
	}

	return err
}

// NumberMenu presents a CLI for selecting packages to install.
func displayNumberMenu(ctx context.Context, run *runtime.Runtime, pkgS []string, dbExecutor db.Executor,
	queryBuilder query.Builder, cmdArgs *parser.Arguments,
) error {
	queryBuilder.Execute(ctx, dbExecutor, pkgS)

	if err := queryBuilder.Results(dbExecutor, query.NumberMenu); err != nil {
		return err
	}

	if queryBuilder.Len() == 0 {
		// no results were found
		return nil
	}

	run.Logger.Infoln(gotext.Get("Packages to install (eg: 1 2 3, 1-3 or ^4)"))

	numberBuf, err := run.Logger.GetInput("", false)
	if err != nil {
		return err
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(numberBuf)

	targets, err := queryBuilder.GetTargets(include, exclude, otherExclude)
	if err != nil {
		return err
	}

	// modify the arguments to pass for the install
	cmdArgs.Targets = targets

	if len(cmdArgs.Targets) == 0 {
		run.Logger.Println(gotext.Get(" there is nothing to do"))
		return nil
	}

	return syncInstall(ctx, run, cmdArgs, dbExecutor)
}

func syncList(ctx context.Context, run *runtime.Runtime,
	httpClient *http.Client, cmdArgs *parser.Arguments, dbExecutor db.Executor,
) error {
	aur := false

	for i := len(cmdArgs.Targets) - 1; i >= 0; i-- {
		if cmdArgs.Targets[i] == "aur" && run.Cfg.Mode.AtLeastAUR() {
			cmdArgs.Targets = append(cmdArgs.Targets[:i], cmdArgs.Targets[i+1:]...)
			aur = true
		}
	}

	if run.Cfg.Mode.AtLeastAUR() && (len(cmdArgs.Targets) == 0 || aur) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, run.Cfg.AURURL+"/packages.gz", http.NoBody)
		if err != nil {
			return err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)

		scanner.Scan()

		for scanner.Scan() {
			name := scanner.Text()
			if cmdArgs.ExistsArg("q", "quiet") {
				run.Logger.Println(name)
			} else {
				run.Logger.Printf("%s %s %s", text.Magenta("aur"), text.Bold(name), text.Bold(text.Green(gotext.Get("unknown-version"))))

				if dbExecutor.LocalPackage(name) != nil {
					run.Logger.Print(text.Bold(text.Blue(gotext.Get(" [Installed]"))))
				}

				run.Logger.Println()
			}
		}
	}

	if run.Cfg.Mode.AtLeastRepo() && (len(cmdArgs.Targets) != 0 || !aur) {
		return run.CmdBuilder.Show(run.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, run.Cfg.Mode, settings.NoConfirm))
	}

	return nil
}
