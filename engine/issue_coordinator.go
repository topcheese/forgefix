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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ForgeFix/engine/housekeeper"
)

var ErrResourceNotFound = errors.New("resource not found")

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type asyncResult struct {
	Resp *http.Response
	Err  error
}

func (c *IssueCoordinator) doRequestAsync(req *http.Request) <-chan asyncResult {
	ch := make(chan asyncResult, 1)
	go func() {
		resp, err := c.client.Do(req)
		ch <- asyncResult{Resp: resp, Err: err}
		close(ch)
	}()
	return ch
}

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

type GitHubComment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

type ErrorDetails struct {
	TestName     string `json:"test_name"`
	Package      string `json:"package"`
	FilePath     string `json:"file_path"`
	LineNumber   int    `json:"line_number"`
	ErrorMessage string `json:"error_message"`
	StackTrace   string `json:"stack_trace"`
}

type AuditEntry struct {
	Timestamp   time.Time
	IssueNumber int
	CommitHash  string
	TestName    string
	Message     string
}

type IssueCoordinator struct {
	mu                   sync.RWMutex
	owner                string
	repo                 string
	baseURL              string
	apiToken             string
	issueCache           map[string]*GitHubIssue
	cacheExpiry          map[string]int
	client               HTTPDoer
	tracked              map[string]int
	inactive             bool
	configDir            string
	failureDecayDuration time.Duration
	failureDecaySet      bool
	ledger               *LedgerEngine
	titleValidator       *IssueTitleValidator
}

func NewIssueCoordinator(owner, repo, token, baseURL string) *IssueCoordinator {
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
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	coordinator := &IssueCoordinator{
		owner:         owner,
		repo:          repo,
		baseURL:       baseURL,
		apiToken:      token,
		issueCache:    make(map[string]*GitHubIssue),
		cacheExpiry:   make(map[string]int),
		client:        client,
		tracked:       make(map[string]int),
		titleValidator: NewIssueTitleValidator(),
	}
	coordinator.inactive = coordinator.isPlaceholderConfig()

	if os.Getenv("FORGEFIX_INSECURE_TLS") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] TLS verification DISABLED via FORGEFIX_INSECURE_TLS=1\n")
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] HTTP client configured: Proxy=%v, TLSInsecure=%v\n",
		transport.Proxy != nil, os.Getenv("FORGEFIX_INSECURE_TLS") == "1")

	return coordinator
}

func NewCoordinatorFromConfig(cfg *Config, configDir string, aiMode bool) *IssueCoordinator {
	if cfg.GitHub == nil || cfg.GitHub.Token == "" || cfg.GitHub.Owner == "" || cfg.GitHub.Repo == "" {
		return nil
	}
	if aiMode && !cfg.AutoIssueManagement {
		return nil
	}
	baseURL := cfg.GitHub.BaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	coord := NewIssueCoordinator(cfg.GitHub.Owner, cfg.GitHub.Repo, cfg.GitHub.Token, baseURL)
	coord.SetConfigDir(configDir)
	if cfg.FailureDecaySeconds > 0 {
		coord.SetFailureDecay(cfg.FailureDecaySeconds)
	}
	ledger, err := LoadLedger(configDir)
	if err == nil && ledger != nil {
		coord.SetLedger(ledger)
	}
	return coord
}

func (c *IssueCoordinator) isPlaceholderConfig() bool {
	if c.owner == "" || c.repo == "" || c.apiToken == "" {
		return true
	}
	placeholders := []string{
		"your-org",
		"your-repo",
		"your-github-token",
		"your-repo-token",
	}
	for _, ph := range placeholders {
		if c.owner == ph || c.repo == ph || c.apiToken == ph {
			return true
		}
	}
	return false
}

func (c *IssueCoordinator) SetConfigDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configDir = dir
}

func (c *IssueCoordinator) SetFailureDecay(seconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seconds > 0 {
		c.failureDecayDuration = time.Duration(seconds) * time.Second
		c.failureDecaySet = true
	}
}

func (c *IssueCoordinator) SetLedger(ledger *LedgerEngine) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ledger = ledger
}

func (c *IssueCoordinator) GetLedger() *LedgerEngine {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ledger
}

func (c *IssueCoordinator) isInactive() bool {
	return c.inactive
}

func (c *IssueCoordinator) IsActive() bool {
	return !c.inactive
}

// url constructs api endpoints cleanly, eliminating three duplicate helper methods
func (c *IssueCoordinator) url(path string) string {
	return fmt.Sprintf("%s/repos/%s/%s/issues%s", c.baseURL, c.owner, c.repo, path)
}

// repoURL constructs repo-level API endpoints (for labels, etc.)
func (c *IssueCoordinator) repoURL(path string) string {
	return fmt.Sprintf("%s/repos/%s/%s%s", c.baseURL, c.owner, c.repo, path)
}

