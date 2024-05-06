//go:build integration
// +build integration

package download

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/aur"

	mockaur "github.com/Jguer/yippee/v12/pkg/dep/mock"
	"github.com/Jguer/yippee/v12/pkg/settings/exe"
	"github.com/Jguer/yippee/v12/pkg/settings/parser"
	"github.com/Jguer/yippee/v12/pkg/text"
)

func TestIntegrationPKGBUILDReposDefinedDBClone(t *testing.T) {
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/linux", "yippee-bin", "yippee-git"}

	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"core/linux": true, "yippee-bin": true, "yippee-git": true}, cloned)
}

func TestIntegrationPKGBUILDReposNotExist(t *testing.T) {
	dir := t.TempDir()

	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil // fakes a package found for all
		},
	}
	targets := []string{"core/yippee", "yippee-bin", "yippee-git"}
	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	cmdRunner := &exe.OSRunner{Log: testLogger}
	cmdBuilder := &exe.CmdBuilder{
		Runner:   cmdRunner,
		GitBin:   "git",
		GitFlags: []string{},
		Log:      testLogger,
	}

	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"yippee": "core"},
	}
	cloned, err := PKGBUILDRepos(context.Background(), searcher, mockClient,
		cmdBuilder, testLogger.Child("test"),
		targets, parser.ModeAny, "https://aur.archlinux.org", dir, false)

	assert.Error(t, err)
	assert.EqualValues(t, map[string]bool{"yippee-bin": true, "yippee-git": true}, cloned)
}

// GIVEN 2 aur packages and 1 in repo
// WHEN defining as specified targets
// THEN all aur be found and cloned
func TestIntegrationPKGBUILDFull(t *testing.T) {
	mockClient := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{}}, nil
		},
	}

	testLogger := text.NewLogger(os.Stdout, os.Stderr, strings.NewReader(""), true, "test")
	targets := []string{"core/linux", "aur/yippee-bin", "yippee-git"}
	searcher := &testDBSearcher{
		absPackagesDB: map[string]string{"linux": "core"},
	}

	fetched, err := PKGBUILDs(searcher, mockClient, &http.Client{}, testLogger.Child("test"),
		targets, "https://aur.archlinux.org", parser.ModeAny)

	assert.NoError(t, err)

	for _, target := range targets {
		assert.Contains(t, fetched, target)
		assert.NotEmpty(t, fetched[target])
	}
}
