package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseReviews(t *testing.T) {
	const body = `{"data":{"viewer":{"login":"octocat"},"search":{"nodes":[
	  {"url":"https://github.com/acme-internal/terraform/pull/36","number":36,"title":"team only",
	   "author":{"login":"hubot"},"repository":{"nameWithOwner":"acme-internal/terraform"},
	   "reviewRequests":{"nodes":[{"requestedReviewer":{"__typename":"Team","slug":"core"}}]}},
	  {"url":"https://github.com/acme-internal/terraform/pull/37","number":37,"title":"me and a team",
	   "author":null,"repository":{"nameWithOwner":"acme-internal/terraform"},
	   "reviewRequests":{"nodes":[
	     {"requestedReviewer":{"__typename":"Team","slug":"core"}},
	     {"requestedReviewer":{"__typename":"User","login":"Octocat"}}]}},
	  {"url":"https://github.com/someone/hobby/pull/2","number":2,"title":"personal",
	   "author":{"login":"friend"},"repository":{"nameWithOwner":"someone/hobby"},
	   "reviewRequests":{"nodes":[{"requestedReviewer":{"__typename":"User","login":"octocat"}}]}},
	  {}
	]}}}`
	var resp reviewsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatal(err)
	}
	work := scope{name: "work", owners: map[string]bool{"acme-internal": true}}

	got := parseReviews(resp, work)
	if len(got) != 2 {
		t.Fatalf("got %d requests, want the 2 work ones: %+v", len(got), got)
	}
	if want := []string{"core"}; !reflect.DeepEqual(got[0].Teams, want) {
		t.Errorf("team request: Teams = %v, want %v", got[0].Teams, want)
	}
	if got[1].Teams != nil {
		t.Errorf("direct request: Teams = %v, want none", got[1].Teams)
	}

	if s := got[0].Suffix(); s != " · hubot · for core" {
		t.Errorf("team suffix = %q", s)
	}
	if s := got[1].Suffix(); s != "" {
		t.Errorf("direct request from a ghost author: suffix = %q, want empty", s)
	}

	if got := parseReviews(resp, scope{name: "personal", owners: work.owners}); len(got) != 1 || got[0].Number != 2 {
		t.Errorf("personal scope kept %+v, want only someone/hobby#2", got)
	}
}

func TestPickerReviews(t *testing.T) {
	r := ReviewRequest{Title: "add template", URL: "https://github.com/o/r/pull/9", Number: 9,
		Repo: "o/r", Author: "hubot", Teams: []string{"core"}}
	n := Notification{ID: "1", Reason: "review_requested"}
	n.Subject.URL = "https://api.github.com/repos/o/r/pulls/9"

	items := pickerItems(PRCache{Reviews: []ReviewRequest{r}, Notifications: []Notification{n}})
	if len(items) != 1 {
		t.Fatalf("got %d rows, want the review row alone", len(items))
	}
	if !strings.Contains(items[0].label, "[o/r#9] add template · hubot · for core") {
		t.Errorf("review row %q is missing its head or suffix", items[0].label)
	}
	if !strings.HasSuffix(items[0].label, "review_requested") {
		t.Errorf("review row %q does not carry the notification reason", items[0].label)
	}
	if items[0].url != r.URL {
		t.Errorf("review row opens %q, want %q", items[0].url, r.URL)
	}
}

func TestPickerDependabot(t *testing.T) {
	human := ReviewRequest{Title: "add template", URL: "https://github.com/o/r/pull/9", Number: 9,
		Repo: "o/r", Author: "hubot"}
	bot := ReviewRequest{Title: "bump boto3", URL: "https://github.com/o/r/pull/10", Number: 10,
		Repo: "o/r", Author: "dependabot", Teams: []string{"core"}}

	items := pickerItems(PRCache{Reviews: []ReviewRequest{bot, human}})
	if len(items) != 3 {
		t.Fatalf("got %d rows, want review, divider, dependabot: %+v", len(items), items)
	}
	if !strings.HasPrefix(items[0].label, glyphReview) || items[0].url != human.URL {
		t.Errorf("first row %q should be the human review", items[0].label)
	}
	if !items[1].divider || !strings.Contains(items[1].label, "Dependabot") {
		t.Errorf("second row %q should be the Dependabot divider", items[1].label)
	}
	if !strings.HasPrefix(items[2].label, glyphDependabot) || items[2].url != bot.URL {
		t.Errorf("third row %q should be the dependabot PR", items[2].label)
	}
	if !strings.HasSuffix(items[2].label, "bump boto3 · for core") {
		t.Errorf("dependabot row %q should drop the author but keep the team", items[2].label)
	}
}

