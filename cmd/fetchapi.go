package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	fetchapiMethod    string
	fetchapiHeaders   []string
	fetchapiData      string
	fetchapiDataFile  string
	fetchapiDataStdin bool
	fetchapiTimeoutMs int
	fetchapiOutput    string
	fetchapiPretty    bool
	fetchapiBodyOnly  bool
)

var fetchapiCmd = &cobra.Command{
	Use:   "fetchapi <url>",
	Short: "Replay a JSON / XHR endpoint with custom headers and body — no Chrome",
	Long: `fetchapi is the JSON-API counterpart to fastfetch. It does a single
HTTP request with the method, headers and body you provide and returns
the response as a JSON envelope { status, url, elapsed_ms, content_length,
is_json, json|body_text, headers }.

Useful for:
  - replaying XHR endpoints captured via 'ghostchrome --observe'
  - hitting known REST / GraphQL / Algolia APIs directly
  - bulk-scraping pipelines that bypass the HTML layer entirely

Examples:
  # Algolia search via the dedicated subcommand (auto-builds the request)
  ghostchrome fetchapi algolia \
    --app-id 691K8M71IA --api-key 95874bf3cc96f8de61eced3440501724 \
    --index production_cars \
    --params 'query=&hitsPerPage=5&page=0'

  # Generic POST with JSON body
  ghostchrome fetchapi --method POST \
    --header 'Content-Type=application/json' \
    --data '{"q":"audi a3"}' \
    https://api.example.com/search

  # GET with cookie auth
  ghostchrome fetchapi --header 'Cookie=session=abc' https://target.fr/api/me

  # Pipe a body from a file
  ghostchrome fetchapi --method POST --data-file payload.json https://api.example.com/x`,
	Args: cobra.ExactArgs(1),
	Run:  runFetchapi,
}

var fetchapiAlgoliaCmd = &cobra.Command{
	Use:   "algolia",
	Short: "Convenience: build and send an Algolia search query",
	Run:   runFetchapiAlgolia,
}

var (
	algoliaAppID    string
	algoliaAPIKey   string
	algoliaIndex    string
	algoliaEndpoint string
	algoliaParams   string
)

func init() {
	fetchapiCmd.Flags().StringVar(&fetchapiMethod, "method", "GET", "HTTP method")
	fetchapiCmd.Flags().StringArrayVar(&fetchapiHeaders, "header", nil, "Extra request header K=V (repeatable)")
	fetchapiCmd.Flags().StringVar(&fetchapiData, "data", "", "Request body (string)")
	fetchapiCmd.Flags().StringVar(&fetchapiDataFile, "data-file", "", "Read request body from this file")
	fetchapiCmd.Flags().BoolVar(&fetchapiDataStdin, "data-stdin", false, "Read request body from stdin")
	fetchapiCmd.Flags().IntVar(&fetchapiTimeoutMs, "timeout-ms", 8000, "HTTP timeout in milliseconds")
	fetchapiCmd.Flags().StringVarP(&fetchapiOutput, "output", "o", "", "Write payload to this path (default: stdout)")
	fetchapiCmd.Flags().BoolVar(&fetchapiPretty, "pretty", false, "Pretty-print JSON output")
	fetchapiCmd.Flags().BoolVar(&fetchapiBodyOnly, "body-only", false, "Output the response body only (drops envelope; exit 1 on non-2xx)")

	fetchapiAlgoliaCmd.Flags().StringVar(&algoliaAppID, "app-id", "", "Algolia application id (required)")
	fetchapiAlgoliaCmd.Flags().StringVar(&algoliaAPIKey, "api-key", "", "Algolia search API key (required)")
	fetchapiAlgoliaCmd.Flags().StringVar(&algoliaIndex, "index", "", "Algolia index name (required)")
	fetchapiAlgoliaCmd.Flags().StringVar(&algoliaEndpoint, "endpoint", "", "Override DSN endpoint (default: https://<app-id>-dsn.algolia.net/1/indexes/<index>/query)")
	fetchapiAlgoliaCmd.Flags().StringVar(&algoliaParams, "params", "query=&hitsPerPage=20&page=0", "Algolia params form-string")
	fetchapiAlgoliaCmd.Flags().IntVar(&fetchapiTimeoutMs, "timeout-ms", 8000, "HTTP timeout in milliseconds")
	fetchapiAlgoliaCmd.Flags().StringVarP(&fetchapiOutput, "output", "o", "", "Write payload to this path (default: stdout)")
	fetchapiAlgoliaCmd.Flags().BoolVar(&fetchapiPretty, "pretty", false, "Pretty-print JSON output")
	fetchapiAlgoliaCmd.Flags().BoolVar(&fetchapiBodyOnly, "body-only", false, "Output only the Algolia response body (drops envelope)")

	fetchapiCmd.AddCommand(fetchapiAlgoliaCmd)
	rootCmd.AddCommand(fetchapiCmd)
}

