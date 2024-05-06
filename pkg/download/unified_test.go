//go:build !integration
// +build !integration

package download

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"

	"github.com/Jguer/aur"

	mockaur "github.com/Jguer/yippee/v12/pkg/dep/mock"
	"github.com/Jguer/yippee/v12/pkg/settings/exe"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
)

func newTestLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
}

// GIVEN 2 aur packages and 1 in repo
// GIVEN package in repo is already present
// WHEN defining package db as a target
// THEN all should be found and cloned, except the repo one
func TestPKGBUILDReposDefinedDBPull(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}

	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")

	os.MkdirAll(filepath.Join(dir, "yippee", ".git"), 0o777)

	targets := []string{"core/yippee", "yippee-bin", "yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
			Log:      testLogger,
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/yippee": false, "yippee-bin": true, "yippee-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining package db as a target
// THEN all should be found and cloned
func TestPKGBUILDReposDefinedDBClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/yippee", "yippee-bin", "yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/yippee": true, "yippee-bin": true, "yippee-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as non specified targets
// THEN all should be found and cloned
func TestPKGBUILDReposClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"yippee", "yippee-bin", "yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yippee": true, "yippee-bin": true, "yippee-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo but wrong db
// WHEN defining as non specified targets
// THEN all aur be found and cloned
func TestPKGBUILDReposNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"extra/yippee", "yippee-bin", "yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yippee-bin": true, "yippee-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as non specified targets in repo mode
// THEN only repo should be cloned
func TestPKGBUILDReposRepoMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil // fakes a package found for all
		},
	}
	targets := []string{"yippee", "yippee-bin", "yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeRepo, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yippee": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as specified targets
// THEN all aur be found and cloned
func TestPKGBUILDFull(t *testing.T) {
	t.Parallel()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}
	gock.New("https://aur.archlinux.org").
		Get("/cgit/aur.git/plain/PKGBUILD").MatchParam("h", "yippee-git").
		Reply(200).
		BodyString("example_yippee-git")
	gock.New("https://aur.archlinux.org").
		Get("/cgit/aur.git/plain/PKGBUILD").MatchParam("h", "yippee-bin").
		Reply(200).
		BodyString("example_yippee-bin")

	gock.New("https://gitlab.archlinux.org/").
		Get("archlinux/packaging/packages/yippee/-/raw/main/PKGBUILD").
		Reply(200).
		BodyString("example_yippee")

	defer gock.Off()
	targets := []string{"core/yippee", "aur/yippee-bin", "yippee-git"}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, newTestLogger(),
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string][]byte{
		"core/yippee":    []byte("example_yippee"),
		"aur/yippee-bin": []byte("example_yippee-bin"),
		"yippee-git":     []byte("example_yippee-git"),
	}, fetched)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN aur packages are not found
// only repo should be cloned
func TestPKGBUILDReposMissingAUR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/yippee", "aur/yippee-bin", "aur/yippee-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, newTestLogger(),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/yippee": true}, cloned)
}
