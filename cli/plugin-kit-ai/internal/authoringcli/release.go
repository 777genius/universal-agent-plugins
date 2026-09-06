package authoringcli

import "github.com/spf13/cobra"

// RejectionKey identifies private, terminal compatibility errors, never jobs.
const RejectionKey = "authoringcli.v1-rejection"

func NewReleaseAuthorCommand(factories ...Factory) (*cobra.Command, error) {
	c, err := NewAuthorCommand(factories...)
	if err == nil {
		c.Hidden = false
	}
	return c, err
}

func NewReleasePluginKitRoot(factories ...Factory) (*cobra.Command, error) {
	return NewPluginKitRoot(factories...)
}

// PrepareReleaseUtilities uses Cobra's real tree. Hidden retirement shims are
// removed before generating scripts, including generators that walk all nodes.
func PrepareReleaseUtilities(root *cobra.Command, completion bool) {
	if completion {
		var prune func(*cobra.Command)
		prune = func(c *cobra.Command) {
			for _, child := range c.Commands() {
				if child.Hidden {
					c.RemoveCommand(child)
				} else {
					prune(child)
				}
			}
		}
		prune(root)
	}
	root.InitDefaultCompletionCmd()
}
