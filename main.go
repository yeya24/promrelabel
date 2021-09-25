// This file is mostly copied from Thanos.

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/oklog/ulid"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/pkg/relabel"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	tsdb_errors "github.com/prometheus/prometheus/tsdb/errors"
	"github.com/prometheus/prometheus/tsdb/fileutil"
	"github.com/thanos-io/thanos/pkg/block"
	"github.com/thanos-io/thanos/pkg/compactv2"
	"github.com/thanos-io/thanos/pkg/runutil"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

const metaFilename = "meta.json"
const metaVersion1 = 1
const defaultDBPath = "data/"
const tmpForCreationBlockDirSuffix = ".tmp-for-creation"

var (
	relabelConfig string
	blockIDs      []string
	dbPath        string
	dryRun        bool
	deleteSource  bool
)

func main() {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	if err := run(logger); err != nil {
		level.Error(logger).Log("msg", "failed to run promrelabel", "error", err)
		os.Exit(1)
	}
}

func run(logger log.Logger) error {
	app := kingpin.New(filepath.Base(os.Args[0]), "A tool to relabel Prometheus TSDB blocks")
	app.HelpFlag.Short('h')

	app.Flag("relabel-config", "Relabel configuration file path. For the file format, please refer to https://github.com/yeya24/promrelabel/blob/master/relabel.yaml and https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config.").StringVar(&relabelConfig)
	app.Flag("id", "Block IDs to apply relabeling. (Repeated flag)").StringsVar(&blockIDs)
	app.Flag("dry-run", "Whether to enable dry run mode or not. Default is true.").Default("true").BoolVar(&dryRun)
	app.Flag("delete-source-block", "Whether to delete source block or not after relabeling. Default is false.").Default("false").BoolVar(&deleteSource)
	app.Arg("db path", "Database path (default is "+defaultDBPath+").").Default(defaultDBPath).StringVar(&dbPath)

	kingpin.MustParse(app.Parse(os.Args[1:]))

	content, err := ioutil.ReadFile(relabelConfig)
	if err != nil {
		return err
	}
	var relabels []*relabel.Config
	if err := yaml.Unmarshal(content, &relabels); err != nil {
		return errors.Wrap(err, "parsing relabel configuration")
	}

	var ids []ulid.ULID
	for _, id := range blockIDs {
		u, err := ulid.Parse(id)
		if err != nil {
		}
		ids = append(ids, u)
	}

	var changeLog compactv2.ChangeLogger
	chunkPool := chunkenc.NewPool()
	for _, id := range ids {
		id := id
		blockDir := filepath.Join(dbPath, id.String())
		b, err := tsdb.OpenBlock(logger, blockDir, chunkPool)
		if err != nil {
			return errors.Wrapf(err, "open block %v", id)
		}

		meta := b.Meta()
		p := compactv2.NewProgressLogger(logger, int(b.Meta().Stats.NumSeries))
		newID := ulid.MustNew(ulid.Now(), rand.Reader)
		meta.ULID = newID
		meta.Compaction.Sources = []ulid.ULID{id}

		newBlockDir := filepath.Join(dbPath, newID.String())
		if err := os.MkdirAll(newBlockDir, os.ModePerm); err != nil {
			return err
		}

		f, err := os.OpenFile(filepath.Join(newBlockDir, "change.log"), os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
		defer runutil.CloseWithLogOnErr(logger, f, "close changelog")

		changeLog = compactv2.NewChangeLog(f)
		level.Info(logger).Log("msg", "changelog will be available", "file", filepath.Join(newBlockDir, "change.log"))

		d, err := block.NewDiskWriter(context.Background(), logger, newBlockDir)
		if err != nil {
			return err
		}

		var comp *compactv2.Compactor
		if dryRun {
			comp = compactv2.NewDryRun("", logger, changeLog, chunkPool)
		} else {
			comp = compactv2.New("", logger, changeLog, chunkPool)
		}

		level.Info(logger).Log("msg", "starting rewrite for block", "source", id, "new", newID, "toRelabel", string(content))
		if err := comp.WriteSeries(context.Background(), []block.Reader{b}, d, p, compactv2.WithRelabelModifier(relabels...)); err != nil {
			return errors.Wrapf(err, "writing series from %v to %v", id, newID)
		}

		if dryRun {
			level.Info(logger).Log("msg", "dry run finished. Changes should be printed to stderr", "source", id)
			continue
		}

		level.Info(logger).Log("msg", "wrote new block after modifications; flushing", "source", id, "new", newID)
		meta.Stats, err = d.Flush()
		if err != nil {
			return errors.Wrap(err, "flush")
		}
		if err := os.Remove(newBlockDir + tmpForCreationBlockDirSuffix); err != nil {
			return errors.Wrap(err, "remove tmp dir")
		}

		if _, err := writeMetaFile(logger, newBlockDir, &meta); err != nil {
			return err
		}

		level.Info(logger).Log("msg", "created new block", "source", id, "new", newID)

		if deleteSource {
			if err := os.RemoveAll(blockDir); err != nil {
				return errors.Wrapf(err, "delete source block after relabeling:%s", id)
			}
		}
	}
	return nil
}

func writeMetaFile(logger log.Logger, dir string, meta *tsdb.BlockMeta) (int64, error) {
	meta.Version = metaVersion1

	// Make any changes to the file appear atomic.
	path := filepath.Join(dir, metaFilename)
	tmp := path + ".tmp"
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			level.Error(logger).Log("msg", "remove tmp file", "err", err.Error())
		}
	}()

	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}

	jsonMeta, err := json.MarshalIndent(meta, "", "\t")
	if err != nil {
		return 0, err
	}

	n, err := f.Write(jsonMeta)
	if err != nil {
		return 0, tsdb_errors.NewMulti(err, f.Close()).Err()
	}

	// Force the kernel to persist the file on disk to avoid data loss if the host crashes.
	if err := f.Sync(); err != nil {
		return 0, tsdb_errors.NewMulti(err, f.Close()).Err()
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return int64(n), fileutil.Replace(tmp, path)
}