func runFetchapi(_ *cobra.Command, args []string) {
	target := args[0]

	headers, err := parseFastfetchHeaders(fetchapiHeaders)
	if err != nil {
		exitErr("fetchapi", err)
	}
	body := readFetchapiBody()

	res, err := engine.FastFetchAPI(context.Background(), engine.APIRequest{
		Method:  fetchapiMethod,
		URL:     target,
		Headers: headers,
		Body:    body,
		Timeout: time.Duration(fetchapiTimeoutMs) * time.Millisecond,
		Proxy:   flagProxy,
	})
	if err != nil {
		exitErr("fetchapi", err)
	}

	emitFetchapiResponse(res)
}

func runFetchapiAlgolia(_ *cobra.Command, _ []string) {
	if algoliaAppID == "" || algoliaAPIKey == "" || algoliaIndex == "" {
		exitErr("fetchapi algolia", fmt.Errorf("--app-id, --api-key and --index are required"))
	}
	res, err := engine.AlgoliaQuery(context.Background(), engine.AlgoliaOpts{
		AppID:      algoliaAppID,
		APIKey:     algoliaAPIKey,
		Index:      algoliaIndex,
		Endpoint:   algoliaEndpoint,
		ParamsForm: algoliaParams,
		Timeout:    time.Duration(fetchapiTimeoutMs) * time.Millisecond,
		Proxy:      flagProxy,
	})
	if err != nil {
		exitErr("fetchapi algolia", err)
	}
	emitFetchapiResponse(res)
}

func readFetchapiBody() []byte {
	switch {
	case fetchapiDataStdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			exitErr("fetchapi data-stdin", err)
		}
		return b
	case fetchapiDataFile != "":
		safe, err := validateOutputPath(fetchapiDataFile)
		if err != nil {
			exitErr("fetchapi data-file", err)
		}
		b, err := os.ReadFile(safe)
		if err != nil {
			exitErr("fetchapi data-file", err)
		}
		return b
	case fetchapiData != "":
		return []byte(fetchapiData)
	}
	return nil
}

func emitFetchapiResponse(res *engine.APIResponse) {
	w, closeFn := openFetchapiWriter()
	defer closeFn()

	if fetchapiBodyOnly {
		if res.Status >= 400 {
			fmt.Fprintf(os.Stderr, "[fetchapi] non-2xx status=%d\n", res.Status)
			defer os.Exit(1)
		}
		if res.IsJSON && fetchapiPretty {
			var v interface{}
			if err := json.Unmarshal(res.JSON, &v); err == nil {
				enc := json.NewEncoder(w)
				enc.SetEscapeHTML(false)
				enc.SetIndent("", "  ")
				_ = enc.Encode(v)
				return
			}
		}
		if res.IsJSON {
			_, _ = w.Write(res.JSON)
			fmt.Fprintln(w)
			return
		}
		_, _ = w.Write(res.Body)
		return
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if fetchapiPretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(res); err != nil {
		exitErr("fetchapi encode", err)
	}
	fmt.Fprintf(os.Stderr, "[fetchapi] %s status=%d is_json=%v size=%dB elapsed=%dms\n",
		res.URL, res.Status, res.IsJSON, res.ContentLength, res.ElapsedMs)
}

func openFetchapiWriter() (writeCloser, func()) {
	if fetchapiOutput == "" {
		return stdoutWriteCloser{}, func() {}
	}
	safe, err := validateOutputPath(fetchapiOutput)
	if err != nil {
		exitErr("output", err)
	}
	f, err := os.Create(safe)
	if err != nil {
		exitErr("output", err)
	}
	return f, func() { _ = f.Close() }
}

// (parseFastfetchHeaders is defined in cmd/fastfetch.go; we reuse it here.)
var _ = strings.Index // keep import alive when run* paths skip strings
