package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wellmaintained/yak-box/internal/errors"
	"github.com/wellmaintained/yak-box/internal/prompt"
	"github.com/wellmaintained/yak-box/internal/runtime"
	"github.com/wellmaintained/yak-box/internal/sessions"
	"github.com/wellmaintained/yak-box/internal/ui"
	"github.com/wellmaintained/yak-box/pkg/devcontainer"
	"github.com/wellmaintained/yak-box/pkg/types"
	"github.com/wellmaintained/yak-box/pkg/worktree"
)

var (
	spawnCWD          string
	spawnName         string
	spawnSession      string
	spawnMode         string
	spawnResources    string
	spawnYaks         []string
	spawnYakPath      string
	spawnRuntime      string
	spawnTool         string
	spawnModel        string
	spawnClean        bool
	spawnAutoWorktree bool
)

const (
	defaultClaudeModel = "default"
	defaultCursorModel = "auto"
)

var spawnCmd = &cobra.Command{
	Use:   "spawn --cwd <dir> --name <tab-name> [flags]",
	Short: "Spawn a new worker",
	Long: `Spawn a new worker with specified configuration.

The spawn command creates a new worker (sandboxed or native) with a randomly
selected name, assembles the appropriate prompt, and assigns tasks.

Sandboxed mode (default): Uses Docker container with resource limits and isolation.
Native mode: Runs the AI tool directly on the host with full system access.

Tool selection:
  --tool claude (default): Uses Claude Code with --print mode and agent prompts.
  --tool opencode: Uses OpenCode with --agent build mode.
  --tool cursor: Uses Cursor agent CLI with --force mode.`,
	Example: `  # Spawn a worker for API authentication tasks
  yak-box spawn --cwd ./api --name api-auth --yaks auth/api/login --yaks auth/api/logout

  # Spawn with automatic worktree creation
  yak-box spawn --cwd ./api --name api-auth --yaks auth/api --auto-worktree

  # Spawn with heavy resources and native runtime
  yak-box spawn --cwd ./backend --name backend-worker --resources heavy --runtime native

  # Spawn in plan mode with custom yak path
  yak-box spawn --cwd ./frontend --name ui-worker --mode plan --yak-path .tasks`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var errs []error

		if spawnCWD == "" {
			errs = append(errs, fmt.Errorf("--cwd is required (working directory for the worker)"))
		}
		if spawnName == "" {
			errs = append(errs, fmt.Errorf("--name is required (worker name used in logs and metadata)"))
		}

		if spawnMode != "plan" && spawnMode != "build" {
			errs = append(errs, fmt.Errorf("--mode must be 'plan' or 'build', got '%s'", spawnMode))
		}

		if spawnResources != "light" && spawnResources != "default" && spawnResources != "heavy" && spawnResources != "ram" {
			errs = append(errs, fmt.Errorf("--resources must be 'light', 'default', 'heavy', or 'ram', got '%s'", spawnResources))
		}

		if spawnRuntime != "auto" && spawnRuntime != "sandboxed" && spawnRuntime != "native" {
			errs = append(errs, fmt.Errorf("--runtime must be 'auto', 'sandboxed', or 'native', got '%s'", spawnRuntime))
		}

		if spawnTool != "opencode" && spawnTool != "claude" && spawnTool != "cursor" {
			errs = append(errs, fmt.Errorf("--tool must be 'opencode', 'claude', or 'cursor', got '%s'", spawnTool))
		}

		if len(errs) > 0 {
			combined := "Validation errors:\n"
			for _, err := range errs {
				combined += fmt.Sprintf("  - %s\n", err)
			}
			return errors.NewValidationError(combined, nil)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSpawn(cmd, cmd.Context(), args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(errors.GetExitCode(err))
		}
	},
}

const lastPersonaFile = ".last-persona"

// pickWorkerName selects the next worker name in round-robin order so consecutive
// spawns get different personas. State is stored in .yak-boxes/.last-persona.
// Falls back to random if the state file cannot be read or written (e.g. not in a git repo).
func pickWorkerName() string {
	n := len(types.WorkerNames)
	if n == 0 {
		return ""
	}
	dir, err := sessions.GetYakBoxesDir()
	if err != nil {
		return types.WorkerNames[rand.Intn(n)]
	}
	path := filepath.Join(dir, lastPersonaFile)
	data, err := os.ReadFile(path)
	idx := 0
	if err == nil {
		idx, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if idx < 0 || idx >= n {
			idx = 0
		}
	}
	next := (idx + 1) % n
	_ = os.WriteFile(path, []byte(strconv.Itoa(next)), 0644)
	return types.WorkerNames[idx]
}

func formatDisplayName(workerName, spawnName string) string {
	trimmedName := strings.TrimSpace(spawnName)
	if trimmedName == "" {
		return workerName
	}
	return fmt.Sprintf("%s 🪒🦬 %s", workerName, trimmedName)
}

func resolveSpawnModel(tool, model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}

	switch tool {
	case "claude":
		return defaultClaudeModel
	case "cursor":
		return defaultCursorModel
	default:
		return ""
	}
}