func TestPickerRenovate(t *testing.T) {
	dep := ReviewRequest{Title: "bump boto3", URL: "https://github.com/o/r/pull/10", Number: 10,
		Repo: "o/r", Author: "dependabot"}
	hosted := ReviewRequest{Title: "update docker", URL: "https://github.com/o/r/pull/11", Number: 11,
		Repo: "o/r", Author: "renovate"}
	self := ReviewRequest{Title: "update nginx", URL: "https://github.com/o/r/pull/12", Number: 12,
		Repo: "o/r", Author: "acme-renovate", Teams: []string{"core"}}

	items := pickerItems(PRCache{Reviews: []ReviewRequest{self, dep, hosted}})
	if len(items) != 4 {
		t.Fatalf("got %d rows, want dependabot, divider, two renovate: %+v", len(items), items)
	}
	if !strings.HasPrefix(items[0].label, glyphDependabot) || items[0].url != dep.URL {
		t.Errorf("first row %q should be the dependabot PR", items[0].label)
	}
	if !items[1].divider || !strings.Contains(items[1].label, "Renovate") {
		t.Errorf("second row %q should be the Renovate divider", items[1].label)
	}
	for i, want := range []ReviewRequest{self, hosted} {
		if got := items[2+i]; !strings.HasPrefix(got.label, glyphRenovate) || got.url != want.URL {
			t.Errorf("row %d %q should be renovate PR %s", 2+i, got.label, want.URL)
		}
	}
	if !strings.HasSuffix(items[2].label, "update nginx · for core") {
		t.Errorf("renovate row %q should drop the author but keep the team", items[2].label)
	}
}

