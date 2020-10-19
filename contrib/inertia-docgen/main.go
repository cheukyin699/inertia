/*

Inertia-docgen is a tool for generating Inertia command documentation.

For example, to generate a man-page reference:

	inertia contrib docgen --ouput $PATH --format man

Generated Markdown documentation is currently published to
https://inertia.ubclaunchpad.com/cli

Learn more about `inertia/contrib` tools:

	inertia contrib -h

*/
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/ubclaunchpad/inertia/cfg"
	"github.com/ubclaunchpad/inertia/cmd"
	"github.com/ubclaunchpad/inertia/cmd/core"
	"github.com/ubclaunchpad/inertia/cmd/core/utils/out"
	remotescmd "github.com/ubclaunchpad/inertia/cmd/remotes"
)

var (
	// Version denotes the version of the binary
	Version string

	mdReadmeTemplate = `# Inertia Command Reference

Click [here](/inertia.md) for the Inertia CLI command reference. It is generated
automatically using ` + "`inertia-docgen`." + `

For a more general usage guide, refer to the [Inertia Usage Guide](https://inertia.ubclaunchpad.com).

For documentation regarding the daemon API, refer to the [API Reference](https://inertia.ubclaunchpad.com/api).

* Generated: %s
* Version: %s
`
)

func main() {
	os.Setenv(out.EnvColorToggle, "false")
	var root = cmd.NewInertiaCmd(Version, "~/.inertia", false)
	if err := newDocgenCmd(root).Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func newDocgenCmd(root *core.Cmd) *cobra.Command {
	const (
		flagOutput = "output"
		flagFormat = "format"
	)
	var docs = &cobra.Command{
		Use:     "inertia-docgen",
		Hidden:  true,
		Version: Version,
		Short:   "Generate command reference for the Inertia CLI.",
		Run: func(cmd *cobra.Command, args []string) {
			var outPath, _ = cmd.Flags().GetString(flagOutput)
			var format, _ = cmd.Flags().GetString(flagFormat)

			// create *full* Inertia tree, for sake of documentation
			remotescmd.AttachRemoteHostCmd(root,
				remotescmd.CmdOptions{
					RemoteCfg: &cfg.Remote{Name: "${remote_name}"},
				},
				false)

			// set up file tree
			os.MkdirAll(outPath, os.ModePerm)

			// gen docs
			switch format {
			case "man":
				if err := doc.GenManTree(root.Command, &doc.GenManHeader{
					Title: "Inertia CLI Command Reference",
					Source: fmt.Sprintf(
						"Generated by inertia-docgen %s",
						root.Version),
					Manual: "https://inertia.ubclaunchpad.com",
				}, outPath); err != nil {
					out.Fatal(err.Error())
				}
			default:
				if err := doc.GenMarkdownTree(root.Command, outPath); err != nil {
					out.Fatal(err.Error())
				}
				var readme = fmt.Sprintf(mdReadmeTemplate, time.Now().Format("2006-Jan-02"), Version)
				ioutil.WriteFile(filepath.Join(outPath, "README.md"), []byte(readme), os.ModePerm)
			}

			fmt.Printf("%s documentation generated in %s\n", format, outPath)
		},
	}
	docs.Flags().StringP(flagOutput, "o", "./docs/cli", "output file path")
	docs.Flags().StringP(flagFormat, "f", "md", "format to generate (md|man)")
	return docs
}
