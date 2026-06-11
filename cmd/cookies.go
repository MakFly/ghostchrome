package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
)

var (
	flagCookieDomain   string
	flagCookiePath     string
	flagCookieSecure   bool
	flagCookieHTTPOnly bool
	flagCookieExpires  string
	flagCookieSameSite string
	flagCookieURL      string
)

var cookiesCmd = &cobra.Command{
	Use:   "cookies",
	Short: "CRUD browser cookies via CDP",
	Long: `Read, set, or delete browser cookies. Useful to prime an authenticated
session before scraping, or to export cookies for reuse.

Examples:
  ghostchrome cookies list --connect ws://...
  ghostchrome cookies list --domain github.com --connect ws://...
  ghostchrome cookies set session=abc --domain example.com --path / --secure --connect ws://...
  ghostchrome cookies delete session --domain example.com --connect ws://...
  ghostchrome cookies clear --connect ws://...`,
}

var cookiesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all browser cookies (optionally filter by --domain)",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := openPage()
		defer b.Close()

		cookies, err := b.RodBrowser().GetCookies()
		if err != nil {
			exitErr("cookies list", err)
		}

		var filtered []*proto.NetworkCookie
		for _, c := range cookies {
			if flagCookieDomain != "" && !domainMatches(c.Domain, flagCookieDomain) {
				continue
			}
			filtered = append(filtered, c)
		}

		text := formatCookieList(filtered)
		output(cookieListResult{Cookies: filtered, Count: len(filtered)}, text)
	},
}

type cookieListResult struct {
	Cookies []*proto.NetworkCookie `json:"cookies"`
	Count   int                    `json:"count"`
}

var cookiesSetCmd = &cobra.Command{
	Use:   "set <name=value>",
	Short: "Set a cookie (NAME=VALUE format)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, value, ok := strings.Cut(args[0], "=")
		if !ok || name == "" {
			exitErr("cookies set", fmt.Errorf("expected NAME=VALUE, got %q", args[0]))
		}

		param := &proto.NetworkCookieParam{
			Name:     name,
			Value:    value,
			Domain:   flagCookieDomain,
			Path:     flagCookiePath,
			Secure:   flagCookieSecure,
			HTTPOnly: flagCookieHTTPOnly,
			URL:      flagCookieURL,
		}
		if flagCookiePath == "" {
			param.Path = "/"
		}
		if flagCookieExpires != "" {
			expires, err := parseExpires(flagCookieExpires)
			if err != nil {
				exitErr("cookies set", err)
			}
			param.Expires = proto.TimeSinceEpoch(expires)
		}
		if flagCookieSameSite != "" {
			param.SameSite = proto.NetworkCookieSameSite(flagCookieSameSite)
		}

		b, _ := openPage()
		defer b.Close()

		if err := b.RodBrowser().SetCookies([]*proto.NetworkCookieParam{param}); err != nil {
			exitErr("cookies set", err)
		}

		type setResult struct {
			Action string `json:"action"`
			Name   string `json:"name"`
			Domain string `json:"domain,omitempty"`
		}
		text := fmt.Sprintf("Cookie set: %s (domain=%s path=%s)", name, param.Domain, param.Path)
		output(&setResult{Action: "set", Name: name, Domain: param.Domain}, text)
	},
}

var cookiesDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a cookie by name (optionally scoped by --domain / --path / --url)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		b, page := openPage()
		defer b.Close()

		req := proto.NetworkDeleteCookies{
			Name:   args[0],
			URL:    flagCookieURL,
			Domain: flagCookieDomain,
			Path:   flagCookiePath,
		}
		if err := req.Call(page); err != nil {
			exitErr("cookies delete", err)
		}

		type delResult struct {
			Action string `json:"action"`
			Name   string `json:"name"`
		}
		text := fmt.Sprintf("Cookie deleted: %s", args[0])
		output(&delResult{Action: "delete", Name: args[0]}, text)
	},
}

var cookiesClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all browser cookies",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, page := openPage()
		defer b.Close()

		if err := (proto.NetworkClearBrowserCookies{}).Call(page); err != nil {
			exitErr("cookies clear", err)
		}

		type clearResult struct {
			Action string `json:"action"`
		}
		output(&clearResult{Action: "clear"}, "All cookies cleared")
	},
}

// domainMatches checks whether a cookie's domain ("foo.example.com" or
// ".example.com") matches a user-provided filter ("example.com").
func domainMatches(cookieDomain, filter string) bool {
	cd := strings.TrimPrefix(cookieDomain, ".")
	f := strings.TrimPrefix(filter, ".")
	return cd == f || strings.HasSuffix(cd, "."+f)
}

// parseExpires accepts either an absolute RFC3339 timestamp or a relative
// duration ("24h", "30d"). Returns seconds since epoch as float64.
func parseExpires(s string) (float64, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return float64(t.Unix()), nil
	}
	// Accept "30d" shorthand for days.
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		d, err := time.ParseDuration(days + "h")
		if err == nil {
			return float64(time.Now().Add(d * 24).Unix()), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("--expires: expected RFC3339 date, \"30d\", or Go duration: %w", err)
	}
	return float64(time.Now().Add(d).Unix()), nil
}

func formatCookieList(cookies []*proto.NetworkCookie) string {
	if len(cookies) == 0 {
		return "No cookies"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[cookies] %d\n", len(cookies))
	for _, c := range cookies {
		flags := ""
		if c.HTTPOnly {
			flags += "H"
		}
		if c.Secure {
			flags += "S"
		}
		if c.Session {
			flags += "s"
		}
		if flags != "" {
			flags = " [" + flags + "]"
		}
		fmt.Fprintf(&sb, "  %s=%s @%s%s%s\n", c.Name, truncateValue(c.Value), c.Domain, c.Path, flags)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func truncateValue(v string) string {
	if len(v) > 40 {
		return v[:37] + "..."
	}
	return v
}

func init() {
	for _, c := range []*cobra.Command{cookiesListCmd, cookiesSetCmd, cookiesDeleteCmd, cookiesClearCmd} {
		c.Flags().StringVar(&flagCookieDomain, "domain", "", "Cookie domain (e.g. example.com)")
	}
	cookiesSetCmd.Flags().StringVar(&flagCookiePath, "path", "/", "Cookie path")
	cookiesSetCmd.Flags().BoolVar(&flagCookieSecure, "secure", false, "Secure flag")
	cookiesSetCmd.Flags().BoolVar(&flagCookieHTTPOnly, "http-only", false, "HttpOnly flag")
	cookiesSetCmd.Flags().StringVar(&flagCookieExpires, "expires", "", "Expiration (RFC3339, \"30d\", or Go duration like \"24h\")")
	cookiesSetCmd.Flags().StringVar(&flagCookieSameSite, "same-site", "", "SameSite: Strict, Lax, None")
	cookiesSetCmd.Flags().StringVar(&flagCookieURL, "url", "", "Associated URL (alternative to --domain + --path)")
	cookiesDeleteCmd.Flags().StringVar(&flagCookiePath, "path", "", "Cookie path to match")
	cookiesDeleteCmd.Flags().StringVar(&flagCookieURL, "url", "", "URL that owns the cookie")

	cookiesCmd.AddCommand(cookiesListCmd, cookiesSetCmd, cookiesDeleteCmd, cookiesClearCmd)
	rootCmd.AddCommand(cookiesCmd)
}
