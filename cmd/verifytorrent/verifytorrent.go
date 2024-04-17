package verifytorrent

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/rclone"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "verifytorrent {torrentFilename | torrentId | torrentUrl}... " +
		"{--save-path dir | --content-path path | --use-comment-meta | --rclone-lsjson-file file | " +
		"--rclone-save-path path} [--check | --check-quick]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "verifytorrent"},
	Aliases:     []string{"verify"},
	Short:       "Verify .torrent (metainfo) files are consistent with local disk contents.",
	Long: fmt.Sprintf(`Verify .torrent (metainfo) files are consistent with local disk contents.
%s.

Example:
ptool verifytorrent file1.torrent file2.torrent --save-path /root/Downloads
ptool verifytorrent file.torrent --content-path /root/Downloads/TorrentContentFolder
ptool verifytorrent *.torrent --rclone-save-path remote:Downloads

Exact one (not less or more) of the following flags must be set.
* --save-path dir : the save path ("Downloads" folder) of torrent content(s).
  It can be used with multiple torrent args.
* --content-path path : the torrent contents path, could be a folder or a single file.
  It can only be used with single torrent arg.
* --use-comment-meta : extract save path from torrent's comment field and use it.
  The "ptool export" and some other cmds can use the same flag to write save path to generated torrent files.
* --rclone-lsjson-file : The filename of index contents that "rclone lsjson --recursive <path>" outputs
  ptool treats <path> as the save path of torrent contents and verify torrents against the index contents.
  For more, see https://github.com/rclone/rclone and https://rclone.org/commands/rclone_lsjson/ .
* --rclone-save-path : The rclone "save path". Instead of reading from --rclone-lsjson-file file, 
  ptool will directly execute "rclone lsjson --recursive <rclone-save-path>"
  and use it's output as index contents. E.g. "remote:Downloads".

By default it will only examine file meta infos (file path & size).
If --check flag is set, it will also do the hash checking.`, constants.HELP_TORRENT_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: verifytorrent,
}

var (
	renameOk             = false
	renameFail           = false
	useComment           = false
	checkHash            = false
	checkQuick           = false
	forceLocal           = false
	showAll              = false
	contentPath          = ""
	defaultSite          = ""
	savePath             = ""
	rcloneLsjsonFilename = ""
	rcloneSavePath       = ""
	rcloneBinary         = ""
	rcloneFlags          = ""
)

func init() {
	command.Flags().BoolVarP(&renameOk, "rename-ok", "", false,
		"Rename verification successed .torrent file to *"+constants.FILENAME_SUFFIX_OK+
			" unless it's name already has that suffix")
	command.Flags().BoolVarP(&renameFail, "rename-fail", "", false,
		"Rename verification failed .torrent file to *"+constants.FILENAME_SUFFIX_FAIL+
			" unless it's name already has that suffix")
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Extract save path from 'comment' field of .torrent file and verify against it")
	command.Flags().BoolVarP(&checkHash, "check", "", false, "Do hash checking when verifying torrent files")
	command.Flags().BoolVarP(&checkQuick, "check-quick", "", false,
		"Do quick hash checking when verifying torrent files, "+
			"only the first and last piece of each file will do hash computing")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().StringVarP(&contentPath, "content-path", "", "",
		"The path of torrent content. Can only be used with single torrent arg")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&savePath, "save-path", "", "", `The "Downloads" folder, save path of torrent contents.`)
	command.Flags().StringVarP(&rcloneLsjsonFilename, "rclone-lsjson-file", "", "",
		`The "rclone lsjson --recursive <path>" output filename, ptool treats <path> as the save path of torrent contents`)
	command.Flags().StringVarP(&rcloneSavePath, "rclone-save-path", "", "",
		`The rclone save path of "Downloads" folder, ptool will execute "rclone lsjson --recursive <rclone-save-path>" `+
			`and read it's output. E.g. "remote:Downloads"`)
	command.Flags().StringVarP(&rcloneFlags, "rclone-flags", "", "",
		`Used with "--rclone-save-path", the additional rclone flags. E.g.: "--config rclone.conf"`)
	command.Flags().StringVarP(&rcloneBinary, "rclone-binary", "", "rclone",
		`Used with "--rclone-save-path", the path of rclone binary`)
	cmd.RootCmd.AddCommand(command)
}

