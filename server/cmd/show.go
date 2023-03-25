package cmd

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/lexers"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/soft-serve/server/backend"
	"github.com/charmbracelet/soft-serve/ui/common"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var (
	lineDigitStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	lineBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	dirnameStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))
	filenameStyle  = lipgloss.NewStyle()
	filemodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
)

// showCommand returns a command that prints the contents of a file.
func showCommand() *cobra.Command {
	var linenumber bool
	var color bool

	showCmd := &cobra.Command{
		Use:               "show PATH",
		Aliases:           []string{"cat"},
		Short:             "Read the contents of file at path.",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: checkIfReadable,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, s := fromContext(cmd)
			ps := strings.Split(args[0], "/")
			rn := strings.TrimSuffix(ps[0], ".git")
			fp := strings.Join(ps[1:], "/")
			auth := cfg.Access.AccessLevel(rn, s.PublicKey())
			if auth < backend.ReadOnlyAccess {
				return ErrUnauthorized
			}
			var repo backend.Repository
			repoExists := false
			repos, err := cfg.Backend.Repositories()
			if err != nil {
				return err
			}
			for _, rp := range repos {
				if rp.Name() == rn {
					repoExists = true
					repo = rp
					break
				}
			}
			if !repoExists {
				return ErrRepoNotFound
			}
			c, _, err := backend.LatestFile(repo, fp)
			if err != nil {
				return err
			}
			if color {
				c, err = withFormatting(fp, c)
				if err != nil {
					return err
				}
			}
			if linenumber {
				c = withLineNumber(c, color)
			}
			cmd.Println(c)
			return nil
		},
	}
	showCmd.Flags().BoolVarP(&linenumber, "linenumber", "l", false, "Print line numbers")
	showCmd.Flags().BoolVarP(&color, "color", "c", false, "Colorize output")

	return showCmd
}

func withLineNumber(s string, color bool) string {
	lines := strings.Split(s, "\n")
	// NB: len() is not a particularly safe way to count string width (because
	// it's counting bytes instead of runes) but in this case it's okay
	// because we're only dealing with digits, which are one byte each.
	mll := len(fmt.Sprintf("%d", len(lines)))
	for i, l := range lines {
		digit := fmt.Sprintf("%*d", mll, i+1)
		bar := "│"
		if color {
			digit = lineDigitStyle.Render(digit)
			bar = lineBarStyle.Render(bar)
		}
		if i < len(lines)-1 || len(l) != 0 {
			// If the final line was a newline we'll get an empty string for
			// the final line, so drop the newline altogether.
			lines[i] = fmt.Sprintf(" %s %s %s", digit, bar, l)
		}
	}
	return strings.Join(lines, "\n")
}

func withFormatting(p, c string) (string, error) {
	zero := uint(0)
	lang := ""
	lexer := lexers.Match(p)
	if lexer != nil && lexer.Config() != nil {
		lang = lexer.Config().Name
	}
	formatter := &gansi.CodeBlockElement{
		Code:     c,
		Language: lang,
	}
	r := strings.Builder{}
	styles := common.StyleConfig()
	styles.CodeBlock.Margin = &zero
	rctx := gansi.NewRenderContext(gansi.Options{
		Styles:       styles,
		ColorProfile: termenv.TrueColor,
	})
	err := formatter.Render(&r, rctx)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}