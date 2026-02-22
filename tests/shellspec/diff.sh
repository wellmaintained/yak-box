# shellcheck shell=bash
Describe 'yak-box diff'
Skip if "yak-box binary not found" yakbox_binary_unavailable

setup_diff_env() {
	DIFF_WORKER="diff-test-worker-$$"
	DIFF_HOME="$TEST_WORK_DIR/.yak-boxes/@home/$DIFF_WORKER"

	# repo-a: no changes beyond initial commit
	mkdir -p "$DIFF_HOME/repo-a"
	setup_test_repo "$DIFF_HOME/repo-a"

	# repo-b: one extra commit so diff output is non-empty
	mkdir -p "$DIFF_HOME/repo-b"
	setup_test_repo "$DIFF_HOME/repo-b"
	echo "hello" >"$DIFF_HOME/repo-b/hello.txt"
	git -C "$DIFF_HOME/repo-b" add hello.txt
	git -C "$DIFF_HOME/repo-b" commit --quiet -m "add hello"

	# not-a-repo: plain directory that should be silently skipped
	mkdir -p "$DIFF_HOME/not-a-repo"

	export DIFF_WORKER DIFF_HOME
}

teardown_diff_env() {
	rm -rf "$DIFF_HOME"
}

Before 'setup_diff_env'
After 'teardown_diff_env'

It 'requires --name flag'
run_diff_no_name() {
	cd "$TEST_WORK_DIR" || return 1
	yak-box diff
}
When call run_diff_no_name
The status should not be success
The stderr should include "required"
End

It 'fails when worker home does not exist'
run_diff_missing() {
	cd "$TEST_WORK_DIR" || return 1
	yak-box diff --name "no-such-worker-$$"
}
When call run_diff_missing
The status should not be success
The stderr should include "no-such-worker"
End

It 'shows a section header for each git repo'
run_diff_headers() {
	cd "$TEST_WORK_DIR" || return 1
	yak-box diff --name "$DIFF_WORKER"
}
When call run_diff_headers
The status should be success
The output should include "repo-a"
The output should include "repo-b"
End

It 'does not mention plain (non-git) directories'
run_diff_skip_plain() {
	cd "$TEST_WORK_DIR" || return 1
	yak-box diff --name "$DIFF_WORKER"
}
When call run_diff_skip_plain
The status should be success
The output should not include "not-a-repo"
End

It 'reports no repos when home contains only plain directories'
run_diff_empty() {
	local empty_worker="diff-empty-$$"
	mkdir -p "$TEST_WORK_DIR/.yak-boxes/@home/$empty_worker/not-a-repo"
	cd "$TEST_WORK_DIR" || return 1
	yak-box diff --name "$empty_worker"
	rm -rf "$TEST_WORK_DIR/.yak-boxes/@home/$empty_worker"
}
When call run_diff_empty
The status should be success
The output should include "No git repos found"
End

End
