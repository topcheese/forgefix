package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ForgeFix/engine/housekeeper"
)

type ErrorDetails struct {
	TestName     string `json:"test_name"`
	Package      string `json:"package"`
	FilePath     string `json:"file_path"`
	LineNumber   int    `json:"line_number"`
	ErrorMessage string `json:"error_message"`
	StackTrace   string `json:"stack_trace"`
}

type IssueCoordinator struct {
	mu                   sync.RWMutex
	owner                string
	repo                 string
	baseURL              string
	apiToken             string
	issueCache           map[string]*GitHubIssue
	cacheExpiry          map[string]int
	gh                   GitHubClient
	sm                   SpecManager
	tracked              map[string]int
	inactive             bool
	configDir            string
	failureDecayDuration time.Duration
	failureDecaySet      bool
	ledger               *LedgerEngine
	titleValidator       *IssueTitleValidator
	auditLog             *AuditLog
}

func NewIssueCoordinator(owner, repo, token, baseURL string) *IssueCoordinator {
	gh := NewGitHubClient(owner, repo, token, baseURL)

	coordinator := &IssueCoordinator{
		owner:          owner,
		repo:           repo,
		baseURL:        baseURL,
		apiToken:       token,
		issueCache:     make(map[string]*GitHubIssue),
		cacheExpiry:    make(map[string]int),
		gh:             gh,
		sm:             NewSpecManager(),
		tracked:        make(map[string]int),
		titleValidator: NewIssueTitleValidator(),
		auditLog:       NewAuditLog(""),
	}
	coordinator.inactive = coordinator.isPlaceholderConfig()
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

// Client returns the underlying GitHubClient for direct access to the HTTP port.
func (c *IssueCoordinator) Client() GitHubClient {
	return c.gh
}

func (c *IssueCoordinator) SetConfigDir(dir string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configDir = dir
	c.auditLog = NewAuditLog(dir)
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

func (c *IssueCoordinator) GetIssueLabels(issueNumber int) ([]RepoLabel, error) {
	return c.gh.GetIssueLabels(issueNumber)
}

func (c *IssueCoordinator) SetIssueLabels(issueNumber int, labelNames []string) error {
	return c.gh.SetIssueLabels(issueNumber, labelNames)
}

var colorNameToHex = map[string]string{
	"black":     "000000",
	"white":     "ffffff",
	"red":       "e74c3c",
	"green":     "2ecc71",
	"yellow":    "f1c40f",
	"blue":      "3498db",
	"magenta":   "9b59b6",
	"cyan":      "1abc9c",
	"orange":    "e67e22",
	"purple":    "8e44ad",
	"grey":      "95a5a6",
	"gray":      "95a5a6",
	"hiblack":   "555555",
	"hiwhite":   "cccccc",
	"hired":     "ff6b6b",
	"higreen":   "69db7c",
	"hiyellow":  "ffd93d",
	"hiblue":    "74b9ff",
	"himagenta": "dda0dd",
	"hicyan":    "81ecec",
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

	repoLabels, err := c.gh.GetRepoLabels()
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
			if err := c.gh.CreateRepoLabel(defaultLabel, color); err != nil {
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
	issueLabels, err := c.gh.GetIssueLabels(spec.RepoIssue)
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

	if err := c.gh.SetIssueLabels(spec.RepoIssue, newLabels); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to set issue labels for #%d: %v\n", spec.RepoIssue, err)
		return
	}
	fmt.Fprintf(os.Stderr, "Updated issue #%d labels to %v for spec %s\n", spec.RepoIssue, desiredLabels, spec.SpecID)
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

	issues, err := c.gh.ListOpenIssues()
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
	issues, err := c.gh.ListOpenIssues()
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
	return c.gh.UpdateIssueTitle(issueNumber, title)
}

func (c *IssueCoordinator) CreateIssueWithBody(title, body string) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	if err := c.titleValidator.Validate(title); err != nil {
		return nil, fmt.Errorf("issue title validation failed: %w", err)
	}
	createdIssue, err := c.gh.CreateIssue(title, body)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.issueCache[title] = createdIssue
	c.tracked[title] = createdIssue.Number
	c.mu.Unlock()
	return createdIssue, nil
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
	jsonBody, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("marshaling issue body: %w", err)
	}

	createdIssue, err := c.gh.CreateIssue(testName, string(jsonBody))
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.issueCache[testName] = createdIssue
	c.tracked[testName] = createdIssue.Number
	c.mu.Unlock()

	return createdIssue, nil
}

