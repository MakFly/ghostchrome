package engine

import "testing"

func TestCmdlineIsSessionServe(t *testing.T) {
	e := SessionEntry{Name: "work", Port: 9222, Profile: "work"}
	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{"exact match (space form)", "/usr/local/bin/ghostchrome serve --port 9222 --user-profile work --headless=true", true},
		{"renamed binary still matches", "/home/kev/.local/bin/gc serve --port 9222 --user-profile work", true},
		{"exact match (equals form)", "ghostchrome serve --port=9222 --user-profile=work", true},
		{"substring port must NOT match", "ghostchrome serve --port 92223 --user-profile work", false},
		{"different profile must NOT match", "ghostchrome serve --port 9222 --user-profile other", false},
		{"missing profile must NOT match", "ghostchrome serve --port 9222 --headless=true", false},
		{"not a serve must NOT match", "ghostchrome agent --port 9222 --user-profile work", false},
		{"unrelated process must NOT match", "/usr/bin/node /home/kev/server.js --port 9222", false},
		{"port as bare substring elsewhere must NOT match", "ghostchrome serve --port 7000 --user-profile work9222", false},
	}
	for _, c := range cases {
		if got := cmdlineIsSessionServe(c.cmd, e); got != c.want {
			t.Errorf("%s: cmdlineIsSessionServe(%q) = %v, want %v", c.name, c.cmd, got, c.want)
		}
	}
}