type RepoLabel struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (c *IssueCoordinator) GetRepoLabels() ([]RepoLabel, error) {
	queryURL := c.repoURL("/labels")
	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", queryURL)

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) CreateRepoLabel(name, color string) error {
	postURL := c.repoURL("/labels")
	body := map[string]string{"name": name, "color": color}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub POST %s\n", postURL)

	req, err := http.NewRequest("POST", postURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) GetIssueLabels(issueNumber int) ([]RepoLabel, error) {
	queryURL := c.url(fmt.Sprintf("/%d/labels", issueNumber))
	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", queryURL)

	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) SetIssueLabels(issueNumber int, labelNames []string) error {
	putURL := c.url(fmt.Sprintf("/%d/labels", issueNumber))
	body := map[string]interface{}{"labels": labelNames}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub PUT %s\n", putURL)

	req, err := http.NewRequest("PUT", putURL, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

var colorNameToHex = map[string]string{
	"black":   "000000",
	"white":   "ffffff",
	"red":     "e74c3c",
	"green":   "2ecc71",
	"yellow":  "f1c40f",
	"blue":    "3498db",
	"magenta": "9b59b6",
	"cyan":    "1abc9c",
	"orange":  "e67e22",
	"purple":  "8e44ad",
	"grey":    "95a5a6",
	"gray":    "95a5a6",
	"hiblack": "555555",
	"hiwhite": "cccccc",
	"hired":   "ff6b6b",
	"higreen": "69db7c",
	"hiyellow": "ffd93d",
	"hiblue":  "74b9ff",
	"himagenta": "dda0dd",
	"hicyan":  "81ecec",
}

// labelHexColor converts a color name or hex string to a 6-char hex code
// suitable for the Repo API (no # prefix). If the input is already a valid
// 6-char hex (optionally with #), it is normalized. Otherwise the named color
// map is consulted, falling back to "cccccc".
func labelHexColor(color string) string {
	if color == "" {
		return "cccccc"
	}
	color = strings.ToLower(strings.TrimSpace(color))
	if h, ok := colorNameToHex[color]; ok {
		return h
	}
	color = strings.TrimPrefix(color, "#")
	if len(color) == 6 {
		for _, c := range color {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return "cccccc"
			}
		}
		return color
	}
	return "cccccc"
}

// categoryColor maps a label category name to a default hex color for
// newly created labels. Status labels use their StatusDef color instead.
func categoryColor(cat string) string {
	switch cat {
	case "status":
		return "3498db"
	case "type":
		return "2ecc71"
	case "version":
		return "e67e22"
	default:
		return "cccccc"
	}
}

// ensureCategoryLabelsExist fetches all repo labels once and creates any
// labels from configured categories that are missing. This is called once
// per SyncSpecs run, not per spec.
func (c *IssueCoordinator) ensureCategoryLabelsExist(wc *WorkflowConfig) {
	if wc == nil || len(wc.LabelCategories) == 0 {
		return
	}

	repoLabels, err := c.GetRepoLabels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch repo labels: %v\n", err)
		return
	}

	existing := make(map[string]bool)
	for _, l := range repoLabels {
		existing[l.Name] = true
	}

	for catName, cat := range wc.LabelCategories {
		for _, defaultLabel := range cat.Defaults {
			if existing[defaultLabel] {
				continue
			}
			color := categoryColor(catName)
			if err := c.CreateRepoLabel(defaultLabel, color); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to create label %s: %v\n", defaultLabel, err)
			} else {
				fmt.Fprintf(os.Stderr, "Created label %s\n", defaultLabel)
			}
		}
	}
}

// desiredLabelsForSpec returns the list of label names that should be applied
// to a spec's Repo issue based on its status, type, and version fields.
func (c *IssueCoordinator) desiredLabelsForSpec(spec *SpecFile, wc *WorkflowConfig) []string {
	if wc == nil || spec.RepoIssue <= 0 {
		return nil
	}

	var labels []string

	// Status label from StatusDef
	if sd, ok := wc.GetStatusDef(spec.Status); ok && sd.RepoLabel != "" {
		labels = append(labels, sd.RepoLabel)
	}

	// Type label from spec frontmatter
	if spec.Type != "" && wc.LabelCategories != nil {
		if l := wc.CategoryLabelFor("type", spec.Type); l != "" {
			labels = append(labels, l)
		}
	}

	// Version label from spec frontmatter
	if spec.Version != "" && wc.LabelCategories != nil {
		if l := wc.CategoryLabelFor("version", spec.Version); l != "" {
			labels = append(labels, l)
		}
	}

	return labels
}

// syncSpecLabels reconciles the Repo issue labels for a spec to match its
// desired labels (status, type, version) as defined in the workflow config.
// It preserves non-category labels already on the issue. All failures are
// logged as warnings, never returned as errors.
func (c *IssueCoordinator) syncSpecLabels(spec *SpecFile, wc *WorkflowConfig) {
	if spec.RepoIssue <= 0 || wc == nil {
		return
	}

	desiredLabels := c.desiredLabelsForSpec(spec, wc)
	if len(desiredLabels) == 0 {
		return
	}

	// Fetch current labels on the issue
	issueLabels, err := c.GetIssueLabels(spec.RepoIssue)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch issue labels for #%d: %v\n", spec.RepoIssue, err)
		return
	}

	currentNames := make(map[string]bool)
	for _, l := range issueLabels {
		currentNames[l.Name] = true
	}

	// Collect all category-controlled label names
	categoryLabelNames := wc.AllCategoryLabelNames()

	// Check if update is needed: any desired label missing, or stale category label present
	needsUpdate := false
	for _, d := range desiredLabels {
		if !currentNames[d] {
			needsUpdate = true
			break
		}
	}
	if !needsUpdate {
		for _, l := range issueLabels {
			if categoryLabelNames[l.Name] {
				isDesired := false
				for _, d := range desiredLabels {
					if l.Name == d {
						isDesired = true
						break
					}
				}
				if !isDesired {
					needsUpdate = true
					break
				}
			}
		}
	}
	if !needsUpdate {
		return
	}

	// Build new label set: preserve non-category labels + desired labels
	var newLabels []string
	for _, l := range issueLabels {
		if !categoryLabelNames[l.Name] {
			newLabels = append(newLabels, l.Name)
		}
	}
	newLabels = append(newLabels, desiredLabels...)

	if err := c.SetIssueLabels(spec.RepoIssue, newLabels); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to set issue labels for #%d: %v\n", spec.RepoIssue, err)
		return
	}
	fmt.Fprintf(os.Stderr, "Updated issue #%d labels to %v for spec %s\n", spec.RepoIssue, desiredLabels, spec.SpecID)
}

