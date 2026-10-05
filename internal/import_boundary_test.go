package internal_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/lsj/copylingo"

// allowedInternalImports lists, for each package under internal/, the other
// internal packages its non-test code may import. A package missing from this
// map, or an import missing from its list, fails the test: changing a package
// boundary means editing this table in the same diff (ADR-059 §5, §8.4).
var allowedInternalImports = map[string][]string{
	// Leaf packages: shared types, settings and logging.
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
	"scheduler": {"model", "observability", "pipeline"},
}

// driverOwners lists the internal packages allowed to import each driver or
// SDK, keyed by import path prefix. Only these packages see raw Redis,
// database, Telegram or external API types (ADR-059 §3, §5, §8.7).
var driverOwners = map[string][]string{
	"github.com/redis/go-redis/":        {"redisstore"},
	"github.com/lib/pq":                 {"repository"},
	"github.com/jmoiron/sqlx":           {"repository", "service", "testutil"}, // service owns transactions (ADR-061)
	"github.com/go-telegram-bot-api/":   {"bot"},
	"github.com/sashabaranov/go-openai": {"external"},
	"github.com/aws/":                   {"external"},
}

// TestInternalImportBoundaries checks the non-test imports of every internal
// package, as reported by go list, against the tables above.
func TestInternalImportBoundaries(t *testing.T) {
	cmd := exec.Command(
		"go",
		"list",
		"-f",
		"{{.ImportPath}}{{range .Imports}} {{.}}{{end}}",
		modulePath+"/internal/...",
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
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
		pkg, ok := strings.CutPrefix(
			fields[0],
			modulePath+"/internal/",
		)
		if !ok {
			continue // internal/ itself holds only this test
		}
		allowed, known := allowedInternalImports[pkg]
		if !known {
			t.Errorf(
				"internal/%s has no import rule; add it to allowedInternalImports",
				pkg,
			)
			continue
		}
		for _, imported := range fields[1:] {
			checkImport(
				t,
				pkg,
				imported,
				allowed,
			)
		}
	}
}

func checkImport(
	t *testing.T,
	pkg string,
	imported string,
	allowedInternal []string,
) {
	t.Helper()
	if local, ok := strings.CutPrefix(
		imported,
		modulePath+"/",
	); ok {
		dep, ok := strings.CutPrefix(
			local,
			"internal/",
		)
		if !ok || !slices.Contains(
			allowedInternal,
			dep,
		) {
			t.Errorf(
				"internal/%s must not import %s",
				pkg,
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
			pkg,
		) {
			t.Errorf(
				"internal/%s must not import %s; only %v may",
				pkg,
				imported,
				owners,
			)
		}
	}
}
