package edittorrent

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "edittorrent {torrentFilename}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "edittorrent"},
	Aliases:     []string{"edit"},
	Short:       "Edit local .torrent (metainfo) files.",
	Long: `Edit .torrent (metainfo) files.
It will update local disk .torrent files in place.
It only supports editing / updating of fields that does NOT affect the info-hash of the torrent.
Args is the torrent filename list. Use a single "-" as args to read the list from stdin, delimited by blanks.

It will ask for confirm before updateing torrent files, unless --force flag is set.

Required flags (at least one of them must be set):
* --remove-tracker
* --add-tracker
* --update-tracker
* --update-created-by
* --update-creation-date
* --update-comment

If --backup flag is set, it will create a backup of original torrent file before updating it.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: edittorrent,
}

var (
	force              = false
	doBackup           = false
	removeTracker      = ""
	addTracker         = ""
	updateTracker      = ""
	updateCreatedBy    = ""
	updateCreationDate = ""
	updateComment      = ""
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Do update torrent files without confirm")
	command.Flags().BoolVarP(&doBackup, "backup", "", false,
		"Backup original .torrent file to *"+constants.FILENAME_SUFFIX_BACKUP+
			" unless it's name already has that suffix. If the same name backup file already exists, it will be overwrited")
	command.Flags().StringVarP(&removeTracker, "remove-tracker", "", "",
		"Remove tracker from torrents. If the tracker does NOT exists in the torrent, do nothing")
	command.Flags().StringVarP(&addTracker, "add-tracker", "", "",
		"Add new tracker to torrents. If the tracker already exists in the torrent, do nothing")
	command.Flags().StringVarP(&updateTracker, "update-tracker", "", "",
		"Set the tracker of torrents. It will become the sole tracker of torrents, all existing ones will be removed")
	command.Flags().StringVarP(&updateCreatedBy, "update-created-by", "", "", `Update "created by" field of torrents`)
	command.Flags().StringVarP(&updateCreationDate, "update-creation-date", "", "",
		`Update "creation date" field of torrents. E.g.: "2024-01-20 15:00:00" (local timezone), `+
			"or a unix timestamp integer (seconds)")
	command.Flags().StringVarP(&updateComment, "update-comment", "", "", `Update "comment" field of torrents`)
	cmd.RootCmd.AddCommand(command)
}

func edittorrent(cmd *cobra.Command, args []string) error {
	torrents, stdinTorrentContents, err := helper.ParseTorrentsFromArgs(args)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		log.Infof("No torrents found")
		return nil
	}
	if len(torrents) == 1 && torrents[0] == "-" {
		return fmt.Errorf(`"-" as reading .torrent content from stdin is NOT supported here`)
	}
	if util.CountNonZeroVariables(removeTracker, addTracker, updateTracker,
		updateCreatedBy, updateCreationDate, updateComment) == 0 {
		return fmt.Errorf(`at least one "--add-*", "--remove-*" or "--update-*" flag must be set`)
	}
	if updateTracker != "" && (addTracker != "" || removeTracker != "") {
		return fmt.Errorf(`"--update-tracker" flag is NOT compatible with "--remove-tracker" or "--add-tracker" flags`)
	}
	errorCnt := int64(0)
	cntTorrents := int64(0)

	if !force {
		fmt.Printf("Will edit (update) the following .torrent files:")
		for _, torrent := range torrents {
			fmt.Printf("  %q", torrent)
		}
		fmt.Printf("\n\nApplying the below modifications:\n-----\n")
		if removeTracker != "" {
			fmt.Printf("Remove tracker: %q\n", removeTracker)
		}
		if addTracker != "" {
			fmt.Printf("Add tracker: %q\n", addTracker)
		}
		if updateTracker != "" {
			fmt.Printf("Update tracker: %q\n", updateTracker)
		}
		if updateCreatedBy != "" {
			fmt.Printf("Update 'created_by' field: %q\n", updateCreatedBy)
		}
		if updateCreationDate != "" {
			fmt.Printf("Update 'creation_date' field: %q\n", updateCreationDate)
		}
		if updateComment != "" {
			fmt.Printf("Update 'comment' field: %q\n", updateComment)
		}
		fmt.Printf("-----\n\n")
		if !helper.AskYesNoConfirm("Will update torrent files") {
			return fmt.Errorf("abort")
		}
	}

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, _, err := helper.GetTorrentContent(torrent, "", true, false,
			stdinTorrentContents, false, nil)
		if err != nil {
			log.Errorf("Failed to parse %s: %v", torrent, err)
			errorCnt++
			continue
		}
		changed := false
		if removeTracker != "" {
			if _err := tinfo.RemoveTracker(removeTracker); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err == nil && addTracker != "" {
			if _err := tinfo.AddTracker(addTracker, -1); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err == nil && updateTracker != "" {
			if _err := tinfo.UpdateTracker(updateTracker); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err == nil && updateCreatedBy != "" {
			if _err := tinfo.UpdateCreatedBy(updateCreatedBy); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err == nil && updateCreationDate != "" {
			if _err := tinfo.UpdateCreationDate(updateCreationDate); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err == nil && updateComment != "" {
			if _err := tinfo.UpdateComment(updateComment); _err != nil {
				if _err != torrentutil.ErrNoChange {
					err = _err
				}
			} else {
				changed = true
			}
		}
		if err != nil {
			fmt.Printf("✕ %s : failed to update torrent: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if !changed {
			fmt.Printf("- %s : no change\n", torrent)
			continue
		}
		if doBackup && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_BACKUP) {
			if err := util.CopyFile(torrent, util.TrimAnySuffix(torrent,
				constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_BACKUP); err != nil {
				fmt.Printf("✕ %s : abort updating file due to failed to create backup file: %v\n", torrent, err)
				errorCnt++
				continue
			}
		}
		if data, err := tinfo.ToBytes(); err != nil {
			fmt.Printf("✕ %s : failed to generate new contents: %v\n", torrent, err)
			errorCnt++
		} else if err := os.WriteFile(torrent, data, constants.PERM); err != nil {
			fmt.Printf("✕ %s : failed to write new contents: %v\n", torrent, err)
			errorCnt++
		} else {
			fmt.Printf("✓ %s : successfully updated\n", torrent)
			cntTorrents++
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "// Updated torrents: %d\n", cntTorrents)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
