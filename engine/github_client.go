package engine

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ErrResourceNotFound is returned when a GitHub API resource is not found (HTTP 404).
var ErrResourceNotFound = errors.New("resource not found")

// GitHubIssue represents a GitHub issue as returned by the API.
type GitHubIssue struct {
	ID        int64  `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	HTMLURL   string `json:"html_url"`
}

// GitHubComment represents a comment on a GitHub issue.
type GitHubComment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

// RepoLabel represents a label in a GitHub repository.
type RepoLabel struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type asyncResult struct {
	Resp *http.Response
	Err  error
}

type GitHubClient interface {
	GetRepoLabels() ([]RepoLabel, error)
	CreateRepoLabel(name, color string) error
	GetIssueLabels(issueNumber int) ([]RepoLabel, error)
	SetIssueLabels(issueNumber int, labelNames []string) error
	ListOpenIssues() ([]GitHubIssue, error)
	GetIssueByNumber(number int) (*GitHubIssue, error)
	CreateIssue(title, body string) (*GitHubIssue, error)
	UpdateIssueTitle(issueNumber int, title string) error
	UpdateIssueBody(issueNumber int, body string) error
	PostComment(issueNumber int, body string) error
	CloseIssueByNumber(issueNumber int) error
	GetIssueComments(issueNumber int) ([]GitHubComment, error)
}

type gitHubClient struct {
	owner    string
	repo     string
	baseURL  string
	apiToken string
	client   HTTPDoer
}

type GitHubClientOption func(*gitHubClient)

func WithHTTPDoer(client HTTPDoer) GitHubClientOption {
	return func(g *gitHubClient) {
		g.client = client
	}
}

func NewGitHubClient(owner, repo, token, baseURL string, opts ...GitHubClientOption) GitHubClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: os.Getenv("FORGEFIX_INSECURE_TLS") == "1",
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	g := &gitHubClient{
		owner:    owner,
		repo:     repo,
		baseURL:  baseURL,
		apiToken: token,
		client:   httpClient,
	}
	for _, opt := range opts {
		opt(g)
	}

	if os.Getenv("FORGEFIX_INSECURE_TLS") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] TLS verification DISABLED via FORGEFIX_INSECURE_TLS=1\n")
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] HTTP client configured: Proxy=%v, TLSInsecure=%v\n",
		transport.Proxy != nil, os.Getenv("FORGEFIX_INSECURE_TLS") == "1")

	return g
}

func (c *gitHubClient) doRequestAsync(req *http.Request) <-chan asyncResult {
	ch := make(chan asyncResult, 1)
	go func() {
		resp, err := c.client.Do(req)
		ch <- asyncResult{Resp: resp, Err: err}
		close(ch)
	}()
	return ch
}

func (c *gitHubClient) url(path string) string {
	return fmt.Sprintf("%s/repos/%s/%s/issues%s", c.baseURL, c.owner, c.repo, path)
}

func (c *gitHubClient) repoURL(path string) string {
	return fmt.Sprintf("%s/repos/%s/%s%s", c.baseURL, c.owner, c.repo, path)
}

func (c *gitHubClient) newRequest(method, requestURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	return req, nil
}

func (c *gitHubClient) GetRepoLabels() ([]RepoLabel, error) {
	queryURL := c.repoURL("/labels")
	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", queryURL)

	req, err := c.newRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching repo labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching repo labels: HTTP %d", resp.StatusCode)
	}

	var labels []RepoLabel
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, fmt.Errorf("decoding repo labels: %w", err)
	}
	return labels, nil
}

func (c *gitHubClient) CreateRepoLabel(name, color string) error {
	postURL := c.repoURL("/labels")
	body := map[string]string{"name": name, "color": color}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub POST %s\n", postURL)

	req, err := c.newRequest("POST", postURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("creating repo label: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating repo label: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *gitHubClient) GetIssueLabels(issueNumber int) ([]RepoLabel, error) {
	queryURL := c.url(fmt.Sprintf("/%d/labels", issueNumber))
	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", queryURL)

	req, err := c.newRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching issue labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching issue labels: HTTP %d", resp.StatusCode)
	}

	var labels []RepoLabel
	if err := json.NewDecoder(resp.Body).Decode(&labels); err != nil {
		return nil, fmt.Errorf("decoding issue labels: %w", err)
	}
	return labels, nil
}

func (c *gitHubClient) SetIssueLabels(issueNumber int, labelNames []string) error {
	putURL := c.url(fmt.Sprintf("/%d/labels", issueNumber))
	body := map[string]interface{}{"labels": labelNames}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub PUT %s\n", putURL)

	req, err := c.newRequest("PUT", putURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("setting issue labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("setting issue labels: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *gitHubClient) ListOpenIssues() ([]GitHubIssue, error) {
	params := url.Values{}
	params.Set("state", "open")
	params.Set("per_page", "100")
	queryURL := c.url("") + "?" + params.Encode()

	req, err := c.newRequest("GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating GET request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching issues: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading issues response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch issues failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var issues []GitHubIssue
	if err := json.NewDecoder(bytes.NewReader(respBody)).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decoding issues: %w", err)
	}
	return issues, nil
}

func (c *gitHubClient) GetIssueByNumber(number int) (*GitHubIssue, error) {
	getURL := c.url(fmt.Sprintf("/%d", number))

	req, err := c.newRequest("GET", getURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", getURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("fetch issue #%d: %w", number, ErrResourceNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch issue #%d failed (%d): %s", number, resp.StatusCode, string(respBody))
	}

	var issue GitHubIssue
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *gitHubClient) CreateIssue(title, body string) (*GitHubIssue, error) {
	bodyMap := map[string]string{
		"title": title,
		"body":  body,
	}
	jsonBody, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("marshaling issue body: %w", err)
	}

	postURL := c.url("")
	req, err := c.newRequest("POST", postURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("creating POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending create issue request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading create issue response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("issue creation failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var createdIssue GitHubIssue
	if err := json.NewDecoder(bytes.NewReader(respBody)).Decode(&createdIssue); err != nil {
		return nil, fmt.Errorf("decoding created issue: %w", err)
	}
	return &createdIssue, nil
}

func (c *gitHubClient) UpdateIssueTitle(issueNumber int, title string) error {
	bodyMap := map[string]string{"title": title}
	jsonBody, _ := json.Marshal(bodyMap)
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, err := c.newRequest("PATCH", patchURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub PATCH %s (title update)\n", patchURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("title update failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *gitHubClient) UpdateIssueBody(issueNumber int, body string) error {
	bodyMap := map[string]string{"body": body}
	jsonBody, _ := json.Marshal(bodyMap)
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, err := c.newRequest("PATCH", patchURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub PATCH %s\n", patchURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *gitHubClient) PostComment(issueNumber int, body string) error {
	bodyMap := map[string]string{"body": body}
	jsonBody, _ := json.Marshal(bodyMap)
	postURL := c.url(fmt.Sprintf("/%d/comments", issueNumber))
	req, err := c.newRequest("POST", postURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub POST comment on issue #%d\n", issueNumber)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("comment posting failed on #%d: %w", issueNumber, ErrResourceNotFound)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("comment posting failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *gitHubClient) CloseIssueByNumber(issueNumber int) error {
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, err := c.newRequest("PATCH", patchURL, strings.NewReader(`{"state":"closed"}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("closing issue #%d: %w", issueNumber, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading close issue response: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("issue close failed on #%d: %w", issueNumber, ErrResourceNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("issue close failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *gitHubClient) GetIssueComments(issueNumber int) ([]GitHubComment, error) {
	req, err := c.newRequest("GET", c.url(fmt.Sprintf("/%d/comments", issueNumber)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("comments fetch failed on #%d: %w", issueNumber, ErrResourceNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comments fetch failed: %d", resp.StatusCode)
	}

	var comments []GitHubComment
	_ = json.NewDecoder(resp.Body).Decode(&comments)
	return comments, nil
}
