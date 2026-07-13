package engine

import (
	"testing"
)

func TestHostFromGitURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/topcheese/forgefix.git", "github.com"},
		{"http://gitlab.com/x/y.git", "gitlab.com"},
		{"ssh://git@192.168.1.18:2222/Jimmy/forgefix.git", "192.168.1.18"},
		{"git@192.168.1.18:Jimmy/forgefix.git", "192.168.1.18"},
		{"git@github.com:topcheese/forgefix.git", "github.com"},
		{"ssh://git@example.com:2222/foo/bar.git", "example.com"},
		{"", ""},
	}
	for _, c := range cases {
		if got := hostFromGitURL(c.in); got != c.want {
			t.Errorf("hostFromGitURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsPublicHost(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://github.com/topcheese/forgefix.git", true},
		{"git@github.com:topcheese/forgefix.git", true},
		{"https://gitlab.com/x/y.git", true},
		{"https://bitbucket.org/x/y.git", true},
		{"ssh://git@192.168.1.18:2222/Jimmy/forgefix.git", false},
		{"git@192.168.1.18:Jimmy/forgefix.git", false},
		{"https://gitea.nas.local/Jimmy/forgefix.git", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isPublicHost(c.in); got != c.want {
			t.Errorf("isPublicHost(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func staticRemotes(m map[string]string) func() (map[string]string, error) {
	return func() (map[string]string, error) { return m, nil }
}

func TestResolveShipRemote_ExplicitShipRemote(t *testing.T) {
	remotes := map[string]string{
		"origin": "https://github.com/topcheese/forgefix.git",
		"nas":    "ssh://git@192.168.1.18:2222/Jimmy/forgefix.git",
	}
	cfg := &Config{ShipRemote: "nas"}
	d, err := resolveShipRemote(cfg, staticRemotes(remotes))
	if err != nil {
		t.Fatal(err)
	}
	if d.Remote != "nas" || d.Public {
		t.Errorf("got %+v, want remote=nas public=false", d)
	}
}

func TestResolveShipRemote_ExplicitMissing(t *testing.T) {
	remotes := map[string]string{"origin": "https://github.com/x/y.git"}
	cfg := &Config{ShipRemote: "doesnotexist"}
	_, err := resolveShipRemote(cfg, staticRemotes(remotes))
	if err == nil {
		t.Fatal("expected error for missing ship_remote")
	}
}

func TestResolveShipRemote_BaseURLMatch(t *testing.T) {
	remotes := map[string]string{
		"origin": "https://github.com/topcheese/forgefix.git",
		"nas":    "ssh://git@192.168.1.18:2222/Jimmy/forgefix.git",
	}
	cfg := &Config{GitHub: &GitHubConfig{BaseURL: "https://192.168.1.18:2222/api/v1"}}
	d, err := resolveShipRemote(cfg, staticRemotes(remotes))
	if err != nil {
		t.Fatal(err)
	}
	if d.Remote != "nas" || d.Public {
		t.Errorf("got %+v, want remote=nas public=false", d)
	}
}

func TestResolveShipRemote_FallbackOriginPublic(t *testing.T) {
	remotes := map[string]string{
		"origin": "https://github.com/topcheese/forgefix.git",
	}
	cfg := &Config{} // no ship_remote, no github base_url
	d, err := resolveShipRemote(cfg, staticRemotes(remotes))
	if err != nil {
		t.Fatal(err)
	}
	if d.Remote != "origin" || !d.Public {
		t.Errorf("got %+v, want remote=origin public=true", d)
	}
}

func TestResolveShipRemote_NoRemotes(t *testing.T) {
	_, err := resolveShipRemote(&Config{}, staticRemotes(map[string]string{}))
	if err == nil {
		t.Fatal("expected error when no remotes")
	}
}
