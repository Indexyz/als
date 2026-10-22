package fakeshell

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/reeflective/console"
	"github.com/samlm0/als/v2/config"
	"github.com/samlm0/als/v2/fakeshell/commands"
	"github.com/spf13/cobra"
)

func filterNexttraceArgs(args []string) ([]string, error) {
	for _, arg := range args {
		if arg == "--" {
			return args, nil
		}
		if arg == "--file" || strings.HasPrefix(arg, "--file=") ||
			arg == "--output" || strings.HasPrefix(arg, "--output=") ||
			arg == "--output-default" || arg == "--mtr-record" || strings.HasPrefix(arg, "--mtr-record=") ||
			arg == "--mtr-replay" || strings.HasPrefix(arg, "--mtr-replay=") ||
			arg == "--setup-api-v4-token" || arg == "--init" ||
			arg == "-o" || strings.HasPrefix(arg, "-o=") || arg == "-O" ||
			(strings.HasPrefix(arg, "-o") && len(arg) > len("-o") && arg[2] != '-') {
			return nil, errors.New("dangerous nexttrace file flag detected, stop running")
		}
	}
	return args, nil
}

func defineMenuCommands(a *console.Console) console.Commands {
	showedIsFirstTime := false
	return func() *cobra.Command {
		rootCmd := &cobra.Command{}

		rootCmd.InitDefaultHelpCmd()
		rootCmd.CompletionOptions.DisableDefaultCmd = true
		rootCmd.DisableFlagsInUseLine = true

		features := map[string]bool{
			"ping":       config.Config.FeaturePing,
			"traceroute": config.Config.FeatureTraceroute,
			"nexttrace":  config.Config.FeatureTraceroute,
			"speedtest":  config.Config.FeatureSpeedtestDotNet,
			"mtr":        config.Config.FeatureMTR,
		}

		argsFilter := map[string]func([]string) ([]string, error){
			"nexttrace": filterNexttraceArgs,
			"ping": func(args []string) ([]string, error) {
				var re = regexp.MustCompile(`(?m)^-?f$|^-\S+f\S*$`)
				for _, str := range args {
					if len(re.FindAllString(str, -1)) != 0 {
						return []string{}, errors.New("dangerous flag detected, stop running")
					}
				}
				return args, nil
			},
		}

		hasNotFound := false

		argsPassthough := func(args []string) ([]string, error) {
			return args, nil
		}

		for command, feature := range features {
			if feature {
				_, err := exec.LookPath(command)
				if err != nil {
					if !showedIsFirstTime {
						fmt.Println("Error: " + command + " is not install")
					}
					hasNotFound = true
					continue
				}
				filter, ok := argsFilter[command]
				if !ok {
					filter = argsPassthough
				}
				commands.AddExecureableAsCommand(rootCmd, command, filter)
			}
		}

		if hasNotFound {
			showedIsFirstTime = true
		}

		rootCmd.SetHelpCommand(&cobra.Command{
			Use:    "no-help",
			Hidden: true,
		})

		return rootCmd
	}
}