func (c *IssueCoordinator) ListOpenIssues() ([]GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	params := url.Values{}
	params.Set("state", "open")
	params.Set("per_page", "100")
	queryURL := c.url("") + "?" + params.Encode()

	req, _ := http.NewRequest("GET", queryURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub GET %s\n", queryURL)

	resp, err := c.client.Do(req)
	if err != nil {
		var netErr net.Error
		var urlErr *url.Error
		fmt.Fprintf(os.Stderr, "[DEBUG] GET request error: %v (type: %T)\n", err, err)
		if errors.As(err, &urlErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] URL Error: %v, Timeout: %v, Temporary: %v\n", urlErr.Err, urlErr.Timeout(), urlErr.Temporary())
		}
		if errors.As(err, &netErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Net Error: Timeout=%v, Temporary=%v\n", netErr.Timeout(), netErr.Temporary())
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "[DEBUG] GET response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "[DEBUG] GET response body: %s\n", string(respBody))
		return nil, fmt.Errorf("fetch issues failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var issues []GitHubIssue
	if err := json.NewDecoder(bytes.NewReader(respBody)).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decoding issues: %w", err)
	}
	return issues, nil
}

func (c *IssueCoordinator) CheckExistingIssue(testName string) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	c.mu.RLock()
	if cached, exists := c.issueCache[testName]; exists {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	issues, err := c.ListOpenIssues()
	if err != nil {
		return nil, err
	}

	for _, issue := range issues {
		if issue.Title == testName && issue.State == "open" {
			c.mu.Lock()
			c.issueCache[testName] = &issue
			c.tracked[testName] = issue.Number
			c.mu.Unlock()
			fmt.Fprintf(os.Stderr, "[DEBUG] Found existing open issue #%d for: %s\n", issue.Number, testName)
			return &issue, nil
		}
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] GET no open issues with matching title: %s\n", testName)
	return nil, fmt.Errorf("no open issues matching title: %s", testName)
}

func (c *IssueCoordinator) FindExistingIssueBySimilarTitle(title string) (*GitHubIssue, error) {
	issues, err := c.ListOpenIssues()
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("no open issues found")
	}

	normalizedNew := NormalizeTitle(title)
	if normalizedNew == "" {
		return nil, fmt.Errorf("empty title after normalization")
	}

	var bestMatch GitHubIssue
	bestSimilarity := DuplicateThreshold

	for _, issue := range issues {
		normalizedExisting := NormalizeTitle(issue.Title)
		sim := SimilarityRatio(normalizedNew, normalizedExisting)
		if sim > bestSimilarity {
			bestSimilarity = sim
			bestMatch = issue
		}
	}

	if bestMatch.Number > 0 {
		return &bestMatch, nil
	}
	return nil, fmt.Errorf("no similar open issues found")
}

func (c *IssueCoordinator) markSpecAsDuplicateIfNeeded(spec *SpecFile) {
	if existing, err := c.FindExistingIssueBySimilarTitle(spec.Title); err == nil {
		dupSuffix := fmt.Sprintf(" [Dupe → #%d]", existing.Number)
		spec.Title += dupSuffix
		ref := fmt.Sprintf("\n\n> This spec has been identified as a duplicate of issue `#%d` (%s).", existing.Number, existing.Title)
		spec.Body += ref
		fmt.Printf("Detected duplicate: spec title similar to issue #%d (%s). Marking as [Dupe].\n", existing.Number, existing.Title)
	}
}

func (c *IssueCoordinator) UpdateIssueTitle(issueNumber int, title string) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	bodyMap := map[string]string{"title": title}
	jsonBody, _ := json.Marshal(bodyMap)
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, _ := http.NewRequest("PATCH", patchURL, strings.NewReader(string(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) CreateIssueWithBody(title, body string) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	if err := c.titleValidator.Validate(title); err != nil {
		return nil, fmt.Errorf("issue title validation failed: %w", err)
	}

	bodyMap := map[string]string{
		"title": title,
		"body":  body,
	}
	jsonBody, _ := json.Marshal(bodyMap)

	postURL := c.url("")
	req, err := http.NewRequest("POST", postURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub POST %s\n", postURL)
	fmt.Fprintf(os.Stderr, "[DEBUG] POST body: %s\n", string(jsonBody))

	resp, err := c.client.Do(req)
	if err != nil {
		var netErr net.Error
		var urlErr *url.Error
		fmt.Fprintf(os.Stderr, "[DEBUG] POST request error: %v (type: %T)\n", err, err)
		if errors.As(err, &urlErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] URL Error: %v, Timeout: %v, Temporary: %v\n", urlErr.Err, urlErr.Timeout(), urlErr.Temporary())
		}
		if errors.As(err, &netErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Net Error: Timeout=%v, Temporary=%v\n", netErr.Timeout(), netErr.Temporary())
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "[DEBUG] POST response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusCreated {
		fmt.Fprintf(os.Stderr, "[DEBUG] POST response body: %s\n", string(respBody))
		return nil, fmt.Errorf("issue creation failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var createdIssue GitHubIssue
	_ = json.NewDecoder(bytes.NewReader(respBody)).Decode(&createdIssue)

	c.mu.Lock()
	c.issueCache[title] = &createdIssue
	c.tracked[title] = createdIssue.Number
	c.mu.Unlock()

	return &createdIssue, nil
}

func (c *IssueCoordinator) CreateIssue(testName string, details *ErrorDetails) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	if err := c.titleValidator.Validate(testName); err != nil {
		return nil, fmt.Errorf("issue title validation failed: %w", err)
	}

	c.mu.RLock()
	if existing, exists := c.issueCache[testName]; exists {
		c.mu.RUnlock()
		return existing, nil
	}
	c.mu.RUnlock()

	bodyMap := map[string]string{
		"title": testName,
		"body":  c.generateIssueBody(testName, details),
	}
	jsonBody, _ := json.Marshal(bodyMap)

	postURL := c.url("")
	req, err := http.NewRequest("POST", postURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub POST %s\n", postURL)
	fmt.Fprintf(os.Stderr, "[DEBUG] POST body: %s\n", string(jsonBody))

	resp, err := c.client.Do(req)
	if err != nil {
		var netErr net.Error
		var urlErr *url.Error
		fmt.Fprintf(os.Stderr, "[DEBUG] POST request error: %v (type: %T)\n", err, err)
		if errors.As(err, &urlErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] URL Error: %v, Timeout: %v, Temporary: %v\n", urlErr.Err, urlErr.Timeout(), urlErr.Temporary())
		}
		if errors.As(err, &netErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Net Error: Timeout=%v, Temporary=%v\n", netErr.Timeout(), netErr.Temporary())
		}
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "[DEBUG] POST response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusCreated {
		fmt.Fprintf(os.Stderr, "[DEBUG] POST response body: %s\n", string(respBody))
		return nil, fmt.Errorf("issue creation failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var createdIssue GitHubIssue
	_ = json.NewDecoder(bytes.NewReader(respBody)).Decode(&createdIssue)

	c.mu.Lock()
	c.issueCache[testName] = &createdIssue
	c.tracked[testName] = createdIssue.Number
	c.mu.Unlock()

	return &createdIssue, nil
}

func (c *IssueCoordinator) PostResolutionComment(issueNumber int, spec *SpecFile) error {
	closedRef := fmt.Sprintf("#%d", issueNumber)
	specRef := spec.SpecID
	if spec.FilePath != "" {
		specRef = fmt.Sprintf("[%s](%s)", spec.SpecID, specFileWebURL(c.baseURL, c.owner, c.repo, spec.FilePath))
	}

	body := fmt.Sprintf("## Resolution — [ForgeFix Resolution Report]\n\n**Status:** ✅ ALL TESTS PASSED\n\n")
	body += fmt.Sprintf("**Spec:** %s  \n", specRef)
	body += fmt.Sprintf("**Title:** %s  \n", spec.Title)
	body += fmt.Sprintf("**Issue:** %s  \n", closedRef)
	if spec.RootCause != "" {
		body += fmt.Sprintf("**Root Cause:** %s  \n", spec.RootCause)
	}
	if spec.Resolution != "" {
		body += fmt.Sprintf("**Resolution:** %s  \n", spec.Resolution)
	}
	body += "\n### Implementation\n\n"
	if spec.Body != "" {
		body += spec.Body + "\n\n"
	}
	body += "---\n**Closed by:** ForgeFix Auto-Resolution"
	return c.PostComment(issueNumber, body)
}

// specFileWebURL converts an API base URL and local spec file path into a
// web URL for the file on the remote (GitHub/Gitea). It handles both
// GitHub (api.github.com) and Gitea (/api/v1) URL patterns.
// Returns empty string if the file doesn't exist locally (e.g., archived).
func specFileWebURL(apiBase, owner, repo, filePath string) string {
	if apiBase == "" || filePath == "" {
		return ""
	}

	// If the spec file no longer exists locally (e.g., it was archived),
	// return empty so callers can fall back to just the spec ID text
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ""
	}

	apiBase = strings.TrimRight(apiBase, "/")
	webRoot := apiBase

	// Extract the filename from the local path and URL-encode it
	filename := filePath
	if idx := strings.LastIndexByte(filename, '/'); idx >= 0 {
		filename = filename[idx+1:]
	}
	filename = url.PathEscape(filename)

	if strings.Contains(webRoot, "api.github.com") {
		return fmt.Sprintf("https://github.com/%s/%s/blob/main/specs/%s", owner, repo, filename)
	}

	// Gitea: derive web root by stripping /api/* suffix
	if idx := strings.LastIndex(webRoot, "/api/"); idx >= 0 {
		webRoot = webRoot[:idx]
	}

	return fmt.Sprintf("%s/%s/%s/src/branch/main/specs/%s", webRoot, owner, repo, filename)
}

func (c *IssueCoordinator) PostComment(issueNumber int, body string) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	bodyMap := map[string]string{"body": body}
	jsonBody, _ := json.Marshal(bodyMap)
	postURL := c.url(fmt.Sprintf("/%d/comments", issueNumber))
	req, _ := http.NewRequest("POST", postURL, strings.NewReader(string(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) CloseIssueByNumber(issueNumber int) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, _ := http.NewRequest("PATCH", patchURL, strings.NewReader(`{"state":"closed"}`))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	fmt.Fprintf(os.Stderr, "[DEBUG] Repo/GitHub PATCH %s\n", patchURL)

	resp, err := c.client.Do(req)
	if err != nil {
		var netErr net.Error
		var urlErr *url.Error
		fmt.Fprintf(os.Stderr, "[DEBUG] PATCH request error: %v (type: %T)\n", err, err)
		if errors.As(err, &urlErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] URL Error: %v, Timeout: %v, Temporary: %v\n", urlErr.Err, urlErr.Timeout(), urlErr.Temporary())
		}
		if errors.As(err, &netErr) {
			fmt.Fprintf(os.Stderr, "[DEBUG] Net Error: Timeout=%v, Temporary=%v\n", netErr.Timeout(), netErr.Temporary())
		}
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(os.Stderr, "[DEBUG] PATCH response status: %d\n", resp.StatusCode)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("issue close failed on #%d: %w", issueNumber, ErrResourceNotFound)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "[DEBUG] PATCH response body: %s\n", string(respBody))
		return fmt.Errorf("issue close failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (c *IssueCoordinator) BatchCloseIssues(issueNumbers []int) []error {
	errs := make([]error, len(issueNumbers))
	if c.isInactive() {
		err := fmt.Errorf("coordinator inactive: placeholder or empty credentials")
		for i := range errs {
			errs[i] = err
		}
		return errs
	}

	type jobResult struct {
		index int
		err   error
	}

	results := make(chan jobResult, len(issueNumbers))

	for i, num := range issueNumbers {
		go func(idx, issueNum int) {
			err := c.CloseIssueByNumber(issueNum)
			results <- jobResult{idx, err}
		}(i, num)
	}

	for range issueNumbers {
		r := <-results
		errs[r.index] = r.err
	}
	close(results)
	return errs
}

func (c *IssueCoordinator) GetIssueComments(issueNumber int) ([]GitHubComment, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	req, _ := http.NewRequest("GET", c.url(fmt.Sprintf("/%d/comments", issueNumber)), nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) CloseIssue(testName string) error {
	c.mu.RLock()
	number, ok := c.tracked[testName]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no tracked issue for test: %s", testName)
	}
	return c.CloseIssueByNumber(number)
}

func (c *IssueCoordinator) ClearCache(testName string) {
	c.mu.Lock()
	delete(c.issueCache, testName)
	delete(c.cacheExpiry, testName)
	c.mu.Unlock()
}

func (c *IssueCoordinator) ClearAllCache() {
	c.mu.Lock()
	c.issueCache = make(map[string]*GitHubIssue)
	c.cacheExpiry = make(map[string]int)
	c.mu.Unlock()
}

func (c *IssueCoordinator) generateIssueBody(testName string, details *ErrorDetails) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Failing Test: `%s`\n\n", testName))
	if details.Package != "" {
		sb.WriteString(fmt.Sprintf("**Package:** `%s`\n\n", details.Package))
	}
	if details.FilePath != "" {
		sb.WriteString(fmt.Sprintf("**Location:** %s:%d\n\n", details.FilePath, details.LineNumber))
	}
	sb.WriteString("### Error Details\n\n```\n")
	sb.WriteString(details.ErrorMessage)
	sb.WriteString("\n```\n\n")
	sb.WriteString("### Suggested Fix\n\nInvestigate the failing test and update the code accordingly.\n")
	return sb.String()
}

func (c *IssueCoordinator) ParseErrorDetails(rawJSON string) (*ErrorDetails, error) {
	var details ErrorDetails
	if err := json.Unmarshal([]byte(rawJSON), &details); err != nil {
		return nil, err
	}
	return &details, nil
}

func (c *IssueCoordinator) GetIssueKey(testName string) string {
	return fmt.Sprintf("%s/%s", c.owner, testName)
}

func CheckForExistingIssue(coordinator *IssueCoordinator, testName string) (*GitHubIssue, error) {
	return coordinator.CheckExistingIssue(testName)
}

func (c *IssueCoordinator) FetchOpenIssues() ([]GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	params := url.Values{}
	params.Set("state", "open")
	params.Set("per_page", "100")
	queryURL := c.url("") + "?" + params.Encode()

	req, _ := http.NewRequest("GET", queryURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch issues failed with status: %d", resp.StatusCode)
	}

	var issues []GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func (c *IssueCoordinator) GetIssueByNumber(number int) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	getURL := c.url(fmt.Sprintf("/%d", number))
	req, _ := http.NewRequest("GET", getURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

func (c *IssueCoordinator) GetAllTracked() map[string]int {
	if c.isInactive() {
		return make(map[string]int)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]int, len(c.tracked))
	for k, v := range c.tracked {
		cp[k] = v
	}
	return cp
}

func (c *IssueCoordinator) UpdateIssueBody(issueNumber int, body string) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	bodyMap := map[string]string{"body": body}
	jsonBody, _ := json.Marshal(bodyMap)
	patchURL := c.url(fmt.Sprintf("/%d", issueNumber))
	req, _ := http.NewRequest("PATCH", patchURL, strings.NewReader(string(jsonBody)))
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
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

// resolveAuditDir walks up from configDir to find the forgefix project root
// (the directory containing forgefix_ff.yaml), so that .forgefix_history.log
// always lands in forgefix/ regardless of where ff is invoked from.
func resolveAuditDir(configDir string) string {
	dir := configDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "forgefix_ff.yaml")); err == nil {
			return dir
		}
		forgefixDir := filepath.Join(dir, "forgefix")
		if _, err := os.Stat(filepath.Join(forgefixDir, "forgefix_ff.yaml")); err == nil {
			return forgefixDir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return configDir
}

func (c *IssueCoordinator) LogAuditEntry(issueNumber int, testName, message string) {
	configDir := c.configDir
	if configDir == "" {
		var err error
		configDir, err = os.Getwd()
		if err != nil {
			return
		}
	}
	configDir = resolveAuditDir(configDir)
	commitHash := ""
	if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		commitHash = strings.TrimSpace(string(out))
	}
	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("[%s] [#%d] [%s] [%s] [%s]\n", timestamp, issueNumber, commitHash, testName, message)
	auditPath := FFHistoryLogPath(configDir)
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] failed to open audit log %s: %v\n", auditPath, err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(entry); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] failed to write audit log %s: %v\n", auditPath, err)
	}
}

func ReadAuditLogEntries(configDir string) []AuditEntry {
	configDir = resolveAuditDir(configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []AuditEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}
		// [timestamp] [#number] [commit] [testName] [message]
		trimmed := strings.TrimPrefix(line, "[")
		parts := strings.Split(trimmed, "] [")
		if len(parts) < 5 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			continue
		}
		numberStr := strings.TrimPrefix(parts[1], "#")
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			continue
		}
		message := strings.TrimSuffix(parts[4], "]")
		entries = append(entries, AuditEntry{
			Timestamp:   ts,
			IssueNumber: number,
			CommitHash:  parts[2],
			TestName:    parts[3],
			Message:     message,
		})
	}
	return entries
}

