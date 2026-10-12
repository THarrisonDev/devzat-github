package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase   = "https://api.github.com"
	listLimit = 20
)

type ghClient struct {
	http        *http.Client
	token       string
	defaultRepo string
	baseURL     string
}

func newGHClient(token, defaultRepo string) *ghClient {
	return &ghClient{
		http:        &http.Client{Timeout: 15 * time.Second},
		token:       token,
		defaultRepo: defaultRepo,
		baseURL:     apiBase,
	}
}

type user struct {
	Login string `json:"login"`
}

type label struct {
	Name string `json:"name"`
}

type issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	User        user      `json:"user"`
	Labels      []label   `json:"labels"`
	Assignees   []user    `json:"assignees"`
	Body        string    `json:"body"`
	Comments    int       `json:"comments"`
	CreatedAt   time.Time `json:"created_at"`
	PullRequest *struct{} `json:"pull_request,omitempty"`
	Milestone   *struct {
		Title string `json:"title"`
	} `json:"milestone"`
}

type pull struct {
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	Draft        bool      `json:"draft"`
	Merged       bool      `json:"merged"`
	Mergeable    *bool     `json:"mergeable"`
	HTMLURL      string    `json:"html_url"`
	User         user      `json:"user"`
	Labels       []label   `json:"labels"`
	Body         string    `json:"body"`
	Comments     int       `json:"comments"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	ChangedFiles int       `json:"changed_files"`
	CreatedAt    time.Time `json:"created_at"`
	Head         struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (c *ghClient) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "devzat-github")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return apiError(resp, body)
	}
	return json.Unmarshal(body, out)
}

func apiError(resp *http.Response, body []byte) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &e)
	msg := e.Message
	if msg == "" {
		msg = resp.Status
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("not found — check the repository and number are correct")
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed — check GITHUB_TOKEN")
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit exceeded — set GITHUB_TOKEN to raise it")
		}
		return fmt.Errorf("GitHub refused the request: %s", msg)
	}
	return fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, msg)
}

func handle(c *ghClient, args string) (string, error) {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return helpText, nil
	}
	switch strings.ToLower(fields[0]) {
	case "help", "?":
		return helpText, nil
	case "issue":
		repo, num, err := parseRepoNumber(fields[1:], c.defaultRepo)
		if err != nil {
			return "", err
		}
		return c.issueLookup(repo, num)
	case "pr", "pull":
		repo, num, err := parseRepoNumber(fields[1:], c.defaultRepo)
		if err != nil {
			return "", err
		}
		return c.pullLookup(repo, num)
	case "issues":
		repo, err := parseRepo(fields[1:], c.defaultRepo)
		if err != nil {
			return "", err
		}
		return c.issueList(repo)
	case "prs", "pulls":
		repo, err := parseRepo(fields[1:], c.defaultRepo)
		if err != nil {
			return "", err
		}
		return c.pullList(repo)
	}
	return "", fmt.Errorf("unknown subcommand %q — try `gh help`", fields[0])
}

func parseRepo(args []string, def string) (string, error) {
	switch len(args) {
	case 0:
		if def == "" {
			return "", fmt.Errorf("missing repository — use `<owner/repo>` or set GITHUB_DEFAULT_REPO")
		}
		return def, nil
	case 1:
		return args[0], nil
	default:
		return "", fmt.Errorf("too many arguments — use `<owner/repo>`")
	}
}

func parseRepoNumber(args []string, def string) (string, int, error) {
	switch len(args) {
	case 0:
		return "", 0, fmt.Errorf("missing issue/PR number")
	case 1:
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return "", 0, fmt.Errorf("expected a number, got %q", args[0])
		}
		if def == "" {
			return "", 0, fmt.Errorf("missing repository — use `<owner/repo> <number>` or set GITHUB_DEFAULT_REPO")
		}
		return def, n, nil
	case 2:
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return "", 0, fmt.Errorf("expected a number, got %q", args[1])
		}
		return args[0], n, nil
	default:
		return "", 0, fmt.Errorf("too many arguments — use `<owner/repo> <number>`")
	}
}

func (c *ghClient) issueLookup(repo string, num int) (string, error) {
	var i issue
	if err := c.getJSON(fmt.Sprintf("/repos/%s/issues/%d", repo, num), &i); err != nil {
		return "", err
	}
	if i.PullRequest != nil {
		return "", fmt.Errorf("#%d is a pull request, not an issue — try `gh pr %s %d`", num, repo, num)
	}
	return formatIssue(i), nil
}

func (c *ghClient) pullLookup(repo string, num int) (string, error) {
	var p pull
	if err := c.getJSON(fmt.Sprintf("/repos/%s/pulls/%d", repo, num), &p); err != nil {
		return "", err
	}
	return formatPull(p), nil
}

func (c *ghClient) issueList(repo string) (string, error) {
	var items []issue
	if err := c.getJSON(fmt.Sprintf("/repos/%s/issues?state=open&per_page=%d", repo, listLimit), &items); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Open issues in %s**\n", repo)
	n := 0
	for _, i := range items {
		if i.PullRequest != nil {
			continue
		}
		n++
		fmt.Fprintf(&b, "`#%d` %s — @%s\n", i.Number, i.Title, i.User.Login)
	}
	if n == 0 {
		b.WriteString("_none_")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (c *ghClient) pullList(repo string) (string, error) {
	var items []pull
	if err := c.getJSON(fmt.Sprintf("/repos/%s/pulls?state=open&per_page=%d", repo, listLimit), &items); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**Open pull requests in %s**\n", repo)
	for _, p := range items {
		draft := ""
		if p.Draft {
			draft = " (draft)"
		}
		fmt.Fprintf(&b, "`#%d` %s — @%s%s\n", p.Number, p.Title, p.User.Login, draft)
	}
	if len(items) == 0 {
		b.WriteString("_none_")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func formatIssue(i issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**#%d · %s**  `%s`\n", i.Number, i.Title, i.State)
	fmt.Fprintf(&b, "_by %s · opened %s · %d comments_\n", i.User.Login, ago(i.CreatedAt), i.Comments)
	if len(i.Labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", labelNames(i.Labels))
	}
	if len(i.Assignees) > 0 {
		fmt.Fprintf(&b, "assignees: %s\n", userNames(i.Assignees))
	}
	if i.Milestone != nil && i.Milestone.Title != "" {
		fmt.Fprintf(&b, "milestone: %s\n", i.Milestone.Title)
	}
	fmt.Fprintf(&b, "%s\n", i.HTMLURL)
	if ex := excerpt(i.Body, 400); ex != "" {
		fmt.Fprintf(&b, "> %s\n", ex)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatPull(p pull) string {
	state := p.State
	if p.Merged {
		state = "merged"
	} else if p.Draft {
		state = "draft"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**#%d · %s**  `%s`\n", p.Number, p.Title, state)
	fmt.Fprintf(&b, "_by %s · opened %s · %d comments_\n", p.User.Login, ago(p.CreatedAt), p.Comments)
	fmt.Fprintf(&b, "`%s` ← `%s` · +%d −%d across %d file(s)\n", p.Base.Ref, p.Head.Ref, p.Additions, p.Deletions, p.ChangedFiles)
	if len(p.Labels) > 0 {
		fmt.Fprintf(&b, "labels: %s\n", labelNames(p.Labels))
	}
	if p.Merged {
		fmt.Fprintf(&b, "mergeable: merged\n")
	} else if p.Mergeable != nil {
		if *p.Mergeable {
			fmt.Fprintf(&b, "mergeable: yes\n")
		} else {
			fmt.Fprintf(&b, "mergeable: conflicts\n")
		}
	}
	fmt.Fprintf(&b, "%s\n", p.HTMLURL)
	if ex := excerpt(p.Body, 400); ex != "" {
		fmt.Fprintf(&b, "> %s\n", ex)
	}
	return strings.TrimRight(b.String(), "\n")
}

func labelNames(labels []label) string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = "`" + l.Name + "`"
	}
	return strings.Join(names, ", ")
}

func userNames(users []user) string {
	names := make([]string, len(users))
	for i, u := range users {
		names[i] = u.Login
	}
	return strings.Join(names, ", ")
}

func excerpt(body string, max int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	body = strings.Join(strings.Fields(body), " ")
	r := []rune(body)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return body
}

func ago(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}
