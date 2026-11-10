package internal_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/lsj/copylingo"

// allowedInternalImports lists the internal packages each package's non-test code may import; changing a boundary
// means editing this table in the same diff (ADR-059 §5, §8.4).
var allowedInternalImports = map[string][]string{
	"model":         nil,
	"config":        nil,
	"callback":      nil,
	"observability": nil,
	"testutil":      nil,

	// Storage and external API implementations import only model and logging,
	// never config, service or the adapters (§8.4).
	"repository": {"model"},
	"redisstore": {"model"},
	"external":   {"model", "observability"},

	// Services build on storage and external API contracts (§5, §8.2).
	"service":  {"external", "model", "observability", "repository"},
	"pipeline": {"external", "model", "service"},

	// Adapters reach storage only through service features or their own
	// narrow interfaces, so no repository, redisstore or external import (§5).
	// config is allowed for the shared Mini App path constants (§8.8).
	"bot":       {"callback", "config", "model", "observability", "service"},
	"miniapp":   {"config", "model", "observability", "service"},
	"scheduler": {"model", "observability"},

	// Setup shared by the cmd/ binaries: connections and config → options
	// mapping. No package above may import it (ADR-069).
	"bootstrap": {"config", "external", "repository", "service"},
}

// driverOwners lists the internal packages allowed to import each driver or
// SDK, keyed by import path prefix. Only these packages see raw Redis,
// database, Telegram or external API types (ADR-059 §3, §5, §8.7).
var driverOwners = map[string][]string{
	"github.com/redis/go-redis/": {"redisstore"},
	"github.com/lib/pq": {
		"bootstrap",
		"repository",
	}, // bootstrap opens the connection (ADR-069)
	"github.com/jmoiron/sqlx": {
		"bootstrap",
		"repository",
		"service",
		"testutil",
	}, // service owns transactions (ADR-061)
	"github.com/go-telegram-bot-api/":   {"bot"},
	"github.com/sashabaranov/go-openai": {"external"},
	"github.com/aws/":                   {"external"},
}

func TestInternalImportBoundaries(t *testing.T) {
	command := exec.Command(
		"go",
		"list",
		"-f",
		"{{.ImportPath}}{{range .Imports}} {{.}}{{end}}",
		modulePath+"/internal/...",
	)
	var stderr strings.Builder
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf(
			"go list internal packages: %v\n%s",
			err,
			stderr.String(),
		)
	}

	for _, line := range strings.Split(
		strings.TrimSpace(string(out)),
		"\n",
	) {
		fields := strings.Fields(line)
		packageName, ok := strings.CutPrefix(
			fields[0],
			modulePath+"/internal/",
		)
		if !ok {
			continue // internal/ itself holds only this test
		}
		allowed, known := allowedInternalImports[packageName]
		if !known {
			t.Errorf(
				"internal/%s has no import rule; add it to allowedInternalImports",
				packageName,
			)
			continue
		}
		for _, imported := range fields[1:] {
			checkImport(
				t,
				packageName,
				imported,
				allowed,
			)
		}
	}
}

func checkImport(
	t *testing.T,
	packageName string,
	imported string,
	allowedInternal []string,
) {
	t.Helper()
	if local, ok := strings.CutPrefix(
		imported,
		modulePath+"/",
	); ok {
		dependency, ok := strings.CutPrefix(
			local,
			"internal/",
		)
		if !ok || !slices.Contains(
			allowedInternal,
			dependency,
		) {
			t.Errorf(
				"internal/%s must not import %s",
				packageName,
				local,
			)
		}
		return
	}
	for prefix, owners := range driverOwners {
		if strings.HasPrefix(
			imported,
			prefix,
		) && !slices.Contains(
			owners,
			packageName,
		) {
			t.Errorf(
				"internal/%s must not import %s; only %v may",
				packageName,
				imported,
				owners,
			)
		}
	}
}