func TestStaleReviewRequest(t *testing.T) {
	notif := func(reason, pull string) Notification {
		var n Notification
		n.Reason = reason
		n.Subject.URL = "https://api.github.com/repos/acme-internal/cache/pulls/" + pull
		return n
	}
	listed := reviewsResult{Complete: true, Requests: []ReviewRequest{
		{URL: "https://github.com/acme-internal/cache/pull/8"},
	}}

	cases := []struct {
		name    string
		reviews reviewsResult
		n       Notification
		want    bool
	}{
		{"still requested", listed, notif("review_requested", "8"), false},
		{"already reviewed", listed, notif("review_requested", "7"), true},
		{"search failed", reviewsResult{}, notif("review_requested", "7"), false},
		{"other reason", listed, notif("mention", "7"), false},
	}
	for _, c := range cases {
		if got := c.reviews.stale(c.n); got != c.want {
			t.Errorf("%s: stale = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBotReviewRequest(t *testing.T) {
	notif := func(reason, pull string) Notification {
		var n Notification
		n.Reason = reason
		n.Subject.URL = "https://api.github.com/repos/acme-internal/deploy/pulls/" + pull
		return n
	}
	reviews := reviewsResult{Complete: true, Requests: []ReviewRequest{
		{URL: "https://github.com/acme-internal/deploy/pull/1", Author: "acme-renovate", Title: "chore(deps): update nginx"},
		{URL: "https://github.com/acme-internal/deploy/pull/2", Author: "dependabot"},
		{URL: "https://github.com/acme-internal/deploy/pull/3", Author: "monalisa", Title: "chore: tidy up"},
	}}

	cases := []struct {
		name string
		n    Notification
		want bool
	}{
		{"renovate", notif("review_requested", "1"), true},
		{"dependabot", notif("review_requested", "2"), true},
		{"human chore", notif("review_requested", "3"), false},
		{"not in search", notif("review_requested", "4"), false},
		{"other reason on a bot PR", notif("mention", "1"), false},
	}
	for _, c := range cases {
		if got := reviews.fromBot(c.n); got != c.want {
			t.Errorf("%s: fromBot = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestQuietReview(t *testing.T) {
	notif := func(reason, repo string) Notification {
		var n Notification
		n.Reason = reason
		n.Repository.FullName = repo
		n.Subject.URL = "https://api.github.com/repos/" + repo + "/pulls/5"
		return n
	}
	quiet := map[string]bool{"acme-internal/sandbox": true}
	reviews := reviewsResult{Complete: true, Requests: []ReviewRequest{
		{URL: "https://github.com/acme-internal/deploy/pull/5", Author: "acme-renovate"},
		{URL: "https://github.com/acme-internal/sandbox/pull/5", Author: "monalisa"},
		{URL: "https://github.com/acme-internal/terraform/pull/5", Author: "monalisa"},
	}}

	cases := []struct {
		name string
		n    Notification
		want bool
	}{
		{"quiet repo", notif("review_requested", "acme-internal/sandbox"), true},
		{"bot elsewhere", notif("review_requested", "acme-internal/deploy"), true},
		{"person elsewhere", notif("review_requested", "acme-internal/terraform"), false},
		{"mention in quiet repo", notif("mention", "acme-internal/sandbox"), false},
	}
	for _, c := range cases {
		if got := quietReview(c.n, reviews, quiet); got != c.want {
			t.Errorf("%s: quietReview = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPickerGroupsReviews(t *testing.T) {
	req := func(repo string, n int, author, title string) ReviewRequest {
		return ReviewRequest{Title: title, URL: fmt.Sprintf("https://github.com/%s/pull/%d", repo, n),
			Number: n, Repo: repo, Author: author}
	}
	reqs := []ReviewRequest{
		req("o/web", 19, "kim", "feat: [SITE-1] [SITE-2] count failures"),
		req("o/beta", 6, "sam", "ci: [OPS-101] Pin the actions"),
		req("o/gamma", 5, "sam", "ci: [OPS-102] Pin the actions"),
		req("o/tools", 20, "sam", "ci: [OPS-103] Add a lint job"),
		req("o/web", 12, "kim", "fix: typo"),
		req("o/alpha", 21, "sam", "ci: [OPS-102] Pin the actions"),
		req("o/notes", 12, "kim", "SITE-204 Rotate the key"),
		req("o/web", 13, "kim", "fix: broken link"),
		req("o/tools", 19, "dependabot", "chore(deps): bump"),
		req("o/manual", 1, "sam", "Add a lint job"),
		req("o/manual", 2, "sam", "Add a lint job"),
	}

	var got []string
	for _, it := range pickerItems(PRCache{Reviews: reqs}) {
		got = append(got, it.label)
	}
	want := []string{
		"[o/tools#20] ci: [OPS-103] Add a lint job · sam",
		"[o/notes#12] SITE-204 Rotate the key · kim",
		"[o/manual#1] Add a lint job · sam",
		"[o/manual#2] Add a lint job · sam",
		"──── web · 3 ",
		"#12 fix: typo · kim",
		"#13 fix: broken link · kim",
		"#19 feat: [SITE-1] [SITE-2] count failures · kim",
		"──── ci: Pin the actions · 3 repos ",
		"[o/alpha#21] OPS-102 · sam",
		"[o/beta#6] OPS-101 · sam",
		"[o/gamma#5] OPS-102 · sam",
		"──── Dependabot ",
	}
	if len(got) != len(want)+1 {
		t.Fatalf("got %d rows, want %d:\n%s", len(got), len(want)+1, strings.Join(got, "\n"))
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("row %d = %q, want it to contain %q", i, got[i], w)
		}
	}
}

func TestSplitTickets(t *testing.T) {
	cases := []struct{ title, rest, tickets string }{
		{"ci: [OPS-102] Pin the actions", "ci: Pin the actions", "OPS-102"},
		{"SITE-204 Rotate the key", "Rotate the key", "SITE-204"},
		{"feat: [SITE-1] [SITE-2] count", "feat: count", "SITE-1 SITE-2"},
		{"Sync labels", "Sync labels", ""},
	}
	for _, c := range cases {
		if rest, tickets := splitTickets(c.title); rest != c.rest || tickets != c.tickets {
			t.Errorf("splitTickets(%q) = %q, %q, want %q, %q", c.title, rest, tickets, c.rest, c.tickets)
		}
	}
}
