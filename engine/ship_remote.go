package engine

import (
	"fmt"
	"net/url"
	"strings"
)

// publicHosts are well-known public git hosting SaaS endpoints. Pushing to a
// remote on one of these hosts is treated as a public ship and requires
// explicit confirmation (default: refused). Private/self-hosted instances
// (including a local NAS Gitea) are never in this set, so they ship silently.
var publicHosts = map[string]bool{
	"github.com":    true,
	"gitlab.com":    true,
	"bitbucket.org": true,
	"gitee.com":     true,
}

// shipRemoteDecision is the outcome of evaluating where `ff ship` should push.
type shipRemoteDecision struct {
	Remote string // resolved remote name to push to
	URL    string // resolved remote URL
	Public bool   // true if the remote host is a public SaaS host
}

// hostFromGitURL extracts the hostname from a git remote URL. It supports
// https://host/path, ssh://git@host:port/path, and scp-like git@host:path.
func hostFromGitURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// scp-like: git@host:path
	if strings.HasPrefix(s, "git@") {
		rest := strings.TrimPrefix(s, "git@")
		if i := strings.Index(rest, ":"); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	// ssh://git@host:port/path
	if strings.HasPrefix(s, "ssh://") {
		s = strings.TrimPrefix(s, "ssh://")
		s = strings.TrimPrefix(s, "git@")
		if i := strings.Index(s, ":"); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[:i]
		}
		return s
	}
	// https://host/path or http://host/path
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		return u.Host
	}
	return ""
}

// isPublicHost reports whether the given git remote URL points at a well-known
// public hosting SaaS. Unknown/private hosts are treated as private.
func isPublicHost(rawURL string) bool {
	host := hostFromGitURL(rawURL)
	if host == "" {
		return false
	}
	// strip any port and userinfo
	if i := strings.Index(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	return publicHosts[strings.ToLower(host)]
}

// resolveShipRemote decides which git remote `ff ship` should push to.
// getRemotes returns the map of remote name -> URL (in production it shells out
// to git; it is injectable so the resolution logic is unit-testable).
//
// Resolution order:
//  1. config.ShipRemote if set (explicit override; must exist among remotes).
//  2. a remote whose host matches config.GitHub.BaseURL (the NAS/Gitea tracker).
//  3. "origin" as a last resort.
func resolveShipRemote(config *Config, getRemotes func() (map[string]string, error)) (shipRemoteDecision, error) {
	remotes, err := getRemotes()
	if err != nil {
		return shipRemoteDecision{}, fmt.Errorf("could not list git remotes: %w", err)
	}
	if len(remotes) == 0 {
		return shipRemoteDecision{}, fmt.Errorf("no git remotes configured")
	}

	remote := "origin"
	if config != nil && config.ShipRemote != "" {
		remote = config.ShipRemote
		if _, ok := remotes[remote]; !ok {
			return shipRemoteDecision{}, fmt.Errorf("configured ship_remote %q not found among git remotes: %s", remote, remoteKeys(remotes))
		}
	} else if config != nil && config.GitHub != nil && config.GitHub.BaseURL != "" {
		baseHost := normalizeHost(hostFromGitURL(config.GitHub.BaseURL))
		if baseHost != "" {
			for name, u := range remotes {
				if normalizeHost(hostFromGitURL(u)) == baseHost {
					remote = name
					break
				}
			}
		}
	}

	u, ok := remotes[remote]
	if !ok {
		return shipRemoteDecision{}, fmt.Errorf("ship remote %q not found among git remotes: %s", remote, remoteKeys(remotes))
	}
	return shipRemoteDecision{Remote: remote, URL: strings.TrimSpace(u), Public: isPublicHost(u)}, nil
}

// normalizeHost strips any port and userinfo and lowercases a hostname so
// that e.g. "192.168.1.18:2222" and "192.168.1.18" compare equal.
func normalizeHost(host string) string {
	h := host
	if i := strings.Index(h, "@"); i >= 0 {
		h = h[i+1:]
	}
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return strings.ToLower(h)
}
func remoteKeys(remotes map[string]string) string {
	keys := make([]string, 0, len(remotes))
	for k := range remotes {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

// gitRemoteMapFunc returns a getRemotes implementation backed by the git CLI
// for the given project directory.
func gitRemoteMapFunc(configDir string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		out, err := execGit(configDir, "remote")
		if err != nil {
			return nil, err
		}
		m := map[string]string{}
		for _, name := range strings.Fields(out) {
			u, err := execGit(configDir, "remote", "get-url", name)
			if err != nil {
				continue
			}
			m[name] = strings.TrimSpace(u)
		}
		return m, nil
	}
}
