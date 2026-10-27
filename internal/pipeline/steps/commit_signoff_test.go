package steps

import (
	"testing"

	"github.com/kunchenguid/no-mistakes/internal/config"
	"github.com/kunchenguid/no-mistakes/internal/types"
)

func TestRewrittenCorrectionSignsOffWithCommittingIdentity(t *testing.T) {
	t.Parallel()
	sctx, dir, _ := agentCommitContext(t, nil)
	agentCommit(t, dir, "agent.txt", "fix: repair widget")
	gitCmd(t, dir, "config", "user.name", "Correction Committer")
	gitCmd(t, dir, "config", "user.email", "corrections@example.com")
	if err := commitAgentFixes(sctx, types.StepReview, "repair widget", "fallback"); err != nil {
		t.Fatal(err)
	}
	if got, want := gitTrailers(t, dir), "Signed-off-by: Correction Committer <corrections@example.com>"; got != want {
		t.Fatalf("rewritten commit trailers = %q, want %q", got, want)
	}
}

func TestPipelineCorrectionSignsOffWithCommittingIdentity(t *testing.T) {
	t.Parallel()
	for _, step := range []types.StepName{types.StepReview, types.StepTest, types.StepDocument, types.StepLint, types.StepPush, types.StepCI} {
		t.Run(string(step), func(t *testing.T) {
			t.Parallel()
			dir, baseSHA, headSHA := setupGitRepo(t)
			gitCmd(t, dir, "config", "user.name", "Correction Committer")
			gitCmd(t, dir, "config", "user.email", "corrections@example.com")
			sctx := newTestContextWithDBRecords(t, &mockAgent{name: "claude"}, dir, baseSHA, headSHA, config.Commands{})
			sctx.Config.CI.RevalidateRepairs = true
			writeAgentChange(t, dir)

			switch step {
			case types.StepPush:
				// Push's catch-all commit has no agent attribution.
				gitCmd(t, dir, "add", "-A")
				if err := commitPipelineCorrection(sctx.Ctx, dir, "no-mistakes: apply agent fixes", sctx.Log); err != nil {
					t.Fatal(err)
				}
			case types.StepCI:
				// A retained repair may have no producer or configured trailers.
				if _, err := (&CIStep{}).commitRepair(sctx, "", nil); err != nil {
					t.Fatal(err)
				}
			default:
				if err := commitAgentFixes(sctx, step, "repair widget", "fallback"); err != nil {
					t.Fatal(err)
				}
			}

			want := "Signed-off-by: Correction Committer <corrections@example.com>"
			if got := gitTrailers(t, dir); got != want {
				t.Fatalf("commit trailers = %q, want %q", got, want)
			}
			if got := gitCmd(t, dir, "log", "-1", "--format=%cn <%ce>"); got != "Correction Committer <corrections@example.com>" {
				t.Fatalf("committing identity = %q", got)
			}
			if got := gitCmd(t, dir, "log", "-1", "--format=%(trailers:key=Signed-off-by,valueonly)", headSHA); got != "" {
				t.Fatalf("original commit gained a sign-off: %q", got)
			}
		})
	}
}
