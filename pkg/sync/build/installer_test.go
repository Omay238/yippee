package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yippee/v12/pkg/db/mock"
	"github.com/Jguer/yippee/v12/pkg/dep"
	"github.com/Jguer/yippee/v12/pkg/settings/exe"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
	"github.com/Jguer/yippee/v12/pkg/vcs"
)

func newTestLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
}

func ptrString(s string) *string {
	return &s
}

func TestInstaller_InstallNeeded(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		isInstalled bool
		isBuilt     bool
		wantShow    []string
		wantCapture []string
	}

	testCases := []testCase{
		{
			desc:        "not installed and not built",
			isInstalled: false,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --needed --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "not installed and built",
			isInstalled: false,
			isBuilt:     true,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --needed --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "installed",
			isInstalled: true,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()
			pkgTar := tmpDir + "/yippee-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 2 {
					if !tc.isBuilt {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			// create a mock file
			if tc.isBuilt {
				f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(td, err)
				require.NoError(td, f.Close())
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return tc.isInstalled
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee": tmpDir,
			}

			targets := []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			}

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallMixedSourcesAndLayers(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		targets     []map[string]*dep.InstallInfo
		wantShow    []string
		wantCapture []string
	}

	tmpDir := t.TempDir()
	tmpDirJfin := t.TempDir()

	testCases := []testCase{
		{
			desc: "same layer -- different sources",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- core/linux",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- linux",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "different layer -- different sources",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- core/linux",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- linux",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				}, {
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "same layer -- sync",
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- extra/linux-zen core/linux",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- linux-zen linux",
			},
			wantCapture: []string{},
			targets: []map[string]*dep.InstallInfo{
				{
					"linux-zen": {
						Source:     dep.Sync,
						Reason:     dep.Explicit,
						Version:    "18.0.0-1",
						SyncDBName: ptrString("extra"),
					},
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Explicit,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			},
		},
		{
			desc: "same layer -- aur",
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "10.8.8-1",
						SrcinfoPath: ptrString(tmpDirJfin + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
			},
		},
		{
			desc: "different layer -- aur",
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-server-10.8.8-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- jellyfin-server",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config /etc/pacman.conf -- pacman -U --config /etc/pacman.conf -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				}, {
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.MakeDep,
						Version:     "10.8.8-1",
						SrcinfoPath: ptrString(tmpDirJfin + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTar := tmpDir + "/yippee-91.0.0-1-x86_64.pkg.tar.zst"
			jfinPkgTar := tmpDirJfin + "/jellyfin-server-10.8.8-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				if cmd.Dir == tmpDirJfin {
					return jfinPkgTar, "", nil
				}

				if cmd.Dir == tmpDir {
					return pkgTar, "", nil
				}

				return "", "", fmt.Errorf("unexpected command: %s - %s", cmd.String(), cmd.Dir)
			}

			showOverride := func(cmd *exec.Cmd) error {
				if strings.Contains(cmd.String(), "makepkg -f --noconfirm") && cmd.Dir == tmpDir {
					f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
					require.NoError(td, err)
					require.NoError(td, f.Close())
				}

				if strings.Contains(cmd.String(), "makepkg -f --noconfirm") && cmd.Dir == tmpDirJfin {
					f, err := os.OpenFile(jfinPkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
					require.NoError(td, err)
					require.NoError(td, f.Close())
				}

				return nil
			}
			defer os.Remove(pkgTar)
			defer os.Remove(jfinPkgTar)

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:       makepkgBin,
				SudoBin:          "su",
				PacmanBin:        pacmanBin,
				PacmanConfigPath: "/etc/pacman.conf",
				Runner:           mockRunner,
				SudoLoopEnabled:  false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny, parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee":      tmpDir,
				"jellyfin": tmpDirJfin,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir")     // replace the temp dir with a static path
				show = strings.ReplaceAll(show, tmpDirJfin, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, tmpDirJfin, "/testdir")
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_RunPostHooks(t *testing.T) {
	mockDB := &mock.DBExecutor{}
	mockRunner := &exe.MockRunner{}
	cmdBuilder := &exe.CmdBuilder{
		MakepkgBin:       "makepkg",
		SudoBin:          "su",
		PacmanBin:        "pacman",
		PacmanConfigPath: "/etc/pacman.conf",
		Runner:           mockRunner,
		SudoLoopEnabled:  false,
	}

	cmdBuilder.Runner = mockRunner

	installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
		parser.RebuildModeNo, false, newTestLogger())

	called := false
	hook := func(ctx context.Context) error {
		called = true
		return nil
	}

	installer.AddPostInstallHook(hook)
	installer.RunPostInstallHooks(context.Background())

	assert.True(t, called)
}

func TestInstaller_CompileFailed(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc           string
		targets        []map[string]*dep.InstallInfo
		wantErrInstall bool
		wantErrCompile bool
		failBuild      bool
		failPkgInstall bool
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc:           "one layer",
			wantErrInstall: false,
			wantErrCompile: true,
			failBuild:      true,
			failPkgInstall: false,
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
		{
			desc:           "one layer -- fail install",
			wantErrInstall: true,
			wantErrCompile: false,
			failBuild:      false,
			failPkgInstall: true,
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
		{
			desc:           "two layers",
			wantErrInstall: false,
			wantErrCompile: true,
			failBuild:      true,
			failPkgInstall: false,
			targets: []map[string]*dep.InstallInfo{
				{"bob": {
					AURBase: ptrString("yippee"),
				}},
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTar := tmpDir + "/yippee-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			showOverride := func(cmd *exec.Cmd) error {
				if tc.failBuild && strings.Contains(cmd.String(), "makepkg -f --noconfirm") && cmd.Dir == tmpDir {
					return errors.New("makepkg failed")
				}
				return nil
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("needed")
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{}, false)
			if tc.wantErrInstall {
				require.Error(td, errI)
			} else {
				require.NoError(td, errI)
			}
			failed, err := installer.CompileFailedAndIgnored()
			if tc.wantErrCompile {
				require.Error(td, err)
				assert.ErrorContains(td, err, "yippee")
				assert.Len(t, failed, len(tc.targets))
			} else {
				require.NoError(td, err)
			}
		})
	}
}

func TestInstaller_InstallSplitPackage(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		wantShow    []string
		wantCapture []string
		targets     []map[string]*dep.InstallInfo
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc: "jellyfin",
			targets: []map[string]*dep.InstallInfo{
				{"jellyfin": {
					Source:      dep.AUR,
					Reason:      dep.Explicit,
					Version:     "10.8.4-1",
					SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
					AURBase:     ptrString("jellyfin"),
				}},
				{
					"jellyfin-server": {
						Source:      dep.AUR,
						Reason:      dep.Dep,
						Version:     "10.8.4-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
					"jellyfin-web": {
						Source:      dep.AUR,
						Reason:      dep.Dep,
						Version:     "10.8.4-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("jellyfin"),
					},
				},
				{
					"dotnet-runtime-6.0": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
					"aspnet-runtime": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
					"dotnet-sdk-6.0": {
						Source:     dep.Sync,
						Reason:     dep.MakeDep,
						Version:    "6.0.12.sdk112-1",
						SyncDBName: ptrString("community"),
					},
				},
			},
			wantShow: []string{
				"pacman -S --config /etc/pacman.conf -- community/dotnet-runtime-6.0 community/aspnet-runtime community/dotnet-sdk-6.0",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- dotnet-runtime-6.0 aspnet-runtime dotnet-sdk-6.0",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst /testdir/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asdeps --config /etc/pacman.conf -- jellyfin-server jellyfin-web",
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --config /etc/pacman.conf -- /testdir/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config /etc/pacman.conf -- jellyfin",
			},
			wantCapture: []string{"makepkg --packagelist", "makepkg --packagelist", "makepkg --packagelist"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			pkgTars := []string{
				tmpDir + "/jellyfin-10.8.4-1-x86_64.pkg.tar.zst",
				tmpDir + "/jellyfin-web-10.8.4-1-x86_64.pkg.tar.zst",
				tmpDir + "/jellyfin-server-10.8.4-1-x86_64.pkg.tar.zst",
			}

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return strings.Join(pkgTars, "\n"), "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 4 {
					for _, pkgTar := range pkgTars {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return false
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:       makepkgBin,
				SudoBin:          "su",
				PacmanBin:        pacmanBin,
				PacmanConfigPath: "/etc/pacman.conf",
				Runner:           mockRunner,
				SudoLoopEnabled:  false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("jellyfin")

			pkgBuildDirs := map[string]string{
				"jellyfin": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "),
					strings.Split(tc.wantShow[i], " "),
					fmt.Sprintf("got at %d: %s \n", i, show))
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallDownloadOnly(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		isInstalled bool
		isBuilt     bool
		wantShow    []string
		wantCapture []string
	}

	testCases := []testCase{
		{
			desc:        "not installed and not built",
			isInstalled: false,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "not installed and built",
			isInstalled: false,
			isBuilt:     true,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
		{
			desc:        "installed",
			isInstalled: true,
			isBuilt:     false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
			},
			wantCapture: []string{"makepkg --packagelist"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()
			pkgTar := tmpDir + "/yippee-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 2 {
					if !tc.isBuilt {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			// create a mock file
			if tc.isBuilt {
				f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(td, err)
				require.NoError(td, f.Close())
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return tc.isInstalled
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, true, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee": tmpDir,
			}

			targets := []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			}

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))
			require.Empty(td, installer.failedAndIgnored)

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallGroup(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc        string
		wantShow    []string
		wantCapture []string
	}

	testCases := []testCase{
		{
			desc: "group",
			wantShow: []string{
				"pacman -S --noconfirm --config  -- community/kubernetes-tools",
			},
			wantCapture: []string{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return "", "", nil
			}

			showOverride := func(cmd *exec.Cmd) error {
				return nil
			}

			mockDB := &mock.DBExecutor{}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, true, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("kubernetes-tools")

			pkgBuildDirs := map[string]string{}

			targets := []map[string]*dep.InstallInfo{
				{
					"kubernetes-tools": {
						Source:     dep.Sync,
						Reason:     dep.Explicit,
						Version:    "",
						IsGroup:    true,
						SyncDBName: ptrString("community"),
					},
				},
			}

			errI := installer.Install(context.Background(), cmdArgs, targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))
			require.Empty(td, installer.failedAndIgnored)

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallRebuild(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc          string
		rebuildOption parser.RebuildMode
		isInstalled   bool
		isBuilt       bool
		wantShow      []string
		wantCapture   []string
		targets       []map[string]*dep.InstallInfo
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc:          "--norebuild(default) when built and not installed",
			rebuildOption: parser.RebuildModeNo,
			isBuilt:       true,
			isInstalled:   false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -c --nobuild --noextract --ignorearch",
				"pacman -U --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
		{
			desc:          "--rebuild when built and not installed",
			rebuildOption: parser.RebuildModeYes,
			isBuilt:       true,
			isInstalled:   false,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
		{
			desc:          "--rebuild when built and installed",
			rebuildOption: parser.RebuildModeYes,
			isInstalled:   true,
			isBuilt:       true,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
		{
			desc:          "--rebuild when built and installed previously as dep",
			rebuildOption: parser.RebuildModeYes,
			isInstalled:   true,
			isBuilt:       true,
			wantShow: []string{
				"makepkg --nobuild -f -C --ignorearch",
				"makepkg -f -c --noconfirm --noextract --noprepare --holdver --ignorearch",
				"pacman -U --config  -- /testdir/yippee-91.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asdeps --config  -- yippee",
			},
			wantCapture: []string{"makepkg --packagelist"},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Dep,
						Version:     "91.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()
			pkgTar := tmpDir + "/yippee-91.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			i := 0
			showOverride := func(cmd *exec.Cmd) error {
				i++
				if i == 2 {
					if !tc.isBuilt {
						f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
						require.NoError(td, err)
						require.NoError(td, f.Close())
					}
				}
				return nil
			}

			// create a mock file
			if tc.isBuilt {
				f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
				require.NoError(td, err)
				require.NoError(td, f.Close())
			}

			isCorrectInstalledOverride := func(string, string) bool {
				return tc.isInstalled
			}

			mockDB := &mock.DBExecutor{IsCorrectVersionInstalledFn: isCorrectInstalledOverride}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride, ShowFn: showOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			cmdBuilder.Runner = mockRunner

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				tc.rebuildOption, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))
			require.Len(td, mockRunner.CaptureCalls, len(tc.wantCapture))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)
			}

			for i, call := range mockRunner.CaptureCalls {
				capture := call.Args[0].(*exec.Cmd).String()
				capture = strings.ReplaceAll(capture, tmpDir, "/testdir") // replace the temp dir with a static path
				capture = strings.ReplaceAll(capture, makepkgBin, "makepkg")
				capture = strings.ReplaceAll(capture, pacmanBin, "pacman")
				assert.Subset(td, strings.Split(capture, " "), strings.Split(tc.wantCapture[i], " "), capture)
			}
		})
	}
}

func TestInstaller_InstallUpgrade(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc       string
		targetMode parser.TargetMode
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc:       "target any",
			targetMode: parser.ModeAny,
		},
		{
			desc:       "target repo",
			targetMode: parser.ModeRepo,
		},
		{
			desc:       "target aur",
			targetMode: parser.ModeAUR,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			mockDB := &mock.DBExecutor{}
			mockRunner := &exe.MockRunner{}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, tc.targetMode,
				parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddArg("u", "upgrades") // Make sure both args are removed

			targets := []map[string]*dep.InstallInfo{
				{
					"linux": {
						Source:     dep.Sync,
						Reason:     dep.Dep,
						Version:    "17.0.0-1",
						SyncDBName: ptrString("core"),
					},
				},
			}

			errI := installer.Install(context.Background(), cmdArgs, targets, map[string]string{}, []string{}, false)
			require.NoError(td, errI)

			require.NotEmpty(td, mockRunner.ShowCalls)

			// The first call is the only call being test
			call := mockRunner.ShowCalls[0]
			show := call.Args[0].(*exec.Cmd).String()
			show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
			show = strings.ReplaceAll(show, makepkgBin, "makepkg")
			show = strings.ReplaceAll(show, pacmanBin, "pacman")

			if tc.targetMode == parser.ModeAUR {
				assert.NotContains(td, show, "--upgrades")
				assert.NotContains(td, show, "-u")
			} else {
				assert.Contains(td, show, "--upgrades")
				assert.Contains(td, show, "-u")
			}
		})
	}
}

func TestInstaller_KeepSrc(t *testing.T) {
	t.Parallel()

	makepkgBin := t.TempDir() + "/makepkg"
	pacmanBin := t.TempDir() + "/pacman"
	f, err := os.OpenFile(makepkgBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	f, err = os.OpenFile(pacmanBin, os.O_RDONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	type testCase struct {
		desc     string
		wantShow []string
		targets  []map[string]*dep.InstallInfo
	}

	tmpDir := t.TempDir()

	testCases := []testCase{
		{
			desc: "--keepsrc",
			wantShow: []string{
				"makepkg --nobuild -f --ignorearch",
				"makepkg --nobuild --noextract --ignorearch",
				"pacman -U --config  -- /testdir/yippee-92.0.0-1-x86_64.pkg.tar.zst",
				"pacman -D -q --asexplicit --config  -- yippee",
			},
			targets: []map[string]*dep.InstallInfo{
				{
					"yippee": {
						Source:      dep.AUR,
						Reason:      dep.Explicit,
						Version:     "92.0.0-1",
						SrcinfoPath: ptrString(tmpDir + "/.SRCINFO"),
						AURBase:     ptrString("yippee"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(td *testing.T) {
			tmpDir := td.TempDir()
			pkgTar := tmpDir + "/yippee-92.0.0-1-x86_64.pkg.tar.zst"

			captureOverride := func(cmd *exec.Cmd) (stdout string, stderr string, err error) {
				return pkgTar, "", nil
			}

			// create a mock file
			f, err := os.OpenFile(pkgTar, os.O_RDONLY|os.O_CREATE, 0o666)
			require.NoError(td, err)
			require.NoError(td, f.Close())

			mockDB := &mock.DBExecutor{}
			mockRunner := &exe.MockRunner{CaptureFn: captureOverride}
			cmdBuilder := &exe.CmdBuilder{
				MakepkgBin:      makepkgBin,
				SudoBin:         "su",
				PacmanBin:       pacmanBin,
				KeepSrc:         true,
				Runner:          mockRunner,
				SudoLoopEnabled: false,
			}

			installer := NewInstaller(mockDB, cmdBuilder, &vcs.Mock{}, parser.ModeAny,
				parser.RebuildModeNo, false, newTestLogger())

			cmdArgs := parser.MakeArguments()
			cmdArgs.AddTarget("yippee")

			pkgBuildDirs := map[string]string{
				"yippee": tmpDir,
			}

			errI := installer.Install(context.Background(), cmdArgs, tc.targets, pkgBuildDirs, []string{}, false)
			require.NoError(td, errI)

			require.Len(td, mockRunner.ShowCalls, len(tc.wantShow))

			for i, call := range mockRunner.ShowCalls {
				show := call.Args[0].(*exec.Cmd).String()
				show = strings.ReplaceAll(show, tmpDir, "/testdir") // replace the temp dir with a static path
				show = strings.ReplaceAll(show, makepkgBin, "makepkg")
				show = strings.ReplaceAll(show, pacmanBin, "pacman")

				// options are in a different order on different systems and on CI root user is used
				assert.Subset(td, strings.Split(show, " "), strings.Split(tc.wantShow[i], " "), show)

				// Only assert makepkg commands don't have clean arguments
				if strings.HasPrefix(show, "makepkg") {
					assert.NotContains(td, show, "-c")
					assert.NotContains(td, show, "-C")
				}
			}
		})
	}
}
