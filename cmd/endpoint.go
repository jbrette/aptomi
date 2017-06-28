package cmd

import (
	"fmt"
	"github.com/Frostman/aptomi/pkg/slinga"
	"github.com/spf13/cobra"
	"sort"
	. "github.com/Frostman/aptomi/pkg/slinga/language"
)

var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Services endpoints control",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var endpointCmdShow = &cobra.Command{
	Use:   "show",
	Short: "Show endpoints for deployed services",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		// User loader
		userLoader := NewAptomiUserLoader()

		// Load the previous usage state
		state := slinga.LoadServiceUsageState(userLoader)

		endpoints := state.Endpoints("")

		var keys []string
		for key := range endpoints {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			keyEndpoints := endpoints[key]
			serviceName, contextName, allocationName, componentName := slinga.ParseServiceUsageKey(key)
			fmt.Println("Service:", serviceName, " |  Context:", contextName, " |  Allocation:", allocationName, " |  Component:", componentName)

			for tp, url := range keyEndpoints {
				fmt.Println("	", tp, url)
			}

			fmt.Println("")
		}
	},
}

func init() {
	endpointCmd.AddCommand(endpointCmdShow)
	RootCmd.AddCommand(endpointCmd)
}