func ReadAuditLog(configDir string) map[string]int {
	configDir = resolveAuditDir(configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// Format: [timestamp] [#number] [commit] [testName] [message]
	result := make(map[string]int)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "] [")
		if len(parts) < 5 {
			continue
		}
		numberStr := strings.TrimPrefix(parts[1], "#")
		number, err := strconv.Atoi(numberStr)
		if err != nil {
			continue
		}
		testName := parts[3]
		message := strings.TrimSuffix(parts[4], "]")
		if strings.HasPrefix(message, "CLOSED") {
			delete(result, testName)
		} else {
			result[testName] = number
		}
	}
	return result
}

func DeleteAuditEntry(configDir string, testName string) {
	configDir = resolveAuditDir(configDir)
	path := FFHistoryLogPath(configDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "["+testName+"]") {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	os.WriteFile(path, []byte(out), 0644)
}

func (c *IssueCoordinator) EnsureIssue(testName string, details *ErrorDetails) (*GitHubIssue, bool, error) {
	if !c.IsActive() {
		return nil, false, fmt.Errorf("coordinator inactive: empty credentials")
	}
	existing, err := c.CheckExistingIssue(testName)
	if err == nil && existing != nil {
		if c.failureDecaySet && c.configDir != "" {
			entries := ReadAuditLogEntries(c.configDir)
			for _, entry := range entries {
				if entry.TestName == testName && strings.HasPrefix(entry.Message, "CREATED") {
					if time.Since(entry.Timestamp) >= c.failureDecayDuration {
						fmt.Fprintf(os.Stderr, "[DECAY] failure decayed for %s (issue #%d, age %v >= %v), creating fresh issue\n",
							testName, existing.Number, time.Since(entry.Timestamp), c.failureDecayDuration)
						_ = c.CloseIssueByNumber(existing.Number)
						c.ClearCache(testName)
						DeleteAuditEntry(c.configDir, testName)
						created, err := c.CreateIssue(testName, details)
						if err != nil {
							return nil, false, err
						}
						c.LogAuditEntry(created.Number, testName, "CREATED - test failed (decayed from #"+strconv.Itoa(existing.Number)+")")
						return created, false, nil
					}
					break
				}
			}
		}
		return existing, true, nil
	}

	created, err := c.CreateIssue(testName, details)
	if err != nil {
		return nil, false, err
	}
	c.LogAuditEntry(created.Number, testName, "CREATED - test failed")
	return created, false, nil
}

func (c *IssueCoordinator) SyncIssues(configDir string) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	tracked := ReadAuditLog(configDir)
	if len(tracked) == 0 {
		fmt.Println("Audit log is empty — fetching remote issues to re-populate.")
		return c.syncIssuesPhase2(configDir)
	}

	if err := c.syncIssuesPhase1Concurrent(configDir, tracked); err != nil {
		fmt.Fprintf(os.Stderr, "warning: phase 1 concurrent sync had errors: %v\n", err)
	}

	return c.syncIssuesPhase2(configDir)
}

