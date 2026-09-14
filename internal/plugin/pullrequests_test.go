package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseReference_Valid(t *testing.T) {
	owner, repo, number, err := parseReference("myorg/myrepo#42")
	require.NoError(t, err)
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", repo)
	require.Equal(t, int64(42), number)
}

func TestParseReference_WithWhitespace(t *testing.T) {
	owner, repo, number, err := parseReference("  myorg/myrepo#42  ")
	require.NoError(t, err)
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", repo)
	require.Equal(t, int64(42), number)
}

func TestParseReference_MissingHash(t *testing.T) {
	_, _, _, err := parseReference("myorg/myrepo")
	require.ErrorContains(t, err, "expected owner/repo#number")
}

func TestParseReference_MissingOwner(t *testing.T) {
	_, _, _, err := parseReference("/myrepo#42")
	require.ErrorContains(t, err, "expected owner/repo#number")
}

func TestParseReference_InvalidNumber(t *testing.T) {
	_, _, _, err := parseReference("myorg/myrepo#abc")
	require.ErrorContains(t, err, "invalid PR number")
}

func TestParseReference_ZeroNumber(t *testing.T) {
	_, _, _, err := parseReference("myorg/myrepo#0")
	require.ErrorContains(t, err, "invalid PR number")
}