func runSpawn(cmd *cobra.Command, ctx context.Context, args []string) error {
	runtimeType := spawnRuntime
	if runtimeType == "auto" {
		runtimeType = runtime.DetectRuntime()
		if runtimeType == "unknown" {
			return fmt.Errorf("no runtime available (docker or zellij). Suggestion: Install Docker and start the daemon, or install Zellij. Force with --runtime=sandboxed or --runtime=native")
		}
	}

	absCWD, err := filepath.Abs(spawnCWD)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w. Suggestion: Ensure --cwd path is valid and accessible", err)
	}

	var absYakPath string
	if cmd.Flags().Changed("yak-path") {
		absYakPath, err = filepath.Abs(spawnYakPath)
		if err != nil {
			return fmt.Errorf("failed to resolve yak path: %w. Suggestion: Ensure --yak-path exists and is accessible", err)
		}
	} else {
		absYakPath, err = findYakPath(absCWD, filepath.Base(spawnYakPath))
		if err != nil {
			return fmt.Errorf("No .yaks found above %s. Use --yak-path to specify explicitly", absCWD)
		}
	}

	worktreePath := ""
	if spawnAutoWorktree && len(spawnYaks) > 0 {
		taskPath := spawnYaks[0]
		fmt.Printf("Creating worktree for task: %s\n", taskPath)

		wt, err := worktree.EnsureWorktree(absCWD, taskPath, true)
		if err != nil {
			return fmt.Errorf("failed to ensure worktree: %w. Suggestion: Ensure you're in a git repository with proper permissions, or disable --auto-worktree", err)
		}

		worktreePath = wt
		absCWD = wt
		fmt.Printf("Using worktree: %s\n", wt)
	}

	workerName := pickWorkerName()

	if spawnClean {
		fmt.Printf("Cleaning home directory for %s...\n", workerName)
		if err := sessions.CleanHome(workerName); err != nil {
			return fmt.Errorf("failed to clean home: %w. Suggestion: Ensure .yak-boxes directory exists and is writable", err)
		}
	}

	homeDir, err := sessions.EnsureHomeDir(workerName)
	if err != nil {
		return fmt.Errorf("failed to ensure home directory: %w. Suggestion: Check that .yak-boxes directory exists and is writable", err)
	}

	devConfig, err := devcontainer.LoadConfig(absCWD)
	if err != nil {
		return fmt.Errorf("failed to load devcontainer config: %w. Suggestion: Ensure .devcontainer/devcontainer.json is valid JSON if it exists", err)
	}

	profile := runtime.GetResourceProfile(spawnResources)

	userPrompt := "Work on the assigned tasks."
	if len(args) > 0 {
		userPrompt = args[0]
	}

	workerPrompt := prompt.BuildPrompt(spawnMode, spawnYakPath, userPrompt, spawnYaks)

	displayName := formatDisplayName(workerName, spawnName)

	sanitizedName := strings.ReplaceAll(spawnName, " ", "-")
	sanitizedName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1
	}, sanitizedName)

	// Resolve agent name for Claude Code workers
	agentName := ""
	if spawnTool == "claude" {
		// Look for a personality-specific agent file: .claude/agents/<workerName>-worker.md
		lowerName := strings.ToLower(workerName)
		candidatePath := filepath.Join(absCWD, ".claude", "agents", lowerName+"-worker.md")
		if _, err := os.Stat(candidatePath); err == nil {
			agentName = lowerName + "-worker"
		}
	}
	if spawnTool == "opencode" && spawnModel != "" {
		ui.Warning("⚠️  --model is currently ignored for --tool opencode\n")
	}
	resolvedModel := resolveSpawnModel(spawnTool, spawnModel)

	worker := &types.Worker{
		Name:          spawnName,
		WorkerName:    workerName,
		DisplayName:   displayName,
		ContainerName: "yak-worker-" + sanitizedName,
		Runtime:       runtimeType,
		CWD:           absCWD,
		YakPath:       absYakPath,
		Tasks:         spawnYaks,
		SpawnedAt:     time.Now(),
		SessionName:   spawnSession,
		WorktreePath:  worktreePath,
		Tool:          spawnTool,
		Model:         resolvedModel,
		AgentName:     agentName,
	}

	if runtimeType == "sandboxed" {
		ui.Info("⏳ Building container...\n")
		if err := runtime.EnsureDevcontainer(); err != nil {
			ui.Error("❌ Build failed: %v\n", err)
			return fmt.Errorf("failed to ensure devcontainer: %w\n\nSuggestion: Install Docker or use native mode.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}

		if err := runtime.SpawnSandboxedWorker(ctx,
			runtime.WithWorker(worker),
			runtime.WithPrompt(workerPrompt),
			runtime.WithResourceProfile(profile),
			runtime.WithHomeDir(homeDir),
			runtime.WithDevConfig(devConfig),
		); err != nil {
			ui.Error("❌ Failed to spawn sandboxed worker: %v\n", err)
			return fmt.Errorf("failed to spawn sandboxed worker: %w\n\nSuggestion: Check Docker is running and has enough resources.\nTo try native mode instead, run:\n  yak-box spawn --runtime=native [same options]", err)
		}
		ui.Success("✅ Container ready\n")
	} else {
		ui.Info("⏳ Starting native worker...\n")
		pidFile, err := runtime.SpawnNativeWorker(worker, workerPrompt, homeDir)
		if err != nil {
			ui.Error("❌ Failed to spawn native worker: %v\n", err)
			return fmt.Errorf("failed to spawn native worker: %w. Suggestion: Ensure Zellij is installed and running, or use --runtime=sandboxed instead", err)
		}
		worker.PidFile = pidFile
		ui.Success("✅ Native worker started\n")
	}

	taskName := ""
	if len(spawnYaks) > 0 {
		taskName = spawnYaks[0]
	}

	if err := sessions.Register(spawnName, sessions.Session{
		Worker:        workerName,
		Task:          taskName,
		Container:     worker.ContainerName,
		SpawnedAt:     worker.SpawnedAt,
		Runtime:       runtimeType,
		CWD:           absCWD,
		DisplayName:   displayName,
		ZellijSession: spawnSession,
		PidFile:       worker.PidFile,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to register session: %v\n", err)
	}

	for _, task := range spawnYaks {
		taskSlug := types.SlugifyTaskPath(task)
		taskDir, err := findTaskDir(absYakPath, taskSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to find task directory for %s: %v\n", task, err)
			continue
		}

		taskFile := filepath.Join(taskDir, "assigned-to")
		if err := os.WriteFile(taskFile, []byte(workerName), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to assign task %s: %v\n", task, err)
		}

		if worktreePath != "" {
			worktreeFile := filepath.Join(taskDir, "worktree-path")
			if err := os.WriteFile(worktreeFile, []byte(worktreePath), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write worktree path for task %s: %v\n", task, err)
			}
		}
	}

	fmt.Printf("Spawned %s (%s) in %s\n", workerName, spawnName, runtimeType)
	return nil
}

// findTaskDir searches the .yaks/ tree for a directory matching the task slug.
// Tasks can be nested (e.g., "release-yakthang/yak-box/missing-tab-emoji"),
// so we walk the tree looking for a directory whose base name matches the slug.
func findTaskDir(yakPath, taskSlug string) (string, error) {
	// If the slug contains path separators, try the direct path first.
	directPath := filepath.Join(yakPath, taskSlug)
	if info, err := os.Stat(directPath); err == nil && info.IsDir() {
		return directPath, nil
	}

	// Otherwise, search for a directory with a matching leaf name.
	leafName := filepath.Base(taskSlug)
	var matches []string
	filepath.Walk(yakPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == leafName && path != yakPath {
			matches = append(matches, path)
		}
		return nil
	})

	if len(matches) == 0 {
		return "", fmt.Errorf("no directory matching %q found under %s", taskSlug, yakPath)
	}
	if len(matches) > 1 {
		fmt.Fprintf(os.Stderr, "Warning: multiple directories match %q, using first: %s\n", taskSlug, matches[0])
	}
	return matches[0], nil
}

// findYakPath walks up from startDir looking for a directory named yakDirName,
// similar to how git finds .git. Returns the full path if found, error if not.
func findYakPath(startDir string, yakDirName string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, yakDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no .yaks directory found above %s — use --yak-path to specify", startDir)
}

func init() {
	spawnCmd.Flags().StringVar(&spawnCWD, "cwd", "", "Working directory for the worker (required)")
	spawnCmd.MarkFlagRequired("cwd")

	spawnCmd.Flags().StringVar(&spawnName, "name", "", "Worker name used in logs and metadata (required)")
	spawnCmd.MarkFlagRequired("name")

	spawnCmd.Flags().StringVar(&spawnSession, "session", "", "Zellij session name (default: auto-detect from ZELLIJ_SESSION_NAME)")

	spawnCmd.Flags().StringVar(&spawnMode, "mode", "build", "Agent mode: 'plan' or 'build'")
	spawnCmd.Flags().StringVar(&spawnResources, "resources", "default", "Resource profile: 'light', 'default', 'heavy', or 'ram'")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "yaks", []string{}, "Yak paths from .yaks/ to assign (can be repeated)")
	spawnCmd.Flags().StringSliceVar(&spawnYaks, "task", []string{}, "Alias for --yaks")
	spawnCmd.Flags().StringVar(&spawnYakPath, "yak-path", ".yaks", "Path to task state directory")
	spawnCmd.Flags().StringVar(&spawnRuntime, "runtime", "auto", "Runtime: 'auto', 'sandboxed', or 'native'")
	spawnCmd.Flags().StringVar(&spawnTool, "tool", "claude", "AI tool: 'opencode', 'claude', or 'cursor'")
	spawnCmd.Flags().StringVar(&spawnModel, "model", "", "Optional model override (defaults: claude='default', cursor='auto')")
	spawnCmd.Flags().BoolVar(&spawnClean, "clean", false, "Clean worker home directory before spawning")
	spawnCmd.Flags().BoolVar(&spawnAutoWorktree, "auto-worktree", false, "Automatically create and use git worktree for the task")
}
