package setcategory

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "setcategory {client} {category} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "setcategory"},
	Short:       "Set category of torrents in client.",
	Long: `Set category of torrents in client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read infoHash list from stdin, delimited by blanks.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: createtags,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	cmd.RootCmd.AddCommand(command)
}

func createtags(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	cat := args[1]
	infoHashes := args[2:]
	if category == "" && tag == "" && filter == "" {
		if len(infoHashes) == 0 {
			return fmt.Errorf("you must provide at least a condition flag or hashFilter")
		}
		if len(infoHashes) == 1 && infoHashes[0] == "-" {
			if config.InShell {
				return fmt.Errorf(`"-" arg can not be used in shell`)
			}
			if data, err := util.ReadArgsFromStdin(); err != nil {
				return fmt.Errorf("failed to parse stdin to info hashes: %v", err)
			} else if len(data) == 0 {
				return nil
			} else {
				infoHashes = data
			}
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	infoHashes, err = client.SelectTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if infoHashes == nil {
		err = clientInstance.SetAllTorrentsCatetory(cat)
		if err != nil {
			return err
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.SetTorrentsCatetory(infoHashes, cat)
		if err != nil {
			return err
		}
	}
	return nil
}
