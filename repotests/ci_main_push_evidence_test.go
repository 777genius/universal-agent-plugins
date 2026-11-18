package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The release verifier looks every required check up by exact name on the
// tagged commit or its merged PR head. A workflow that produces one of those
// checks on a push to main must keep that run: a newer push supersedes only
// the same pull request, and each push to main gets a group of its own, so it
// is neither cancelled while running nor replaced while pending. Workflows
// that produce no required check are out of scope and keep their own rules.
const releaseEvidenceCancel = "${{ github.event_name == 'pull_request' }}"

var releaseEvidenceGroup = regexp.MustCompile(`^[a-z0-9-]+-\$\{\{ github\.event_name \}\}-\$\{\{ github\.event_name == 'pull_request' && github\.event\.pull_request\.number \|\| github\.run_id \}\}$`)

type evidenceJob struct {
	Name        string    `yaml:"name"`
	If          string    `yaml:"if"`
	Uses        string    `yaml:"uses"`
	Needs       yaml.Node `yaml:"needs"`
	Concurrency yaml.Node `yaml:"concurrency"`
	Strategy    struct {
		Matrix yaml.Node `yaml:"matrix"`
	} `yaml:"strategy"`
}

type evidenceWorkflow struct {
	On          map[string]yaml.Node   `yaml:"on"`
	Concurrency yaml.Node              `yaml:"concurrency"`
	Jobs        map[string]evidenceJob `yaml:"jobs"`
}

type concurrencySpec struct {
	Group  string `yaml:"group"`
	Cancel string `yaml:"cancel-in-progress"`
}

func TestCISupersession_ReleaseEvidenceMainPushRunsAreKept(t *testing.T) {
	root := RepoRoot(t)
	required := releaseRequiredChecks(t, readRepoFile(t, root, ".github", "workflows", "agentplugins-release.yml"))

	entries, err := os.ReadDir(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	producedOnMain := map[string][]string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		var w evidenceWorkflow
		if err := yaml.Unmarshal([]byte(readRepoFile(t, root, ".github", "workflows", name)), &w); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		producers := map[string][]string{}
		for id, job := range w.Jobs {
			for _, check := range checkNames(t, name, id, job) {
				if slices.Contains(required, check) {
					producers[id] = append(producers[id], check)
				}
			}
		}
		if len(producers) == 0 || !pushRunsOnMain(t, name, w) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			c := concurrencyOf(t, w.Concurrency)
			if !releaseEvidenceGroup.MatchString(c.Group) || c.Cancel != releaseEvidenceCancel {
				t.Errorf("a push to main must get a group of its own and only pull requests may be superseded: %+v", c)
			}
			for id, checks := range producers {
				for _, job := range append([]string{id}, transitiveNeeds(t, w, id)...) {
					spec := w.Jobs[job]
					if spec.Concurrency.Kind != 0 {
						t.Errorf("job %s (release checks %v) must not declare job-level concurrency", job, checks)
					}
					if !runsOnEveryPush(spec.If) {
						t.Errorf("job %s (release checks %v) may skip on a push to main: if %q", job, checks, spec.If)
					}
				}
				for _, check := range checks {
					producedOnMain[check] = append(producedOnMain[check], name)
				}
			}
		})
	}
	for _, check := range required {
		if len(producedOnMain[check]) == 0 {
			t.Errorf("release requires %q, but no workflow produces it on a push to main", check)
		}
	}
}

// releaseRequiredChecks reads the names from the release verifier. Every
// require_check argument must parse, so a quoted name cannot be skipped.
func releaseRequiredChecks(t *testing.T, release string) []string {
	t.Helper()
	argument := func(raw string) string {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), `\`))
		switch {
		case len(raw) >= 2 && (raw[0] == '\'' || raw[0] == '"') && raw[len(raw)-1] == raw[0]:
			return raw[1 : len(raw)-1]
		case regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`).MatchString(raw):
			return raw
		}
		t.Fatalf("cannot parse release required check %q", raw)
		return ""
	}
	var checks []string
	for _, match := range regexp.MustCompile(`(?m)^\s*require_check\s+(.+)$`).FindAllStringSubmatch(release, -1) {
		if strings.TrimSpace(match[1]) == `"${name}"` {
			continue
		}
		checks = append(checks, argument(match[1]))
	}
	loops := regexp.MustCompile(`(?s)for name in \\\n(.*?); do\n\s+require_check "\$\{name\}"`).FindAllStringSubmatch(release, -1)
	if len(loops) != strings.Count(release, `require_check "${name}"`) {
		t.Fatal("every require_check loop must use the parsed `for name in \\ ...; do` form")
	}
	for _, loop := range loops {
		for _, line := range strings.Split(loop[1], "\n") {
			if strings.TrimSpace(line) != "" {
				checks = append(checks, argument(line))
			}
		}
	}
	if len(checks) < 10 || !slices.Contains(checks, "test") {
		t.Fatalf("parsed an implausible release required check list: %v", checks)
	}
	sort.Strings(checks)
	return checks
}

// checkNames expands a job's check names: the job id without a name, or the
// name with every `${{ matrix.<key> }}` placeholder replaced by each value of
// that key in the matrix lists and include entries. Reusable workflow calls
// report "<caller> / <callee>" names, which the release never requires.
func checkNames(t *testing.T, workflow, id string, job evidenceJob) []string {
	t.Helper()
	if job.Uses != "" {
		return nil
	}
	if job.Name == "" {
		return []string{id}
	}
	placeholder := regexp.MustCompile(`\$\{\{ matrix\.([A-Za-z0-9_-]+) \}\}`)
	if strings.Count(job.Name, "${{") != len(placeholder.FindAllString(job.Name, -1)) {
		// Not statically comparable. An extra producer never removes
		// evidence; a required name that only such a job produced is
		// reported as missing below.
		return nil
	}
	names := []string{job.Name}
	for _, key := range placeholder.FindAllStringSubmatch(job.Name, -1) {
		values := matrixValues(t, workflow, id, job.Strategy.Matrix, key[1])
		var expanded []string
		for _, name := range names {
			for _, value := range values {
				expanded = append(expanded, strings.Replace(name, key[0], value, 1))
			}
		}
		names = expanded
	}
	return names
}

