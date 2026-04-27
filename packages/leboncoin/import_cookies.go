package leboncoin

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

// importCookiesCmd parses a curl(1) command (the kind you get from
// Chrome DevTools → Copy as cURL) and merges every cookie it carries
// into the profile's `.ghostchrome-cookies.json` snapshot.
//
// updateOrCreate semantics: cookies whose `name` already exists in the
// snapshot have their value/expires refreshed in place; new cookies are
// appended. Untouched entries are preserved (so re-importing a curl
// that only carries `datadome` will just refresh that one).
func (f *CommandFactory) importCookiesCmd() *cobra.Command {
	var (
		fromFile string
		profile  string
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "import-cookies",
		Short: "Import cookies from a curl(1) command into the profile snapshot",
		Long: `Read a curl command (DevTools → Copy as cURL) and upsert every
cookie it carries into ~/.ghostchrome/profiles/<profile>/.ghostchrome-cookies.json.

Cookies are matched by name: existing entries are updated, new ones are
created, and entries already present in the snapshot but missing from the
curl are kept as-is. Useful for refreshing the rotated ` + "`datadome`" + ` cookie
without losing the rest of the session.

Sources accepted:
  - stdin (default):   pbpaste | ghostchrome leboncoin import-cookies
  - --from <file>:     ghostchrome leboncoin import-cookies --from req.sh
`,
		Example: `  pbpaste | ghostchrome leboncoin import-cookies
  ghostchrome leboncoin import-cookies --from /tmp/curl.sh
  ghostchrome leboncoin import-cookies --profile leboncoin --dry-run < req.sh`,
		Run: func(cmd *cobra.Command, args []string) {
			raw, err := readCurlInput(fromFile)
			if err != nil {
				exitErr("import-cookies", err)
			}
			pairs := parseCurlCookies(raw)
			if len(pairs) == 0 {
				exitErr("import-cookies", fmt.Errorf("no cookies found in input (looked for -b, --cookie, -H 'cookie: ...')"))
			}
			profileDir, err := engine.ResolveProfileDir(profile)
			if err != nil {
				exitErr("import-cookies", err)
			}

			existing, _ := engine.LoadCookiesJSON(profileDir)
			byName := make(map[string]int, len(existing))
			for i, c := range existing {
				byName[c.Name] = i
			}

			created, updated := 0, 0
			expires := float64(time.Now().Add(30 * 24 * time.Hour).Unix())
			for name, value := range pairs {
				rec := engine.CookieRecord{
					Name:     name,
					Value:    value,
					Domain:   ".leboncoin.fr",
					Path:     "/",
					Expires:  expires,
					Secure:   true,
					HTTPOnly: name == "datadome", // datadome is HttpOnly on lbc
					SameSite: "Lax",
				}
				if idx, ok := byName[name]; ok {
					// Preserve previously-known domain / path / flags so we
					// don't downgrade an httpOnly host-cookie just because
					// the curl payload doesn't carry that metadata.
					prev := existing[idx]
					prev.Value = value
					prev.Expires = expires
					existing[idx] = prev
					updated++
				} else {
					existing = append(existing, rec)
					byName[name] = len(existing) - 1
					created++
				}
			}

			fmt.Fprintf(os.Stderr, "parsed %d cookie(s) from curl input\n", len(pairs))
			fmt.Fprintf(os.Stderr, "  → %d updated, %d created, %d total in snapshot\n", updated, created, len(existing))
			if dryRun {
				fmt.Fprintf(os.Stderr, "[dry-run] not writing %s/%s\n", profileDir, engine.CookiesJSONFilename)
				return
			}
			if err := os.MkdirAll(profileDir, 0o700); err != nil {
				exitErr("import-cookies", err)
			}
			if err := engine.SaveCookiesJSON(profileDir, existing); err != nil {
				exitErr("import-cookies", err)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", profileDir+"/"+engine.CookiesJSONFilename)
		},
	}
	cmd.Flags().StringVar(&fromFile, "from", "", "Read curl text from this file (default: stdin)")
	cmd.Flags().StringVar(&profile, "profile", "leboncoin", "Target ghostchrome profile name")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Parse and report without writing the snapshot")
	return cmd
}

func readCurlInput(fromFile string) (string, error) {
	if fromFile != "" {
		b, err := os.ReadFile(fromFile)
		return string(b), err
	}
	st, _ := os.Stdin.Stat()
	if (st.Mode() & os.ModeCharDevice) != 0 {
		return "", fmt.Errorf("no input on stdin and --from not set")
	}
	b, err := io.ReadAll(os.Stdin)
	return string(b), err
}

// RE2 has no backreferences, so we match single- and double-quoted
// payloads with two regexes each. (?s) lets `.` cross newlines for
// multi-line cookies.
var (
	curlBSingle = regexp.MustCompile(`(?s)(?:^|\s)(?:-b|--cookie)\s+'([^']*)'`)
	curlBDouble = regexp.MustCompile(`(?s)(?:^|\s)(?:-b|--cookie)\s+"([^"]*)"`)
	curlHSingle = regexp.MustCompile(`(?is)(?:^|\s)-H\s+'\s*cookie\s*:\s*([^']*)'`)
	curlHDouble = regexp.MustCompile(`(?is)(?:^|\s)-H\s+"\s*cookie\s*:\s*([^"]*)"`)
)

// parseCurlCookies extracts every name=value pair from a curl shell
// command. Order is not preserved (we use a map) — that's fine because
// we merge into the existing snapshot by name.
func parseCurlCookies(curl string) map[string]string {
	// Normalize backslash-newline continuations (DevTools formats curl
	// across many lines) so the regexes can scan a single string.
	curl = strings.ReplaceAll(curl, "\\\n", " ")

	out := map[string]string{}
	collect := func(payload string) {
		for _, kv := range strings.Split(payload, ";") {
			kv = strings.TrimSpace(kv)
			eq := strings.IndexByte(kv, '=')
			if eq <= 0 {
				continue
			}
			name := strings.TrimSpace(kv[:eq])
			value := strings.TrimSpace(kv[eq+1:])
			if name == "" {
				continue
			}
			out[name] = value
		}
	}
	for _, re := range []*regexp.Regexp{curlBSingle, curlBDouble, curlHSingle, curlHDouble} {
		for _, m := range re.FindAllStringSubmatch(curl, -1) {
			collect(m[1])
		}
	}
	return out
}
