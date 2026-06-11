package cmd

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage persistent Chrome profiles (~/.ghostchrome/profiles)",
	Long: `Persistent profiles store cookies/cache so logins and sessions survive.
They accumulate disk over time — list them and remove the ones you no longer
need. Tearing a session down with --purge also removes its profile.`,
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles with their on-disk size",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		profiles, err := engine.ListProfiles()
		if err != nil {
			exitErr("list profiles", err)
		}
		sort.Slice(profiles, func(i, j int) bool { return profiles[i].Bytes > profiles[j].Bytes })

		if flagFormat == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			_ = enc.Encode(profiles)
			return
		}
		if len(profiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no profiles")
			return
		}
		var total int64
		fmt.Fprintf(cmd.OutOrStdout(), "%-24s  %s\n", "NAME", "SIZE")
		for _, p := range profiles {
			total += p.Bytes
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s  %s\n", p.Name, humanBytes(p.Bytes))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-24s  %s\n", "(total)", humanBytes(total))
	},
}

var profilesRmCmd = &cobra.Command{
	Use:   "rm <name> [name...]",
	Short: "Remove one or more profiles and reclaim their disk",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		removed := 0
		for _, name := range args {
			if err := engine.RemoveProfile(name); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "skip %q: %v\n", name, err)
				continue
			}
			removed++
			if flagFormat != "json" {
				fmt.Fprintf(cmd.OutOrStdout(), "removed profile %q\n", name)
			}
		}
		if flagFormat == "json" {
			fmt.Fprintf(cmd.OutOrStdout(), `{"removed":%d}`+"\n", removed)
		}
	},
}

func init() {
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesRmCmd)
	rootCmd.AddCommand(profilesCmd)
	commandGroups["profiles"] = "session"
}
