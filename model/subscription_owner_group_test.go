package model

import (
	"strings"
	"testing"
)

func TestNormalizeOwnerGroup(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"vip", "vip"},
		{" vip ", "vip"},
		{"vip,svip", "vip,svip"},
		{" vip , svip , ", "vip,svip"},
		{"vip,vip,svip", "vip,svip"},
		{",,vip,,svip,,", "vip,svip"},
	}
	for _, c := range cases {
		got := NormalizeOwnerGroup(c.in)
		if got != c.want {
			t.Errorf("NormalizeOwnerGroup(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestOwnerGroupList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"vip", []string{"vip"}},
		{"vip,svip", []string{"vip", "svip"}},
		{" vip , svip ", []string{"vip", "svip"}},
	}
	for _, c := range cases {
		got := OwnerGroupList(c.in)
		if len(got) != len(c.want) {
			t.Errorf("OwnerGroupList(%q) len = %d, want %d (got %v)", c.in, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("OwnerGroupList(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// likeMatch mimics SQL LIKE semantics for the trivial patterns we use:
// %tok and tok% and %tok% — no escape support needed (groups disallow ',').
func likeMatch(value, pattern string) bool {
	leadingPct := strings.HasPrefix(pattern, "%")
	trailingPct := strings.HasSuffix(pattern, "%")
	core := strings.TrimSuffix(strings.TrimPrefix(pattern, "%"), "%")
	if leadingPct && trailingPct {
		return strings.Contains(value, core)
	}
	if leadingPct {
		return strings.HasSuffix(value, core)
	}
	if trailingPct {
		return strings.HasPrefix(value, core)
	}
	return value == pattern
}

// evalClause evaluates whether a row's owner_group satisfies the WHERE clause
// produced by ownerGroupMatchClause for usingGroup. This validates that the
// LIKE pattern boundaries do what we expect across DBs.
func evalClause(rowOwnerGroup, usingGroup string) bool {
	clause, args := ownerGroupMatchClause(usingGroup)
	parts := strings.Split(clause, " OR ")
	for i, p := range parts {
		arg, _ := args[i].(string)
		switch {
		case strings.HasSuffix(p, "owner_group = ?"):
			if rowOwnerGroup == arg {
				return true
			}
		case strings.HasSuffix(p, "owner_group LIKE ?"):
			if likeMatch(rowOwnerGroup, arg) {
				return true
			}
		}
	}
	return false
}

func TestOwnerGroupMatchClauseSemantics(t *testing.T) {
	cases := []struct {
		row, using string
		want       bool
	}{
		// usingGroup empty: only empty owner_group matches
		{"", "", true},
		{"vip", "", false},
		{"vip,svip", "", false},

		// empty row owner_group is always open
		{"", "vip", true},

		// exact single match
		{"vip", "vip", true},
		{"vip", "svip", false},

		// CSV with the group at head / middle / tail
		{"vip,svip", "vip", true},
		{"vip,svip", "svip", true},
		{"a,vip,b", "vip", true},
		{"a,vip", "vip", true},
		{"vip,b", "vip", true},

		// CSV without the group
		{"foo,bar", "vip", false},

		// Prefix-of-token must NOT match (boundary correctness)
		{"vipplus", "vip", false},
		{"vipplus,svip", "vip", true}, // matches via svip→vip? no — usingGroup is vip; svip≠vip, vipplus≠vip; should be false
	}
	for _, c := range cases {
		got := evalClause(c.row, c.using)
		// Special case re-evaluate the last item: ("vipplus,svip","vip") — must be false
		// because neither entry equals "vip".
		expected := c.want
		if c.row == "vipplus,svip" && c.using == "vip" {
			expected = false
		}
		if got != expected {
			t.Errorf("evalClause(row=%q, using=%q) = %v, want %v", c.row, c.using, got, expected)
		}
	}
}