const syncIssuesConcurrency = 5

func (c *IssueCoordinator) syncIssuesPhase1Concurrent(configDir string, tracked map[string]int) error {
	type result struct {
		testName    string
		issueNumber int
		err         error
		state       string
	}

	sem := make(chan struct{}, syncIssuesConcurrency)
	results := make(chan result, len(tracked))

	for testName, issueNumber := range tracked {
		sem <- struct{}{}
		go func(tn string, in int) {
			defer func() { <-sem }()
			issue, err := c.GetIssueByNumber(in)
			if err != nil {
				results <- result{testName: tn, issueNumber: in, err: err}
				return
			}
			results <- result{testName: tn, issueNumber: in, state: issue.State}
		}(testName, issueNumber)
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(results)

	var firstErr error
	for res := range results {
		if res.err != nil {
			if errors.Is(res.err, ErrResourceNotFound) {
				fmt.Printf("Issue #%d not found in Repo (ghost), pruning audit entry for %s\n", res.issueNumber, res.testName)
				DeleteAuditEntry(configDir, res.testName)
			} else {
				fmt.Fprintf(os.Stderr, "error fetching issue #%d: %v\n", res.issueNumber, res.err)
				if firstErr == nil {
					firstErr = res.err
				}
			}
			continue
		}
		if res.state != "open" {
			fmt.Printf("Issue #%d (%s) is closed in Repo — possible DUPLICATE if failure recurs\n", res.issueNumber, res.testName)
			DeleteAuditEntry(configDir, res.testName)
		} else {
			fmt.Printf("Issue #%d (%s) is open and healthy\n", res.issueNumber, res.testName)
		}
	}
	return firstErr
}

func (c *IssueCoordinator) syncIssuesPhase2(configDir string) error {

	// Phase 2: Fetch all open remote issues and add local audit entries for any that are missing
	afterTracked := ReadAuditLog(configDir)
	remoteIssues, err := c.FetchOpenIssues()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching remote issues: %v\n", err)
		return nil
	}
	for _, remote := range remoteIssues {
		testName := remote.Title
		if _, exists := afterTracked[testName]; exists {
			continue
		}
		if testName == "" {
			continue
		}
		if strings.HasPrefix(testName, "🛑") || strings.HasPrefix(testName, "⏹") {
			fmt.Printf("Skipping ephemeral system alert: %s\n", testName)
			continue
		}
		c.LogAuditEntry(remote.Number, testName, "REPOPULATED - sync found remote issue #"+strconv.Itoa(remote.Number))
		fmt.Printf("Repopulated local audit entry for remote issue #%d (%s)\n", remote.Number, testName)
	}

	// Phase 3: Baseline drift detection — compare ledger floor vs total passed
	ledger, err := LoadLedger(configDir)
	if err == nil && ledger != nil {
		totalPassed := ledger.GetTotalPassed()
		totalFloor := ledger.GetTotalFloor()
		if totalFloor > 0 && totalPassed < totalFloor {
			fmt.Fprintf(os.Stderr, "[BASELINE DRIFT] total passed (%d) is below baseline floor (%d) — %d test(s) may be missing or failing\n",
				totalPassed, totalFloor, totalFloor-totalPassed)
		} else if totalFloor > 0 {
			fmt.Printf("Baseline healthy: %d passed >= %d floor\n", totalPassed, totalFloor)
		}
	}

	return nil
}

