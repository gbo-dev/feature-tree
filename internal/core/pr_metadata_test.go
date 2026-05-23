package core

import "testing"

func TestForkRemoteName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		owner string
		want  string
	}{
		{owner: "octocat", want: "ft-fork-octocat"},
		{owner: "My_Org", want: "ft-fork-my-org"},
		{owner: "", want: "ft-fork-head"},
	}
	for _, tc := range cases {
		if got := forkRemoteName(tc.owner); got != tc.want {
			t.Fatalf("forkRemoteName(%q) = %q, want %q", tc.owner, got, tc.want)
		}
	}
}
