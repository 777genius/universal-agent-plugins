package commands

import (
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
)

const operationKey = "authoring.public-operation"

// Tag only factories supplied by this engine. Enclosing installer commands and
// Cobra utilities cannot become author capabilities by having a matching name.
func tagOperations(c *cobra.Command, prefix string) {
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations[operationKey] = prefix
	for _, child := range c.Commands() {
		tagOperations(child, prefix+"."+child.Name())
	}
}
func implementedLeaves(root *cobra.Command) []string {
	var out []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c == nil {
			return
		}
		if !c.Hidden && !c.HasSubCommands() && c.Runnable() && c.Annotations[operationKey] != "" {
			out = append(out, c.Annotations[operationKey])
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Strings(out)
	return out
}

type selection struct {
	command                 *cobra.Command
	operation, mode, format string
	help, invalid           bool
	flags                   []string
	values                  map[string][]string
	positional, helpCommand bool
}

// selectPublic observes definitions without executing Cobra's parser or accessing a project.
// Unknown arguments freeze command routing; known flags still consume their actual arity.
func selectPublic(root *cobra.Command, args []string) selection {
	s := selection{command: root, operation: "author", mode: "read", format: "human", values: map[string][]string{}}
	for i := 0; i < len(args); {
		next, stop := s.consumeToken(args, i)
		if stop {
			break
		}
		i = next
	}
	s.finalize()
	return s
}

// consumeToken records exactly the argument span selected for parsing.
func (s *selection) consumeToken(args []string, i int) (next int, stop bool) {
	start := i
	token := args[i]
	if token == "--" {
		if i+1 < len(args) && s.command.HasSubCommands() {
			s.invalid = true
		}
		return len(args), true
	}
	if strings.HasPrefix(token, "--") {
		unknown, record := s.consumeLongFlag(args, &i)
		if unknown {
			s.positional = true
		}
		if record {
			s.flags = append(s.flags, args[start:i+1]...)
		}
		return i + 1, false
	}
	if strings.HasPrefix(token, "-") && token != "-" {
		if s.consumeShortFlags(args, &i) {
			s.positional = true
		}
		s.flags = append(s.flags, args[start:i+1]...)
		return i + 1, false
	}
	s.consumeRouteToken(token)
	return i + 1, false
}

func (s *selection) consumeRouteToken(token string) {
	if s.positional {
		return
	}
	if token == "help" && s.command.HasSubCommands() {
		s.helpCommand = true
		return
	}
	next := s.findChild(token)
	if next == nil {
		s.positional = true
		if s.command.HasSubCommands() || s.helpCommand {
			s.invalid = true
		}
		return
	}
	s.command = next
	if id := next.Annotations[operationKey]; id != "" {
		s.operation = id
	}
}

func (s *selection) lookupFlag(name string, short bool) *pflag.Flag {
	for c := s.command; c != nil; c = c.Parent() {
		for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
			var f *pflag.Flag
			if short {
				f = fs.ShorthandLookup(name)
			} else {
				f = fs.Lookup(name)
			}
			if f != nil {
				return f
			}
		}
	}
	return nil
}

func (s *selection) recordFlag(f *pflag.Flag, value string) {
	s.values[f.Name] = append(s.values[f.Name], value)
	if f.Name == "format" {
		s.format = value
	}
	if f.Name == "help" {
		v, err := strconv.ParseBool(value)
		s.help = v
		if err != nil {
			s.invalid = true
		}
	}
	switch f.Name {
	case "scope", "project-root", "accept-security-risk", "security-details":
		s.invalid = true
	}
}

// consumeLongFlag distinguishes unknown arity from a recognized but missing value.
func (s *selection) consumeLongFlag(args []string, i *int) (unknown, recorded bool) {
	name, value, equal := strings.Cut(args[*i][2:], "=")
	f := s.lookupFlag(name, false)
	if f == nil {
		s.invalid = true
		return true, false
	}
	if !equal {
		if f.NoOptDefVal != "" {
			value = f.NoOptDefVal
		} else if *i+1 < len(args) {
			*i += 1
			value = args[*i]
		} else {
			s.values[f.Name] = append(s.values[f.Name], "")
			s.invalid = true
			return false, false
		}
	}
	s.recordFlag(f, value)
	return false, true
}

// consumeShortFlags preserves the original cluster and trailing-value policy.
func (s *selection) consumeShortFlags(args []string, i *int) (unknown bool) {
	cluster := args[*i][1:]
	for cluster != "" {
		f := s.lookupFlag(cluster[:1], true)
		cluster = cluster[1:]
		if f == nil {
			s.invalid = true
			unknown = true
			continue
		}
		value := f.NoOptDefVal
		if strings.HasPrefix(cluster, "=") {
			value = cluster[1:]
			cluster = ""
		} else if f.NoOptDefVal == "" {
			if cluster != "" {
				value = cluster
				cluster = ""
			} else if *i+1 < len(args) {
				*i += 1
				value = args[*i]
			} else {
				s.invalid = true
				break
			}
		}
		s.recordFlag(f, value)
	}
	return unknown
}

func (s selection) findChild(token string) *cobra.Command {
	for _, c := range s.command.Commands() {
		if (c.Name() == token || c.Annotations[operationKey] == "" && c.HasAlias(token)) &&
			(!c.Hidden || c.Name() == "author" || c.Annotations[authoringcli.RejectionKey] != "") {
			return c
		}
	}
	return nil
}

func (s *selection) finalize() {
	s.help = s.help || s.helpCommand
	if id := s.command.Annotations[operationKey]; id != "" {
		s.operation = id
	} else if s.command.Annotations[authoringcli.RejectionKey] == "" && !isCompletion(s.command) {
		s.invalid = true
	}
	s.finalizeMode()
	if s.format != "human" && s.format != "json" {
		s.invalid = true
	}
}

func (s *selection) finalizeMode() {
	if s.operation == "author.init" || s.operation == "author.skills.init" || s.operation == "author.bootstrap" {
		s.mode = "local_mutation"
	}
	if values := s.values["dry-run"]; len(values) > 0 {
		dry, err := strconv.ParseBool(values[len(values)-1])
		if err != nil || s.operation != "author.bootstrap" {
			s.invalid = true
		} else if dry {
			s.mode = "read"
		}
	}
	if s.operation == "author.normalize" || s.operation == "author.import.native" {
		values := s.values["write"]
		if len(values) > 0 {
			write, err := strconv.ParseBool(values[len(values)-1])
			if err != nil {
				s.invalid = true
			} else if write {
				s.mode = "local_mutation"
			}
		}
	}
	if s.operation == "author.test" && len(s.values["runtime"]) > 0 || s.operation == "author.dev" {
		s.mode = "runtime"
	}
}