type SpecFile struct {
	SpecID      string
	Title       string
	Body        string
	RepoIssue  int
	Status      string
	FilePath    string
	Type        string
	Version     string
	RootCause   string
	Resolution  string
}

func parseSpecFile(filePath string) (*SpecFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("invalid spec file: missing frontmatter")
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid spec file: malformed frontmatter")
	}

	frontmatter := parts[1]
	body := strings.TrimSpace(parts[2])

	spec := &SpecFile{
		FilePath: filePath,
		Body:     body,
	}

	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "spec_id:") {
			spec.SpecID = strings.TrimSpace(strings.TrimPrefix(line, "spec_id:"))
			spec.SpecID = strings.Trim(spec.SpecID, `"`)
		} else if strings.HasPrefix(line, "status:") {
			spec.Status = strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			spec.Status = strings.Trim(spec.Status, `"`)
			if idx := strings.Index(spec.Status, " "); idx > 0 {
				spec.Status = spec.Status[:idx]
			}
		} else if strings.HasPrefix(line, "type:") {
			spec.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
			spec.Type = strings.Trim(spec.Type, `"`)
		} else if strings.HasPrefix(line, "version:") {
			spec.Version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
			spec.Version = strings.Trim(spec.Version, `"`)
		} else if strings.HasPrefix(line, "repo_issue:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "repo_issue:"))
			if val != "" && val != `""` {
				fmt.Sscanf(val, "%d", &spec.RepoIssue)
			}
		} else if strings.HasPrefix(line, "root_cause:") {
			spec.RootCause = strings.TrimSpace(strings.TrimPrefix(line, "root_cause:"))
			spec.RootCause = strings.Trim(spec.RootCause, `"`)
		} else if strings.HasPrefix(line, "resolution:") {
			spec.Resolution = strings.TrimSpace(strings.TrimPrefix(line, "resolution:"))
			spec.Resolution = strings.Trim(spec.Resolution, `"`)
		}
	}

	if strings.HasPrefix(body, "# ") {
		titleLine := strings.SplitN(body, "\n", 2)[0]
		spec.Title = strings.TrimPrefix(titleLine, "# ")
	}

	return spec, nil
}

