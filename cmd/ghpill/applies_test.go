package main

import (
	"slices"
	"testing"
)

// stripComment is the one piece of this module with a rule subtle enough to
// break silently: get the quoting wrong and every line holding a URL loses
// everything from the "//" of "https://", which reads as a change and marks
// half the roots as needing an apply.
func TestStripComment(t *testing.T) {
	cases := []struct{ in, want string }{
		{`  count = 2 # a note`, `count = 2`},
		{`url = "https://example.com/x" # note`, `url = "https://example.com/x"`},
		{`url = "https://example.com/x"`, `url = "https://example.com/x"`},
		{`# whole line`, ``},
		{`  // slash comment`, ``},
		{`a = "hash # inside string"`, `a = "hash # inside string"`},
		{`a = "escaped \" quote" # tail`, `a = "escaped \" quote"`},
		{`   spaced    out   = 1   `, `spaced out = 1`},
	}
	for _, c := range cases {
		if got := stripComment(c.in); got != c.want {
			t.Errorf("stripComment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The two directions that matter: a comment-only edit must not ask for an
// apply, and a real edit must never be swallowed.
func TestSubstantive(t *testing.T) {
	cosmetic := `@@ -7,7 +7,7 @@
   name  = var.name
-  count = 2 # old note: https://example.com/a
+  count = 2 # new note: https://example.com/b
 
   tags = ["x"]`
	if substantive(cosmetic) {
		t.Error("comment-only patch reported as substantive")
	}

	real := `@@ -7,7 +7,7 @@
-  count = 2
+  count = 3`
	if !substantive(real) {
		t.Error("real change reported as cosmetic")
	}

	// Anything we cannot read confidently must fail towards "needs applying".
	if !substantive("") {
		t.Error("absent patch must be treated as substantive")
	}
	if !substantive("@@ -1 +1 @@\n-/* block */\n+/* other */") {
		t.Error("block comments must be treated as substantive")
	}
}

func TestRootsForFile(t *testing.T) {
	roots := []string{"github", "app/dev", "app/prod", "db/prod"}
	shared := []sharedDir{{dir: "modules/network", pattern: "app/*"}}

	// .github/ is CI config, not the `github` terraform root.
	if got := rootsForFile(".github/workflows/apply.yml", roots, shared); len(got) != 0 {
		t.Errorf(".github/ matched %v, want none", got)
	}
	if got := rootsForFile("github/org.tf", roots, shared); len(got) != 1 || got[0] != "github" {
		t.Errorf("github/org.tf -> %v", got)
	}
	if got := rootsForFile("modules/network/vpc.tf", roots, shared); !slices.Equal(got, []string{"app/dev", "app/prod"}) {
		t.Errorf("shared dir -> %v, want the two app roots", got)
	}
	if got := rootsForFile("modules/network/vpc.tf", roots, nil); len(got) != 0 {
		t.Errorf("shared dir without --apply-shared -> %v, want none", got)
	}
	if got := rootsForFile("modules/network-old/vpc.tf", roots, shared); len(got) != 0 {
		t.Errorf("sibling of a shared dir -> %v, want none", got)
	}
}

func TestParseSharedDirs(t *testing.T) {
	got, err := parseSharedDirs(" modules/network/:app/* , shared:* ,")
	want := []sharedDir{{"modules/network", "app/*"}, {"shared", "*"}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("got %v, %v; want %v", got, err, want)
	}
	if got, err := parseSharedDirs(""); err != nil || len(got) != 0 {
		t.Errorf("empty -> %v, %v", got, err)
	}
	for _, bad := range []string{"modules", "modules:", ":app/*", "modules:app/["} {
		if _, err := parseSharedDirs(bad); err == nil {
			t.Errorf("%q parsed, want an error", bad)
		}
	}
}

func TestFoldApplyRunsPartialSuccessList(t *testing.T) {
	run := func(root, sha, status, conclusion string) applyRunListRun {
		return applyRunListRun{
			DisplayTitle: "terraform apply " + root + " (@octocat)",
			HeadSHA:      sha, Status: status, Conclusion: conclusion,
		}
	}
	recent := applyRunList{WorkflowRuns: []applyRunListRun{
		run("app/prod", "p3", "waiting", ""),
		run("db/prod", "a2", "completed", "success"),
		run("app/prod", "p2", "completed", "success"),
		run("github", "g2", "completed", "failure"),
	}}
	success := applyRunList{WorkflowRuns: []applyRunListRun{
		run("app/prod", "p2", "completed", "success"),
		run("db/prod", "a1", "completed", "success"),
		run("github", "g1", "completed", "success"),
	}}
	last, busy := foldApplyRuns(recent, success, []string{"app/prod", "db/prod", "github"})

	want := map[string]string{"app/prod": "p2", "db/prod": "a2", "github": "g1"}
	for root, sha := range want {
		if last[root] != sha {
			t.Errorf("last[%s] = %q, want %q", root, last[root], sha)
		}
	}
	if !busy["app/prod"] || busy["db/prod"] {
		t.Errorf("busy = %v, want only app/prod", busy)
	}
}

func TestParseRunRoot(t *testing.T) {
	if got := parseRunRoot("terraform apply net/prod (@octocat)"); got != "net/prod" {
		t.Errorf("got %q", got)
	}
	if got := parseRunRoot("something else entirely"); got != "" {
		t.Errorf("unmatched title should yield empty, got %q", got)
	}
}