func (c *IssueCoordinator) PostResolutionComment(issueNumber int, spec *SpecFile) error {
	closedRef := fmt.Sprintf("#%d", issueNumber)
	specRef := spec.SpecID
	if spec.FilePath != "" {
		specRef = fmt.Sprintf("[%s](%s)", spec.SpecID, c.sm.SpecWebURL(c.baseURL, c.owner, c.repo, spec.FilePath))
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
	return c.gh.PostComment(issueNumber, body)
}

func (c *IssueCoordinator) PostComment(issueNumber int, body string) error {
	return c.gh.PostComment(issueNumber, body)
}

func (c *IssueCoordinator) CloseIssueByNumber(issueNumber int) error {
	return c.gh.CloseIssueByNumber(issueNumber)
}

// CreateRelease creates a release on the remote (Gitea/GitHub) for the
// given version tag with the supplied body. It is non-fatal: callers should
// log a warning and continue if it returns an error.
func (c *IssueCoordinator) CreateRelease(version, body string) (int, error) {
	if c.isInactive() {
		return 0, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	return c.gh.CreateRelease(version, body)
}

// UploadReleaseAsset uploads a binary file as an asset to a release.
func (c *IssueCoordinator) UploadReleaseAsset(releaseID int, name string, data []byte) error {
	if c.isInactive() {
		return fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	return c.gh.UploadReleaseAsset(releaseID, name, data)
}

// LatestRelease returns the most recent release from the remote.
func (c *IssueCoordinator) LatestRelease() (*Release, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	return c.gh.LatestRelease()
}

// DownloadReleaseAsset downloads a release asset by its ID.
func (c *IssueCoordinator) DownloadReleaseAsset(assetID int) ([]byte, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}
	return c.gh.DownloadReleaseAsset(assetID)
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
			err := c.gh.CloseIssueByNumber(issueNum)
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
	return c.gh.GetIssueComments(issueNumber)
}

func (c *IssueCoordinator) CloseIssue(testName string) error {
	c.mu.RLock()
	number, ok := c.tracked[testName]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no tracked issue for test: %s", testName)
	}
	return c.gh.CloseIssueByNumber(number)
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

func (c *IssueCoordinator) GetIssueByNumber(number int) (*GitHubIssue, error) {
	return c.gh.GetIssueByNumber(number)
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
	return c.gh.UpdateIssueBody(issueNumber, body)
}

func (c *IssueCoordinator) LogAuditEntry(issueNumber int, testName, message string) {
	c.auditLog.AppendEntry(issueNumber, testName, message)
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
	remoteIssues, err := c.gh.ListOpenIssues()
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

func (c *IssueCoordinator) findRemoteIssueByTitle(title string) (*GitHubIssue, error) {
	if c.isInactive() {
		return nil, fmt.Errorf("coordinator inactive: placeholder or empty credentials")
	}

	issues, err := c.gh.ListOpenIssues()
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

	var syncedCount int

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		// Skip archive files — aggregated spec dumps, not individual specs
		if strings.HasPrefix(entry.Name(), "archive_") {
			continue
		}

		filePath := filepath.Join(specDir, entry.Name())
		spec, err := c.sm.ParseSpecFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to parse spec %s: %v\n", entry.Name(), err)
			continue
		}
		syncedCount++

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
				issue, err := c.CreateIssueWithBody(title, effectiveSpecBody(spec))
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
					if err := c.sm.UpdateRepoIssue(filePath, 0); err != nil {
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
						issue, err := c.CreateIssueWithBody(title, effectiveSpecBody(spec))
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
				postedBody := effectiveSpecBody(spec)
				if remoteIssue.Body != postedBody {
					err := c.UpdateIssueBody(spec.RepoIssue, postedBody)
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
						SpecURL:   c.sm.SpecWebURL(c.baseURL, c.owner, c.repo, filePath),
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

			if remoteIssue.State == "closed" && spec.Status != "closed" && spec.Status != "ship" {
				fmt.Printf("Status change detected: issue #%d is now closed, updating spec status to closed\n", spec.RepoIssue)
				spec.Status = "closed"
				if err := c.sm.UpdateStatus(filePath, "closed"); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to update spec file status: %v\n", err)
				}
			}
			}
		}

		if err := c.sm.UpdateRepoIssue(filePath, spec.RepoIssue); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update spec file %s: %v\n", entry.Name(), err)
		}

		if ledger != nil && spec.SpecID != "" {
			ledger.SetSpecEntry(spec.SpecID, &SpecEntry{
				SpecID:        spec.Title,
				RepoIssueID:   spec.RepoIssue,
				Status:        spec.Status,
				LinkedCommits: []string{},
			})
			if err := SaveLedger(ledger, configDir); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", err)
			}
		}

		c.syncSpecLabels(spec, ledger.WorkflowConfig)
	}

	// Reconciliation: after all specs are synced and repo_issue fields updated,
	// close any remote issues that still have no matching local spec.
	entries, err = os.ReadDir(specDir)
	if err == nil {
		c.performReconciliation(configDir, ledger, entries)
	}

	fmt.Printf("Spec sync completed. %d spec(s) synced.\n", syncedCount)
	return nil
}