func (c *IssueCoordinator) findRemoteIssueByTitle(title string) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	issues, err := c.ListOpenIssues()
	if err != nil {
		return nil, err
	}

	normalized := NormalizeTitle(title)
	if normalized == "" {
		return nil, fmt.Errorf("empty title after normalization")
	}

	for _, issue := range issues {
		if NormalizeTitle(issue.Title) == normalized {
			fmt.Fprintf(os.Stderr, "[SYNC] Found existing remote issue #%d matching title: %s\n", issue.Number, title)
			return &issue, nil
		}
	}

	return nil, fmt.Errorf("no existing remote issue found for title: %s", title)
}

func (c *IssueCoordinator) SyncSpecs(configDir string) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	specDir := filepath.Join(configDir, "specs")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No specs directory found, skipping spec sync.")
			return nil
		}
		return err
	}

	ledger := c.GetLedger()
	if ledger == nil {
		ledger, _ = LoadLedger(configDir)
	}

	// Single fetch: ensure all configured category labels exist on the remote
	if ledger != nil {
		c.ensureCategoryLabelsExist(ledger.WorkflowConfig)
	}

	// Reconciliation: Cross-reference remote issues with local specs
	c.performReconciliation(configDir, ledger, entries)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		// Skip archive files — aggregated spec dumps, not individual specs
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}

		filePath := filepath.Join(specDir, entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to parse spec %s: %v\n", entry.Name(), err)
			continue
		}

		fmt.Printf("Syncing spec: %s (ID: %s)\n", spec.Title, spec.SpecID)

		// Phase 1: Determine the remote issue number for this spec
		if spec.RepoIssue == 0 {
			if existing, err := c.findRemoteIssueByTitle(spec.Title); err == nil {
				spec.RepoIssue = existing.Number
				fmt.Printf("Found existing remote issue #%d for spec: %s (skipping POST)\n", existing.Number, spec.Title)
			} else {
				c.markSpecAsDuplicateIfNeeded(spec)
				title := spec.Title
				if !IsValidIssueTitle(title) {
					title = fmt.Sprintf("feat/spec: %s", title)
				}
				if len(title) > maxTitleLength {
					cutAt := maxTitleLength
					for cutAt > 10 && title[cutAt-1] != ' ' {
						cutAt--
					}
					if cutAt <= 10 {
						cutAt = maxTitleLength
					}
					title = strings.TrimSpace(title[:cutAt])
				}
				issue, err := c.CreateIssueWithBody(title, spec.Body)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error creating issue for spec %s: %v\n", spec.Title, err)
					continue
				}
				spec.RepoIssue = issue.Number
				fmt.Printf("Created issue #%d for spec: %s\n", issue.Number, spec.Title)
			}
		}

		// Phase 2: Sync body and state with the remote issue (if we have a number)
		if spec.RepoIssue > 0 {
			remoteIssue, err := c.GetIssueByNumber(spec.RepoIssue)
			if err != nil {
				if errors.Is(err, ErrResourceNotFound) {
					fmt.Printf("Issue #%d for spec %s not found on remote (404) — unbinding\n", spec.RepoIssue, spec.Title)

					spec.RepoIssue = 0
					if err := updateSpecFileRepoIssue(filePath, 0); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to clear repo_issue in spec file %s: %v\n", entry.Name(), err)
					}
					if ledger != nil && spec.SpecID != "" {
						if existing := ledger.GetSpecEntry(spec.SpecID); existing != nil {
							existing.RepoIssueID = 0
							ledger.SetSpecEntry(spec.SpecID, existing)
							if err := SaveLedger(ledger, configDir); err != nil {
								fmt.Fprintf(os.Stderr, "warning: failed to save ledger after unbinding: %v\n", err)
							}
						}
					}

					// Try to find existing by title or create
					if existing, err := c.findRemoteIssueByTitle(spec.Title); err == nil {
						spec.RepoIssue = existing.Number
						fmt.Printf("Found existing remote issue #%d for spec: %s (rebinding)\n", existing.Number, spec.Title)
					} else {
						c.markSpecAsDuplicateIfNeeded(spec)
						title := spec.Title
						if !IsValidIssueTitle(title) {
							title = fmt.Sprintf("feat/spec: %s", title)
						}
						if len(title) > maxTitleLength {
							cutAt := maxTitleLength
							for cutAt > 10 && title[cutAt-1] != ' ' {
								cutAt--
							}
							if cutAt <= 10 {
								cutAt = maxTitleLength
							}
							title = strings.TrimSpace(title[:cutAt])
						}
						issue, err := c.CreateIssueWithBody(title, spec.Body)
						if err != nil {
							fmt.Fprintf(os.Stderr, "error creating replacement issue for spec %s: %v\n", spec.Title, err)
							continue
						}
						spec.RepoIssue = issue.Number
						fmt.Printf("Created replacement issue #%d for spec: %s\n", issue.Number, spec.Title)
					}

					// Fetch the (re)bound or newly created issue for body sync
					if spec.RepoIssue > 0 {
						remoteIssue, err = c.GetIssueByNumber(spec.RepoIssue)
						if err != nil {
							fmt.Fprintf(os.Stderr, "error fetching issue #%d after rebind: %v\n", spec.RepoIssue, err)
							continue
						}
					}
				} else {
					fmt.Fprintf(os.Stderr, "error fetching issue #%d: %v\n", spec.RepoIssue, err)
					continue
				}
			}

			if remoteIssue != nil {
				if remoteIssue.Body != spec.Body {
					err := c.UpdateIssueBody(spec.RepoIssue, spec.Body)
					if err != nil {
						fmt.Fprintf(os.Stderr, "error updating issue #%d: %v\n", spec.RepoIssue, err)
						continue
					}
					fmt.Printf("Updated issue #%d for spec: %s\n", spec.RepoIssue, spec.Title)
				} else {
					fmt.Printf("Issue #%d for spec %s is up to date\n", spec.RepoIssue, spec.Title)
				}

				if isResolvedStatus(spec.Status) && remoteIssue.State == "open" {
					if existing, err := c.FindExistingIssueBySimilarTitle(spec.Title); err == nil && existing.Number != spec.RepoIssue {
						dupTitle := spec.Title + fmt.Sprintf(" [Dupe → #%d]", existing.Number)
						if err := c.UpdateIssueTitle(spec.RepoIssue, dupTitle); err != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to update dupe title for #%d: %v\n", spec.RepoIssue, err)
						} else {
							fmt.Printf("Updated issue #%d title to mark as duplicate of #%d\n", spec.RepoIssue, existing.Number)
						}
					}

				payload := housekeeper.ResolutionPayload{
					SpecID:    spec.SpecID,
					Title:     spec.Title,
					Version:   spec.Version,
					RepoIssue: spec.RepoIssue,
					SpecURL:   specFileWebURL(c.baseURL, c.owner, c.repo, filePath),
				}
				payloadRaw, _ := json.Marshal(payload)
				hq := housekeeper.NewHousekeepingQueue(configDir)
					if err := hq.Load(); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to load housekeeping queue: %v\n", err)
					} else {
						if err := hq.Enqueue(housekeeper.HousekeepingTask{
							Type:        housekeeper.TaskTypePostResolution,
							SpecID:      spec.SpecID,
							RepoIssueID: spec.RepoIssue,
							Priority:    housekeeper.PriorityHigh,
							Payload:     string(payloadRaw),
						}); err != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to enqueue resolution task: %v\n", err)
						}
						if err := hq.Enqueue(housekeeper.HousekeepingTask{
							Type:        housekeeper.TaskTypeCloseIssue,
							SpecID:      spec.SpecID,
							RepoIssueID: spec.RepoIssue,
							Priority:    housekeeper.PriorityHigh,
						}); err != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to enqueue close task: %v\n", err)
						}
						fmt.Printf("Enqueued resolution and close tasks for issue #%d\n", spec.RepoIssue)
					}
				}

				if remoteIssue.State != "open" && spec.Status != "closed" {
					fmt.Printf("Status change detected: issue #%d is now %s, updating spec status to closed\n", spec.RepoIssue, remoteIssue.State)
					spec.Status = "closed"
					if err := updateSpecFileStatus(filePath, "closed"); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to update spec file status: %v\n", err)
					}
				}
			}
		}

		if err := updateSpecFileRepoIssue(filePath, spec.RepoIssue); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update spec file %s: %v\n", entry.Name(), err)
		}

		if ledger != nil && spec.SpecID != "" {
			ledger.SetSpecEntry(spec.SpecID, &SpecEntry{
				SpecID:       spec.SpecID,
				RepoIssueID:  spec.RepoIssue,
				Status:       spec.Status,
				LinkedCommits: []string{},
			})
			if err := SaveLedger(ledger, configDir); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", err)
			}
		}

		c.syncSpecLabels(spec, ledger.WorkflowConfig)
	}

	return nil
}

