package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/wellmaintained/yak-box/internal/errors"
	"github.com/wellmaintained/yak-box/pkg/types"
)

func TestSpawnFlags(t *testing.T) {
	// Verify core flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("cwd"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("name"))

	// Verify optional flags are registered
	assert.NotNil(t, spawnCmd.Flags().Lookup("mode"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("resources"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yaks"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("yak-path"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("runtime"))
	assert.NotNil(t, spawnCmd.Flags().Lookup("model"))

	// Verify flag defaults
	mode, _ := spawnCmd.Flags().GetString("mode")
	assert.Equal(t, "build", mode)

	resources, _ := spawnCmd.Flags().GetString("resources")
	assert.Equal(t, "default", resources)

	yakPath, _ := spawnCmd.Flags().GetString("yak-path")
	assert.Equal(t, ".yaks", yakPath)

	runtime, _ := spawnCmd.Flags().GetString("runtime")
	assert.Equal(t, "auto", runtime)

	model, _ := spawnCmd.Flags().GetString("model")
	assert.Equal(t, "", model)
}

func TestFormatDisplayName(t *testing.T) {
	t.Run("uses worker and spawn names", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "cursor-test")
		assert.Equal(t, "Yakov 🪒🦬 cursor-test", displayName)
	})

	t.Run("trims spawn name", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "  cursor test  ")
		assert.Equal(t, "Yakov 🪒🦬 cursor test", displayName)
	})

	t.Run("falls back to worker name when spawn name empty", func(t *testing.T) {
		displayName := formatDisplayName("Yakov", "   ")
		assert.Equal(t, "Yakov", displayName)
	})
}

func TestSpawnValidation(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		spawnName string
		mode      string
		resources string
		runtime   string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "missing name",
			cwd:       "",
			spawnName: "",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--name is required",
		},
		{
			name:      "invalid mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "invalid",
			resources: "default",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--mode must be 'plan' or 'build'",
		},
		{
			name:      "invalid resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "invalid",
			runtime:   "auto",
			wantErr:   true,
			errMsg:    "--resources must be",
		},
		{
			name:      "invalid runtime",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "--runtime must be",
		},
		{
			name:      "multiple validation errors batched",
			cwd:       "",
			spawnName: "",
			mode:      "invalid",
			resources: "invalid",
			runtime:   "invalid",
			wantErr:   true,
			errMsg:    "Validation errors",
		},
		{
			name:      "valid minimal config",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "default",
			runtime:   "auto",
			wantErr:   false,
		},
		{
			name:      "valid with plan mode",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "plan",
			resources: "light",
			runtime:   "native",
			wantErr:   false,
		},
		{
			name:      "valid with heavy resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "heavy",
			runtime:   "sandboxed",
			wantErr:   false,
		},
		{
			name:      "valid with ram resources",
			cwd:       "/tmp/test",
			spawnName: "test-worker",
			mode:      "build",
			resources: "ram",
			runtime:   "auto",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.PersistentFlags().AddFlagSet(spawnCmd.PersistentFlags())
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = tt.cwd
			spawnName = tt.spawnName
			spawnMode = tt.mode
			spawnResources = tt.resources
			spawnRuntime = tt.runtime

			err := spawnCmd.PreRunE(cmd, []string{})

			if tt.wantErr {
				assert.Error(t, err, "expected error for test case: %s", tt.name)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg, "error message should contain expected text")
				}

				_, ok := err.(*errors.ValidationError)
				if ok {
					exitCode := errors.GetExitCode(err)
					assert.Equal(t, 2, exitCode, "ValidationError should return exit code 2")
				}
			} else {
				assert.NoError(t, err, "expected no error for test case: %s", tt.name)
			}
		})
	}
}

func TestSpawnToolOptions(t *testing.T) {
	validTools := []string{"claude", "opencode", "cursor"}

	for _, tool := range validTools {
		t.Run("valid_tool_"+tool, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = "default"
			spawnRuntime = "auto"
			spawnTool = tool

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}

	t.Run("invalid_tool", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().AddFlagSet(spawnCmd.Flags())

		spawnCWD = "/tmp/test"
		spawnName = "test-worker"
		spawnMode = "build"
		spawnResources = "default"
		spawnRuntime = "auto"
		spawnTool = "invalid"

		err := spawnCmd.PreRunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--tool must be")

		// Reset to avoid polluting subsequent tests that share this global
		spawnTool = "claude"
	})
}

