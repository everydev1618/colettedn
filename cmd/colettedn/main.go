package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	vega "github.com/everydev1618/govega"
	"github.com/everydev1618/govega/dsl"
	"github.com/everydev1618/govega/serve"
	"github.com/everydev1618/govega/tools"

	"github.com/everydev1618/colettedn/internal/rdap"
	cdntools "github.com/everydev1618/colettedn/internal/tools"
)

//go:embed colette.vega.yaml
var agentYAML []byte

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(Version)
		return
	}

	loadEnvFile()

	// Prompt for API key if not set
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		fmt.Print("Enter your Anthropic API key: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			key := strings.TrimSpace(scanner.Text())
			if key != "" {
				os.Setenv("ANTHROPIC_API_KEY", key)
				// Save for next time
				home, _ := os.UserHomeDir()
				if home != "" {
					os.MkdirAll(home+"/.vega", 0700)
					os.WriteFile(home+"/.vega/env", []byte("ANTHROPIC_API_KEY="+key+"\n"), 0600)
					fmt.Println("Saved to ~/.vega/env")
				}
			}
		}
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			fmt.Fprintln(os.Stderr, "API key is required. Get one at https://console.anthropic.com/")
			os.Exit(1)
		}
	}

	// Create RDAP client (fetches IANA bootstrap for broad TLD support)
	rdapClient := rdap.New()

	// Parse the embedded agent YAML
	parser := dsl.NewParser()
	doc, err := parser.Parse(agentYAML)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	// Create interpreter with lazy spawn
	interp, err := dsl.NewInterpreter(doc, dsl.WithLazySpawn())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating interpreter: %v\n", err)
		os.Exit(1)
	}
	defer interp.Shutdown()

	// Register custom tools
	registerTools(interp.Tools(), rdapClient)

	// Ensure vega home directory exists
	if err := vega.EnsureHome(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating vega home: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ColetteDN: %d agents, %d workflows\n", len(doc.Agents), len(doc.Workflows))

	// Start server
	cfg := serve.Config{
		Addr:   ":3001",
		DBPath: vega.DefaultDBPath(),
	}
	srv := serve.New(interp, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := srv.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func loadEnvFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.Open(home + "/.vega/env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		// ~/.vega/env takes precedence — it's the user's explicit config
		os.Setenv(strings.TrimSpace(key), strings.TrimSpace(val))
	}
}

func registerTools(t *tools.Tools, rdapClient *rdap.Client) {
	checkDomain := cdntools.NewCheckDomainTool(rdapClient)
	checkDomains := cdntools.NewCheckDomainsTool(rdapClient)

	t.Register("check_domain", tools.ToolDef{
		Description: "Check if a single domain name is available for registration using RDAP lookup.",
		Fn: func(ctx context.Context, params map[string]any) (string, error) {
			domain, _ := params["domain"].(string)
			result, err := checkDomain.Execute(ctx, domain)
			if err != nil {
				return "", err
			}
			if result.Error != "" {
				return fmt.Sprintf("%s: unknown (%s)", result.Domain, result.Error), nil
			}
			if result.Available {
				return fmt.Sprintf("%s: AVAILABLE", result.Domain), nil
			}
			return fmt.Sprintf("%s: TAKEN", result.Domain), nil
		},
		Params: map[string]tools.ParamDef{
			"domain": {Type: "string", Description: "Domain to check (e.g. mycoolapp.com)", Required: true},
		},
	})

	t.Register("check_domains", tools.ToolDef{
		Description: "Check availability of multiple domain names at once. More efficient than checking one at a time.",
		Fn: func(ctx context.Context, params map[string]any) (string, error) {
			domainsRaw, _ := params["domains"].([]any)
			domains := make([]string, 0, len(domainsRaw))
			for _, d := range domainsRaw {
				if s, ok := d.(string); ok {
					domains = append(domains, s)
				}
			}
			results, err := checkDomains.Execute(ctx, domains)
			if err != nil {
				return "", err
			}
			var out string
			for _, r := range results {
				status := "TAKEN"
				if r.Available {
					status = "AVAILABLE"
				}
				if r.Error != "" {
					status = fmt.Sprintf("UNKNOWN (%s)", r.Error)
				}
				out += fmt.Sprintf("%s: %s\n", r.Domain, status)
			}
			return out, nil
		},
		Params: map[string]tools.ParamDef{
			"domains": {Type: "array", Description: "List of domains to check (e.g. [\"foo.com\", \"bar.io\"])", Required: true},
		},
	})
}
