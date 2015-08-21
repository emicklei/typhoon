package main

import (
	"os"
	"strconv"

	"github.com/emicklei/artreyu/command"
	"github.com/emicklei/artreyu/local"
	"github.com/emicklei/artreyu/model"
	"github.com/spf13/cobra"
)

var (
	VERSION   string = "dev"
	BUILDDATE string = "now"

	applicationSettings *model.Settings
	rootCmd             *cobra.Command
)

func main() {
	model.Printf("artreyu - artifact assembly tool (build:%s, commit:%s)\n", BUILDDATE, VERSION)
	initRootCommand()
	rootCmd.Execute()
}

func initRootCommand() {
	rootCmd = &cobra.Command{
		Use:   "artreyu",
		Short: "archives, fetches and assembles build artifacts",
		Long: `A tool for handling versioned, platform dependent artifacts.
Its primary purpose is to create assembly artifacts from build artifacts archived in a (remote) repository.

See https://github.com/emicklei/artreyu for more details.

(c)2015 http://ernestmicklei.com, MIT license`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	applicationSettings = command.NewSettingsBoundToFlags(rootCmd)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// TODO refactor this
		model.Verbose = applicationSettings.Verbose
		if applicationSettings.Verbose {
			dir, _ := os.Getwd()
			model.Printf("working directory = [%s]", dir)
		}
	}

	archive := command.NewCommandForArchive()
	archive.Run = func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			model.Fatalf("archive failed, missing source argument")
		}
		artifact, err := model.LoadArtifact(applicationSettings.ArtifactConfigLocation)
		if err != nil {
			model.Fatalf("archive failed, invalid artifact: %v", err)
		}
		descriptorArtifact := model.Artifact{
			Api:     artifact.Api,
			AnyOS:   artifact.AnyOS,
			Group:   artifact.Group,
			Name:    artifact.Name,
			Version: artifact.Version,
			Type:    artifact.Type,
		}
		descriptorArtifact.UseStorageBase("artreyu.yaml")
		repoName := applicationSettings.TargetRepository
		// put versions in local repo.
		// put snapshots in local store if local is target
		if !artifact.IsSnapshot() || "local" == repoName {
			archive := command.Archive{
				Artifact:    artifact,
				Repository:  local.NewRepository(model.RepositoryConfigNamed(applicationSettings, "local"), applicationSettings.OS),
				Source:      args[0],
				ExitOnError: false,
			}
			ok := archive.Perform()
			if ok {
				model.Printf("... stored artifact in local cache")
				// now store the descriptor
				descArchive := command.Archive{
					Artifact:    descriptorArtifact,
					Repository:  local.NewRepository(model.RepositoryConfigNamed(applicationSettings, "local"), applicationSettings.OS),
					Source:      applicationSettings.ArtifactConfigLocation,
					ExitOnError: false,
				}
				if descArchive.Perform() {
					model.Printf("... stored descriptor in local cache")
				}
			} else {
				model.Printf("[WARN] unable to store artifact in local cache")
			}
		}
		// done if local is target
		if "local" == repoName {
			return
		}
		// not local, no archive specific flags to add
		if err := command.RunPluginWithArtifact("artreyu-"+repoName, "archive", artifact, *applicationSettings, args); err != nil {
			model.Fatalf("archive failed, could not run plugin: %v", err)
		} else {
			// now store the descriptor. replace the source in args
			args[0] = applicationSettings.ArtifactConfigLocation
			command.RunPluginWithArtifact("artreyu-"+repoName, "archive", descriptorArtifact, *applicationSettings, args)
		}
	}
	rootCmd.AddCommand(archive)

	fetch := command.NewCommandForFetch()
	fetch.Run = func(cmd *cobra.Command, args []string) {
		artifact, err := model.LoadArtifact(applicationSettings.ArtifactConfigLocation)
		if err != nil {
			model.Fatalf("fetch failed, unable to load artifact: %v", err)
		}
		repoName := applicationSettings.TargetRepository
		var destination = "."
		if len(args) > 0 {
			destination = args[0]
		}
		// versions may be in local store
		// snapshots are in local store if target is set to local
		fetched := false
		if !artifact.IsSnapshot() || "local" == repoName {
			fetch := command.Fetch{
				Artifact:    artifact,
				Repository:  local.NewRepository(model.RepositoryConfigNamed(applicationSettings, "local"), applicationSettings.OS),
				Destination: destination,
				AutoExtract: command.AutoExtract,
				ExitOnError: false,
			}
			if fetch.Perform() {
				model.Printf("... fetched artifact from local cache")
				fetched = fetch.Perform()
			}
		}
		// done if target is set to local or local fetch of version was ok
		if "local" == repoName || fetched {
			return
		}

		// extend args with fetch specific flags
		extendedArgs := append(args, "--extract="+strconv.FormatBool(command.AutoExtract))

		// not local
		if err := command.RunPluginWithArtifact("artreyu-"+repoName, "fetch", artifact, *applicationSettings, extendedArgs); err != nil {
			model.Fatalf("fetch failed, could not run plugin:  %v", err)
		} else {
			// remote fetching succeeded, store a copy of a version in local
			if !artifact.IsSnapshot() {
				archive := command.Archive{
					Artifact:    artifact,
					Repository:  local.NewRepository(model.RepositoryConfigNamed(applicationSettings, "local"), applicationSettings.OS),
					Source:      destination,
					ExitOnError: false,
				}
				if archive.Perform() {
					model.Printf("... stored copy in local cache")
				}
			}
		}
	}
	rootCmd.AddCommand(fetch)
	rootCmd.AddCommand(newAssembleCmd())
	rootCmd.AddCommand(newFormatCmd())
	rootCmd.AddCommand(newTreeCmd())
}
