package common

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

type TorrentType int

const (
	TORRENT_SUCCESS TorrentType = iota
	TORRENT_FAILURE
	TORRENT_INVALID
)

type TorrentsStatistics struct {
	TorrentsCnt         int64 // number of valid .torrent files
	SuccessCnt          int64
	SuccessSize         int64
	SuccessContentFiles int64
	FailureCnt          int64
	FailureSize         int64
	InvalidCnt          int64
	SmallestSize        int64
	LargestSize         int64
}

func NewTorrentsStatistics() *TorrentsStatistics {
	return &TorrentsStatistics{
		SmallestSize: -1,
		LargestSize:  -1,
	}
}

func (ts *TorrentsStatistics) Update(torrentType TorrentType, size int64, files int64) {
	switch torrentType {
	case TORRENT_SUCCESS:
		ts.TorrentsCnt++
		ts.SuccessCnt++
		ts.SuccessContentFiles += files
		ts.SuccessSize += size
		if ts.LargestSize == -1 || size > ts.LargestSize {
			ts.LargestSize = size
		}
		if ts.SmallestSize == -1 || size < ts.SmallestSize {
			ts.SmallestSize = size
		}
	case TORRENT_FAILURE:
		ts.TorrentsCnt++
		ts.FailureCnt++
		ts.FailureSize += size
	case TORRENT_INVALID:
		ts.InvalidCnt++
	}
}

func (ts *TorrentsStatistics) UpdateClientTorrent(torrentType TorrentType, torrent *client.Torrent) {
	ts.Update(torrentType, torrent.Size, 0)
}

func (ts *TorrentsStatistics) UpdateTinfo(torrentType TorrentType, tinfo *torrentutil.TorrentMeta) {
	if tinfo != nil {
		ts.Update(torrentType, tinfo.Size, int64(len(tinfo.Files)))
	} else {
		ts.Update(torrentType, 0, 0)
	}
}

func (ts *TorrentsStatistics) Print(output io.Writer) {
	averageSize := int64(0)
	if ts.SuccessCnt > 0 {
		averageSize = ts.SuccessSize / ts.SuccessCnt
	}
	fmt.Fprintf(output, "Success torrents: %d\n", ts.TorrentsCnt)
	fmt.Fprintf(output, "Total contents size: %s (%d Byte)\n", util.BytesSize(float64(ts.SuccessSize)), ts.SuccessSize)
	fmt.Fprintf(output, "Total number of content files: %d\n", ts.SuccessContentFiles)
	fmt.Fprintf(output, "Smallest / Average / Largest torrent contents size: %s / %s / %s\n",
		util.BytesSize(float64(ts.SmallestSize)), util.BytesSize(float64(averageSize)),
		util.BytesSize(float64(ts.LargestSize)))
	fmt.Fprintf(output, "Failure torrents: %d (%s)\n", ts.FailureCnt, util.BytesSize(float64(ts.FailureSize)))
	fmt.Fprintf(output, "Invalid torrents: %d\n", ts.InvalidCnt)
}

type PathMapper struct {
	mapper  map[string]string
	befores []string
}

func (spm *PathMapper) Before2After(beforePath string) (afterPath string, match bool) {
	beforePath = path.Clean(filepath.ToSlash(beforePath))
	for _, before := range spm.befores {
		if before == "/" {
			if strings.HasPrefix(beforePath, before) {
				return spm.mapper[before] + strings.TrimPrefix(beforePath, before), true
			}
		} else if strings.HasPrefix(beforePath, before+"/") {
			return spm.mapper[before] + strings.TrimPrefix(beforePath, before), true
		}
	}
	return beforePath, false
}

func (spm *PathMapper) After2Before(afterPath string) (beforePath string, match bool) {
	afterPath = path.Clean(filepath.ToSlash(afterPath))
	for _, before := range spm.befores {
		after := spm.mapper[before]
		if after == "/" {
			if strings.HasPrefix(afterPath, after) {
				return before + strings.TrimPrefix(afterPath, after), true
			}
		} else if strings.HasPrefix(afterPath, after+"/") {
			return before + strings.TrimPrefix(afterPath, after), true
		}
	}
	return afterPath, false
}

func NewPathMapper(rules []string) (*PathMapper, error) {
	pm := &PathMapper{
		mapper: map[string]string{},
	}
	for _, rule := range rules {
		before, after, found := strings.Cut(rule, ":")
		if !found || before == "" || after == "" {
			return nil, fmt.Errorf("invalid path mapper rule %q", rule)
		}
		before = path.Clean(filepath.ToSlash(before))
		after = path.Clean(filepath.ToSlash(after))
		pm.mapper[before] = after
		pm.befores = append(pm.befores, before)
	}
	slices.SortFunc(pm.befores, func(a, b string) int { return len(b) - len(a) }) // longest first
	return pm, nil
}
