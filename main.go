package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

var VERSION string = "dev"
var BUILDDATE string = "now"

func main() {
	RootCmd.AddCommand(newArchiveCmd())
	RootCmd.AddCommand(newFetchCmd())
	RootCmd.AddCommand(newListCmd())
	RootCmd.AddCommand(newVersionCmd())
	RootCmd.Execute()
}

var RootCmd = &cobra.Command{
	Use:   "typhoon",
	Short: "typhoon a is a tool for artifact management",
	Run:   func(cmd *cobra.Command, args []string) {},
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "show build info",
		Run: func(cmd *cobra.Command, args []string) {
			log.Println("_/^\\_")
			log.Println(" | | typhoon - artifact assembly tool [commit=", VERSION, "build=", BUILDDATE, "]")
			log.Println("-\\_/-")
		},
	}
}

type artifactCmd struct {
	*cobra.Command
	artifact string
	group    string
	version  string
}

func newListCmd() *cobra.Command {
	cmd := newArtifactCmd(&cobra.Command{
		Use:   "list",
		Short: "list all available artifacts from the typhoon repository",
	})
	cmd.Command.Run = cmd.doList
	return cmd.Command
}

func (c *artifactCmd) doList(cmd *cobra.Command, args []string) {
	g := path.Join(strings.Split(c.group, ".")...)
	group := path.Join(getRepo(), g, c.artifact, c.version)
	files, _ := ioutil.ReadDir(group)
	for _, f := range files {
		fmt.Println(f.Name())
	}
}

type archiveCmd struct {
	*artifactCmd
	overwrite bool
}

func newArtifactCmd(cobraCmd *cobra.Command) *artifactCmd {
	cmd := new(artifactCmd)
	cmd.Command = cobraCmd
	cmd.PersistentFlags().StringVar(&cmd.artifact, "artifact", ".", "file location of artifact to copy")
	cmd.PersistentFlags().StringVar(&cmd.group, "group", ".", "folder containing the artifacts")
	cmd.PersistentFlags().StringVar(&cmd.version, "version", ".", "version of the artifact")
	return cmd
}

func newArchiveCmd() *cobra.Command {
	cmd := newArtifactCmd(&cobra.Command{
		Use:   "archive [artifact]",
		Short: "copy an artifact to the typhoon repository",
	})
	archiveCmd := new(archiveCmd)
	archiveCmd.artifactCmd = cmd
	archiveCmd.PersistentFlags().BoolVar(&archiveCmd.overwrite, "force", false, "force overwrite if version exists")
	cmd.Command.Run = archiveCmd.doArchive
	return cmd.Command
}

func getRepo() string {
	repo := os.Getenv("TYPHOON_REPO")
	if len(repo) == 0 {
		log.Fatal("missing TYPHOON_REPO environment setting")
	}
	return repo
}

func (a *archiveCmd) doArchive(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		log.Fatalf("missing artifact")
	}
	source := args[len(args)-1]
	g := path.Join(strings.Split(a.group, ".")...)
	regular := path.Base(path.Clean(source))
	p := path.Join(getRepo(), g, a.artifact, a.version)

	log.Printf("copying %s into folder %s\n", regular, p)
	if err := os.MkdirAll(p, os.ModePerm); err != nil {
		log.Fatalf("unable to create dirs: %s cause: %v", p, err)
	}
	dest := path.Join(p, regular)

	// SNAPSHOT can be overwritten
	if strings.HasSuffix(a.version, "SNAPSHOT") {
		log.Println("will overwrite|create SNAPSHOT version")
		a.overwrite = true
	}
	if !a.overwrite && Exists(dest) {
		log.Fatalf("unable to copy artifact: %s to: %s cause: it already exists and --force=false", source, p)
	}
	if err := Cp(dest, source); err != nil {
		log.Fatalf("unable to copy artifact: %s to: %s cause:%v", source, p, err)
	}
}

func Exists(dest string) bool {
	_, err := os.Stat(dest)
	return err == nil
}

func Cp(dst, src string) error {
	return exec.Command("cp", src, dst).Run()
}

// Copy does what is says. Ignores errors on Close though.
func Copy(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