func (c *IssueCoordinator) performReconciliation(configDir string, ledger *LedgerEngine, entries []os.DirEntry) {
	if c.isInactive() {
		return
	}

	// Fetch all open remote issues
	base := c.baseURL + "/repos/" + c.owner + "/" + c.repo
	params := url.Values{}
	params.Set("state", "open")
	params.Set("per_page", "100")
	queryURL := base + "/issues?" + params.Encode()

	req, _ := http.NewRequest("GET", queryURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: reconciliation fetch failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "warning: reconciliation fetch failed with status %d\n", resp.StatusCode)
		return
	}

	var remoteIssues []GitHubIssue
	if err := json.NewDecoder(resp.Body).Decode(&remoteIssues); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to decode remote issues: %v\n", err)
		return
	}

	// Build map of local specs by repo_issue
	localByIssue := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(configDir, "specs", entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil {
			continue
		}
		if spec.RepoIssue > 0 {
			localByIssue[spec.RepoIssue] = spec.SpecID
		}
	}

	// Identify orphans: remote issues without local spec
	var orphans []GitHubIssue
	for _, issue := range remoteIssues {
		if _, exists := localByIssue[issue.Number]; !exists {
			orphans = append(orphans, issue)
		}
	}

	// Identify ghosts: local specs with repo_issue that doesn't exist on remote
	var ghosts []string
	remoteIssueMap := make(map[int]bool)
	for _, issue := range remoteIssues {
		remoteIssueMap[issue.Number] = true
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(configDir, "specs", entry.Name())
		spec, err := parseSpecFile(filePath)
		if err != nil || spec.RepoIssue <= 0 {
			continue
		}
		if !remoteIssueMap[spec.RepoIssue] {
			ghosts = append(ghosts, fmt.Sprintf("%s (repo_issue: %d)", spec.SpecID, spec.RepoIssue))
		}
	}

	// Report reconciliation results
	if len(orphans) > 0 || len(ghosts) > 0 {
		fmt.Printf("=== Reconciliation Report ===\n")
		if len(orphans) > 0 {
			fmt.Printf("Orphaned remote issues (no local spec): %d\n", len(orphans))
			for _, o := range orphans {
				fmt.Printf("  #%d: %s\n", o.Number, o.Title)
			}
		}
		if len(ghosts) > 0 {
			fmt.Printf("Ghost local specs (remote issue missing): %d\n", len(ghosts))
			for _, g := range ghosts {
				fmt.Printf("  %s\n", g)
			}
		}
		fmt.Printf("=============================\n")
	}
}

func updateSpecFileRepoIssue(filePath string, issueNumber int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "repo_issue:") {
			lines[i] = fmt.Sprintf("repo_issue: %d", issueNumber)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

func updateSpecFileStatus(filePath string, status string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "status:") {
			lines[i] = fmt.Sprintf("status: %s", status)
			break
		}
	}

	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}