func TestResolveSpawnModel(t *testing.T) {
	t.Run("respects explicit model override", func(t *testing.T) {
		assert.Equal(t, "haiku", resolveSpawnModel("claude", "haiku"))
	})

	t.Run("uses claude default model", func(t *testing.T) {
		assert.Equal(t, "default", resolveSpawnModel("claude", ""))
	})

	t.Run("uses cursor default model", func(t *testing.T) {
		assert.Equal(t, "auto", resolveSpawnModel("cursor", ""))
	})

	t.Run("uses no default for opencode", func(t *testing.T) {
		assert.Equal(t, "", resolveSpawnModel("opencode", ""))
	})
}

func TestSpawnValidationBatching(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().AddFlagSet(spawnCmd.Flags())

	spawnCWD = ""
	spawnName = ""
	spawnMode = "invalid"
	spawnResources = "invalid"
	spawnRuntime = "invalid"

	err := spawnCmd.PreRunE(cmd, []string{})

	assert.Error(t, err)
	errMsg := err.Error()

	assert.Contains(t, errMsg, "Validation errors")
	assert.Contains(t, errMsg, "--name is required")
	assert.Contains(t, errMsg, "--mode must be")
	assert.Contains(t, errMsg, "--resources must be")
	assert.Contains(t, errMsg, "--runtime must be")
}

func TestSpawnFlagTypes(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
		want     interface{}
	}{
		{name: "cwd string flag", flagName: "cwd", want: ""},
		{name: "name string flag", flagName: "name", want: ""},
		{name: "session string flag", flagName: "session", want: "yak-box"},
		{name: "mode string flag", flagName: "mode", want: "build"},
		{name: "resources string flag", flagName: "resources", want: "default"},
		{name: "yak-path string flag", flagName: "yak-path", want: ".yaks"},
		{name: "runtime string flag", flagName: "runtime", want: "auto"},
		{name: "model string flag", flagName: "model", want: ""},
		{name: "clean bool flag", flagName: "clean", want: false},
		{name: "auto-worktree bool flag", flagName: "auto-worktree", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := spawnCmd.Flags().Lookup(tt.flagName)
			assert.NotNil(t, flag)
		})
	}
}