func verifytorrent(cmd *cobra.Command, args []string) error {
	if util.CountNonZeroVariables(useComment, savePath, contentPath, rcloneSavePath, rcloneLsjsonFilename) != 1 {
		return fmt.Errorf("exact one (not less or more) of the --use-comment-meta, --save-path, --content-path, " +
			"--rclone-save-path and --rclone-lsjson-file flags must be set")
	}
	if rcloneSavePath != "" || rcloneLsjsonFilename != "" {
		if checkHash || checkQuick {
			return fmt.Errorf("--rclone-* can NOT be used with --check or --check-quick flags")
		}
	} else {
		if checkHash && checkQuick {
			return fmt.Errorf("--check and --check-quick flags are NOT compatible")
		}
	}
	torrents, stdinTorrentContents, err := helper.ParseTorrentsFromArgs(args)
	if err != nil {
		return err
	}
	if len(torrents) > 1 && contentPath != "" {
		return fmt.Errorf("--content-path flag can only be used to verify single torrent")
	}
	errorCnt := int64(0)
	checkMode := int64(0)
	checkModeStr := "none"
	if checkQuick {
		checkMode = 1
		checkModeStr = "quick"
	} else if checkHash {
		checkMode = 99
		checkModeStr = "full"
	}

	var rcloneSavePathFs fs.ReadDirFS
	if len(torrents) > 0 && (rcloneSavePath != "" || rcloneLsjsonFilename != "") {
		var rcloneLsjsonContents []byte
		var err error
		if rcloneSavePath != "" {
			args := []string{"lsjson", "--recursive"}
			if rcloneFlags != "" {
				if flags, err := shlex.Split(rcloneFlags); err != nil {
					return fmt.Errorf("failed to parse rclone flags: %v", err)
				} else {
					args = append(args, flags...)
				}
			}
			args = append(args, rcloneSavePath)
			fmt.Fprintf(os.Stderr, "Run %s with args %v\n", rcloneBinary, args)
			rcloneCmd := exec.Command(rcloneBinary, args...)
			rcloneLsjsonContents, err = rcloneCmd.Output()
		} else {
			rcloneLsjsonContents, err = os.ReadFile(rcloneLsjsonFilename)
		}
		if err != nil {
			return fmt.Errorf("failed to get rclone lsjson : %v", err)
		}
		rcloneSavePathFs, err = rclone.GetFsFromRcloneLsjsonResult(rcloneLsjsonContents)
		if err != nil {
			return fmt.Errorf("failed to parse rclone lsjson file: %v", err)
		}
	}

	for i, torrent := range torrents {
		if showAll && i > 0 {
			fmt.Printf("\n")
		}
		_, tinfo, _, _, _, _, isLocal, err :=
			helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, stdinTorrentContents, false, nil)
		if err != nil {
			fmt.Printf("X torrent %s: failed to get: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if showAll {
			tinfo.Print(torrent, true)
		}
		if useComment {
			if commentMeta := tinfo.DecodeComment(); commentMeta == nil {
				fmt.Printf("✕ %s : failed to parse comment meta\n", torrent)
				errorCnt++
				continue
			} else if commentMeta.SavePath == "" {
				fmt.Printf("✕ %s : comment meta has empty save_path\n", torrent)
				errorCnt++
				continue
			} else {
				log.Debugf("Found torrent %s comment meta %v", torrent, commentMeta)
				savePath = commentMeta.SavePath
			}
		}
		if rcloneSavePathFs != nil {
			log.Infof("Verifying %s against rclone lsjson output", torrent)
			err = tinfo.VerifyAgaintSavePathFs(rcloneSavePathFs)
		} else {
			log.Infof("Verifying %s (savepath=%s, contentpath=%s, checkhash=%t)", torrent, savePath, contentPath, checkHash)
			err = tinfo.Verify(savePath, contentPath, checkMode)
		}
		if err != nil {
			fmt.Printf("X torrent %s: contents do NOT match with disk content(s) (hash check = %s): %v\n",
				torrent, checkModeStr, err)
			errorCnt++
			if isLocal && torrent != "-" && renameFail && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_FAIL) {
				if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
					constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_FAIL); err != nil {
					log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_FAIL, err)
				}
			}
		} else {
			if isLocal && torrent != "-" && renameOk && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_OK) {
				if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
					constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_OK); err != nil {
					log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_OK, err)
				}
			}
			fmt.Printf("✓ torrent %s: contents match with disk content(s) (hash check = %s)\n", torrent, checkModeStr)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