func matrixValues(t *testing.T, workflow, id string, node yaml.Node, key string) []string {
	t.Helper()
	if node.Kind != yaml.MappingNode {
		// A computed matrix cannot be expanded statically; such jobs report
		// no comparable check name.
		return nil
	}
	matrix := map[string]yaml.Node{}
	if err := node.Decode(&matrix); err != nil {
		t.Fatalf("%s job %s matrix: %v", workflow, id, err)
	}
	var values []string
	if list, ok := matrix[key]; ok && list.Kind == yaml.SequenceNode {
		var items []string
		if err := list.Decode(&items); err == nil {
			values = append(values, items...)
		}
	}
	if include, ok := matrix["include"]; ok && include.Kind == yaml.SequenceNode {
		var entries []map[string]any
		if err := include.Decode(&entries); err == nil {
			for _, entry := range entries {
				if value, ok := entry[key].(string); ok {
					values = append(values, value)
				}
			}
		}
	}
	return values
}

// pushRunsOnMain applies GitHub's branch filter patterns, including globs and
// negations, to refs/heads/main.
func pushRunsOnMain(t *testing.T, workflow string, w evidenceWorkflow) bool {
	t.Helper()
	push, ok := w.On["push"]
	if !ok {
		return false
	}
	var filter struct {
		Branches       []string `yaml:"branches"`
		BranchesIgnore []string `yaml:"branches-ignore"`
		Tags           []string `yaml:"tags"`
		TagsIgnore     []string `yaml:"tags-ignore"`
	}
	if push.Kind != 0 && !(push.Kind == yaml.ScalarNode && push.Value == "") {
		if err := push.Decode(&filter); err != nil {
			t.Fatalf("%s push filter: %v", workflow, err)
		}
	}
	switch {
	case len(filter.BranchesIgnore) > 0:
		return !branchFilterMatches(filter.BranchesIgnore, "main")
	case len(filter.Branches) > 0:
		return branchFilterMatches(filter.Branches, "main")
	default:
		// Only a tag filter limits a push trigger to tags.
		return len(filter.Tags) == 0 && len(filter.TagsIgnore) == 0
	}
}

func branchFilterMatches(patterns []string, branch string) bool {
	matched := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		if globPattern(strings.TrimPrefix(pattern, "!")).MatchString(branch) {
			matched = !negated
		}
	}
	return matched
}

func globPattern(pattern string) *regexp.Regexp {
	var out strings.Builder
	out.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				out.WriteString(".*")
				i++
			} else {
				out.WriteString("[^/]*")
			}
		case '?', '+':
			// GitHub: zero or one, or one or more, of the preceding character.
			out.WriteByte(c)
		case '[', ']':
			out.WriteByte(c)
		default:
			out.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	out.WriteString("$")
	compiled, err := regexp.Compile(out.String())
	if err != nil {
		return regexp.MustCompile(`^\b$`) // matches nothing
	}
	return compiled
}

func concurrencyOf(t *testing.T, node yaml.Node) concurrencySpec {
	t.Helper()
	var spec concurrencySpec
	switch node.Kind {
	case 0:
	case yaml.ScalarNode:
		spec.Group = node.Value
	default:
		if err := node.Decode(&spec); err != nil {
			t.Fatal(err)
		}
	}
	return spec
}

func transitiveNeeds(t *testing.T, w evidenceWorkflow, id string) []string {
	t.Helper()
	seen := map[string]bool{}
	var walk func(string)
	walk = func(job string) {
		node := w.Jobs[job].Needs
		var needs []string
		switch node.Kind {
		case 0:
		case yaml.ScalarNode:
			needs = []string{node.Value}
		default:
			if err := node.Decode(&needs); err != nil {
				t.Fatal(err)
			}
		}
		for _, parent := range needs {
			if !seen[parent] {
				seen[parent] = true
				walk(parent)
			}
		}
	}
	walk(id)
	var out []string
	for job := range seen {
		out = append(out, job)
	}
	sort.Strings(out)
	return out
}

// runsOnEveryPush accepts no condition, or a top-level || whose first operand
// is the plain `github.event_name != 'pull_request'`, which is true for every
// push. Status functions would replace GitHub's implicit success() and are
// not accepted.
func runsOnEveryPush(condition string) bool {
	expression := strings.TrimSpace(condition)
	if expression == "" {
		return true
	}
	if strings.HasPrefix(expression, "${{") && strings.HasSuffix(expression, "}}") {
		expression = strings.TrimSpace(expression[3 : len(expression)-2])
	}
	for _, function := range []string{"always()", "cancelled()", "failure()", "success()"} {
		if strings.Contains(expression, function) {
			return false
		}
	}
	operands := topLevelOr(expression)
	return len(operands) > 1 && strings.TrimSpace(operands[0]) == "github.event_name != 'pull_request'"
}

// topLevelOr splits an expression on || outside parentheses and quotes.
func topLevelOr(expression string) []string {
	var operands []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(expression); i++ {
		switch c := expression[i]; {
		case c == '\'':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && strings.HasPrefix(expression[i:], "||"):
			operands = append(operands, expression[start:i])
			start = i + 2
			i++
		}
	}
	return append(operands, expression[start:])
}