func TestSpawnResourceOptions(t *testing.T) {
	validResources := []string{"light", "default", "heavy", "ram"}

	for _, resource := range validResources {
		t.Run("valid_resource_"+resource, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = resource
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestSpawnModeOptions(t *testing.T) {
	validModes := []string{"plan", "build"}

	for _, mode := range validModes {
		t.Run("valid_mode_"+mode, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = mode
			spawnResources = "default"
			spawnRuntime = "auto"

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}

func TestFindTaskDir(t *testing.T) {
	// Create a temp .yaks tree:
	// .yaks/
	//   release/
	//     yak-box/
	//       missing-tab-emoji/
	//   fixes/
	//     tab-emoji/
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "release", "yak-box", "missing-tab-emoji")
	os.MkdirAll(nested, 0755)
	other := filepath.Join(tmpDir, "fixes", "tab-emoji")
	os.MkdirAll(other, 0755)

	t.Run("finds nested task by leaf name", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "missing-tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, nested, dir)
	})

	t.Run("finds task with direct full path", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "release/yak-box/missing-tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, nested, dir)
	})

	t.Run("finds task in different subtree", func(t *testing.T) {
		dir, err := findTaskDir(tmpDir, "tab-emoji")
		assert.NoError(t, err)
		assert.Equal(t, other, dir)
	})

	t.Run("returns error for nonexistent task", func(t *testing.T) {
		_, err := findTaskDir(tmpDir, "nonexistent-task")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no directory matching")
	})
}

func TestFindYakPath(t *testing.T) {
	// Create a nested dir structure:
	// tmpDir/
	//   .yaks/           <- the target
	//   sub/
	//     deep/          <- startDir for walk-up tests
	tmpDir := t.TempDir()
	yakDir := filepath.Join(tmpDir, ".yaks")
	os.MkdirAll(yakDir, 0755)
	deepDir := filepath.Join(tmpDir, "sub", "deep")
	os.MkdirAll(deepDir, 0755)

	t.Run("finds .yaks at start dir", func(t *testing.T) {
		got, err := findYakPath(tmpDir, ".yaks")
		assert.NoError(t, err)
		assert.Equal(t, yakDir, got)
	})

	t.Run("finds .yaks two levels up", func(t *testing.T) {
		got, err := findYakPath(deepDir, ".yaks")
		assert.NoError(t, err)
		assert.Equal(t, yakDir, got)
	})

	t.Run("returns error when no .yaks exists", func(t *testing.T) {
		noYakDir := t.TempDir()
		_, err := findYakPath(noYakDir, ".yaks")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no .yaks directory found above")
		assert.Contains(t, err.Error(), "--yak-path")
	})
}

func TestResolveInheritedWorktrees(t *testing.T) {
	workspace := t.TempDir()
	absYakPath := filepath.Join(workspace, ".yaks")
	taskDir := filepath.Join(absYakPath, "sc-12345", "child-task")
	repoA := filepath.Join(workspace, "repos", "releng", "release")
	repoB := filepath.Join(workspace, "repos", "releng", "monix")

	assert.NoError(t, os.MkdirAll(taskDir, 0755))
	assert.NoError(t, os.MkdirAll(repoA, 0755))
	assert.NoError(t, os.MkdirAll(repoB, 0755))
	assert.NoError(t, os.WriteFile(
		filepath.Join(absYakPath, "sc-12345", "worktrees"),
		[]byte("repos/releng/release,repos/releng/monix"),
		0644,
	))

	initRepo := func(path string) {
		cmd := exec.Command("git", "init")
		cmd.Dir = path
		assert.NoError(t, cmd.Run())
	}
	initRepo(repoA)
	initRepo(repoB)

	t.Run("inherits worktrees from ancestor and uses ancestor as branch", func(t *testing.T) {
		gotRepos, gotBranch, err := resolveInheritedWorktrees(absYakPath, "sc-12345/child-task")
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{repoA, repoB}, gotRepos)
		assert.Equal(t, "sc-12345", gotBranch)
	})

	t.Run("returns no worktrees when field is absent", func(t *testing.T) {
		emptyYakPath := filepath.Join(workspace, ".empty-yaks")
		emptyTaskDir := filepath.Join(emptyYakPath, "sc-54321", "child-task")
		assert.NoError(t, os.MkdirAll(emptyTaskDir, 0755))

		gotRepos, gotBranch, err := resolveInheritedWorktrees(emptyYakPath, "sc-54321/child-task")
		assert.NoError(t, err)
		assert.Empty(t, gotRepos)
		assert.Empty(t, gotBranch)
	})

	t.Run("returns an error for non-git worktree paths", func(t *testing.T) {
		badYakPath := filepath.Join(workspace, ".bad-yaks")
		badTaskDir := filepath.Join(badYakPath, "sc-99999", "child-task")
		notGitRepo := filepath.Join(workspace, "repos", "releng", "not-git")
		assert.NoError(t, os.MkdirAll(badTaskDir, 0755))
		assert.NoError(t, os.MkdirAll(notGitRepo, 0755))
		assert.NoError(t, os.WriteFile(
			filepath.Join(badYakPath, "sc-99999", "worktrees"),
			[]byte("repos/releng/not-git"),
			0644,
		))

		_, _, err := resolveInheritedWorktrees(badYakPath, "sc-99999/child-task")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not a git repository")
	})
}

func TestPickWorkerNameRoundRobin(t *testing.T) {
	// Run from a temp git repo so GetYakBoxesDir() succeeds and we use .last-persona
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, "repo")
	assert.NoError(t, os.MkdirAll(gitDir, 0755))
	// Initialize git so getRoot() returns gitDir
	initCmd := exec.Command("git", "init")
	initCmd.Dir = gitDir
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	assert.NoError(t, initCmd.Run())
	origWd, err := os.Getwd()
	assert.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	assert.NoError(t, os.Chdir(gitDir))

	// Remove .last-persona if present so we start from a known state
	_ = os.Remove(filepath.Join(gitDir, ".yak-boxes", lastPersonaFile))

	var names []string
	for i := 0; i < 8; i++ {
		names = append(names, pickWorkerName())
	}
	// Consecutive spawns must get different personas
	for i := 1; i < len(names); i++ {
		assert.NotEqual(t, names[i-1], names[i], "consecutive spawns at %d and %d should differ", i-1, i)
	}
	// First four should be a permutation of WorkerNames; second four same cycle
	expect := make(map[string]int)
	for _, w := range types.WorkerNames {
		expect[w] = 0
	}
	for i := 0; i < 4; i++ {
		assert.Contains(t, expect, names[i])
		expect[names[i]]++
	}
	for _, c := range expect {
		assert.Equal(t, 1, c, "first 4 spawns should each use a different persona")
	}
	// Second cycle should match first
	for i := 0; i < 4; i++ {
		assert.Equal(t, names[i], names[i+4], "round-robin should repeat after 4")
	}
}

func TestSpawnRuntimeOptions(t *testing.T) {
	validRuntimes := []string{"auto", "sandboxed", "native"}

	for _, runtime := range validRuntimes {
		t.Run("valid_runtime_"+runtime, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().AddFlagSet(spawnCmd.Flags())

			spawnCWD = "/tmp/test"
			spawnName = "test-worker"
			spawnMode = "build"
			spawnResources = "default"
			spawnRuntime = runtime

			err := spawnCmd.PreRunE(cmd, []string{})
			assert.NoError(t, err)
		})
	}
}