func (c *IssueCoordinator) performReconciliation(configDir string, ledger *LedgerEngine, entries []os.DirEntry) {
	if c.isInactive() {
		return
	}

	// Fetch all open remote issues
	remoteIssues, err := c.gh.ListOpenIssues()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: reconciliation fetch failed: %v\n", err)
		return
	}

	// Build map of local specs by repo_issue
	localByIssue := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(configDir, "specs", entry.Name())
		spec, err := c.sm.ParseSpecFile(filePath)
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
		spec, err := c.sm.ParseSpecFile(filePath)
		if err != nil || spec.RepoIssue <= 0 {
			continue
		}
		if !remoteIssueMap[spec.RepoIssue] {
			ghosts = append(ghosts, fmt.Sprintf("%s (repo_issue: %d)", spec.SpecID, spec.RepoIssue))
		}
	}

	// Close orphaned remote issues that have no local spec
	closedCount := 0
	for _, o := range orphans {
		comment := "## Resolution — [ForgeFix Reconciliation]\n\n" +
			"**Status:** ✅ AUTO-CLOSED\n\n" +
			"This issue was automatically closed because its associated local spec has been archived or removed.\n\n" +
			"---\n**Closed by:** ForgeFix Reconciliation"
		if err := c.gh.PostComment(o.Number, comment); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to post closure comment on issue #%d: %v\n", o.Number, err)
		}
		if err := c.gh.CloseIssueByNumber(o.Number); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close orphaned issue #%d: %v\n", o.Number, err)
		} else {
			closedCount++
		}
	}

	// Report reconciliation results
	if len(orphans) > 0 || len(ghosts) > 0 || closedCount > 0 {
		fmt.Printf("=== Reconciliation Report ===\n")
		if len(orphans) > 0 {
			fmt.Printf("Orphaned remote issues (no local spec): %d\n", len(orphans))
			for _, o := range orphans {
				fmt.Printf("  #%d: %s\n", o.Number, o.Title)
			}
			if closedCount > 0 {
				fmt.Printf("  => Closed %d orphaned issue(s)\n", closedCount)
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
