package worktree

import (
	"testing"
)

func TestDetermineWorktreePath(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		taskPath     string
		wantContains []string
	}{
		{
			name:         "basic worktree path",
			projectPath:  "/home/user/myproject",
			taskPath:     "auth/api",
			wantContains: []string{"yakthang/worktrees", "myproject", "auth-api"},
		},
		{
			name:         "single level task",
			projectPath:  "/home/user/myproject",
			taskPath:     "bugfix",
			wantContains: []string{"yakthang/worktrees", "myproject", "bugfix"},
		},
		{
			name:         "deep task path",
			projectPath:  "/home/user/myproject",
			taskPath:     "feature/auth/api/oauth",
			wantContains: []string{"yakthang/worktrees", "myproject", "feature-auth-api-oauth"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineWorktreePath(tt.projectPath, tt.taskPath)

			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("DetermineWorktreePath() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}

func TestSanitizeTaskPath(t *testing.T) {
	tests := []struct {
		name     string
		taskPath string
		want     string
	}{
		{
			name:     "simple path",
			taskPath: "auth",
			want:     "auth",
		},
		{
			name:     "path with slashes",
			taskPath: "auth/api/login",
			want:     "auth-api-login",
		},
		{
			name:     "path with colons",
			taskPath: "auth:api",
			want:     "auth-api",
		},
		{
			name:     "path with spaces",
			taskPath: "auth api login",
			want:     "auth-api-login",
		},
		{
			name:     "mixed characters",
			taskPath: "feature/auth: api login",
			want:     "feature-auth--api-login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTaskPath(tt.taskPath)
			if got != tt.want {
				t.Errorf("sanitizeTaskPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[0:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
