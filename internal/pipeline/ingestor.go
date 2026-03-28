package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/linxGnu/grocksdb"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/parser"
	"gs-cli/internal/rocks"
	"gs-cli/internal/storage"
)

const MaxBatchSizeBytes = 16 * 1024 * 1024 // 16 MB

type Ingestor struct {
	db    *rocks.Store
	meta  *metastore.MetaStore
	cache *cache.NodeCache
}

func NewIngestor(db *rocks.Store, meta *metastore.MetaStore, cache *cache.NodeCache) *Ingestor {
	return &Ingestor{
		db:    db,
		meta:  meta,
		cache: cache,
	}
}

func (ig *Ingestor) IngestNodes(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening nodes file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var currentHeader []string
	splitBuffer := make([]string, 0, 50)

	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	var currentBatchBytes int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			continue
		}

		if currentHeader == nil {
			continue // ignore data without header
		}

		rec := parser.ParseLine(line, currentHeader, splitBuffer)
		if rec.ID == "" || rec.Label == "" {
			continue
		}

		// save in cache
		ig.cache.Put(rec.ID, rec.Label)

		// serialize
		nodeKey := storage.EncodeNodeKey(rec.ID)
		nodeVal := storage.EncodeNodeValue(rec.Label, rec.Props)

		// add batch to rocks
		batch.PutCF(ig.db.CFNodes, nodeKey, nodeVal)
		currentBatchBytes += len(nodeKey) + len(nodeVal)

		// props secondary index
		for k, v := range rec.Props {
			if v == "" {
				continue
			}
			idxKey := storage.IdxKey("prop", k, storage.Norm(v), rec.ID)
			batch.PutCF(ig.db.CFIndex, idxKey, []byte{})
			currentBatchBytes += len(idxKey)
		}

		// update metadata
		propKeys := make([]string, 0, len(rec.Props))
		for k := range rec.Props {
			propKeys = append(propKeys, k)
		}
		ig.meta.IncNode(rec.Label, propKeys)

		// flush if the batch is big enough
		if currentBatchBytes >= MaxBatchSizeBytes {
			if err := ig.db.DB.Write(ig.db.WO, batch); err != nil {
				return fmt.Errorf("error writing nodes: %w", err)
			}
			batch.Clear()
			currentBatchBytes = 0
		}
	}

	// final flush
	if batch.Count() > 0 {
		if err := ig.db.DB.Write(ig.db.WO, batch); err != nil {
			return fmt.Errorf("error during final nodes flush: %w", err)
		}
	}

	return scanner.Err()
}

func (ig *Ingestor) IngestEdges(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening edges file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var currentHeader []string
	splitBuffer := make([]string, 0, 50)

	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	var currentBatchBytes int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			continue
		}

		if currentHeader == nil {
			continue // ignore data without header
		}

		rec := parser.ParseLine(line, currentHeader, splitBuffer)

		if rec.Label == "" || rec.Src == "" || rec.Dst == "" {
			continue
		}
		if !strings.EqualFold(rec.Dir, "T") {
			continue
		}

		// generate if no ID
		edgeID := rec.ID
		if edgeID == "" {
			edgeID = storage.MakeEdgeID(rec.Src, rec.Label, rec.Dst)
		}

		// serialize
		edgeKey := storage.EncodeEdgeKey(edgeID)
		edgeVal := storage.EncodeEdgeValue(rec.Label, rec.Src, rec.Dst, rec.Props)

		batch.PutCF(ig.db.CFEdges, edgeKey, edgeVal)
		currentBatchBytes += len(edgeKey) + len(edgeVal)

		// covering index by label
		idxLabelKey := storage.IdxKey("label", "edge", rec.Label, edgeID)
		batch.PutCF(ig.db.CFIndex, idxLabelKey, edgeVal)
		currentBatchBytes += len(idxLabelKey)

		// src index
		idxSrcKey := storage.IdxKey("edgesBySrc", rec.Src, edgeID)
		batch.PutCF(ig.db.CFIndex, idxSrcKey, []byte{})
		currentBatchBytes += len(idxSrcKey)

		// dst index
		idxDstKey := storage.IdxKey("edgesByDst", rec.Dst, edgeID)
		batch.PutCF(ig.db.CFIndex, idxDstKey, []byte{})
		currentBatchBytes += len(idxDstKey)

		// property index
		for k, v := range rec.Props {
			if v == "" {
				continue
			}
			idxKey := storage.IdxKey("propEdge", k, storage.Norm(v), edgeID)
			batch.PutCF(ig.db.CFIndex, idxKey, []byte{})
			currentBatchBytes += len(idxKey)
		}

		// save metadata using cache
		srcLabel, _ := ig.cache.Get(rec.Src)
		dstLabel, _ := ig.cache.Get(rec.Dst)

		propKeys := make([]string, 0, len(rec.Props))
		for k := range rec.Props {
			propKeys = append(propKeys, k)
		}

		ig.meta.IncEdge(rec.Label, srcLabel, dstLabel, propKeys)

		// Flush batch
		if currentBatchBytes >= MaxBatchSizeBytes {
			if err := ig.db.DB.Write(ig.db.WO, batch); err != nil {
				return fmt.Errorf("error writing edges: %w", err)
			}
			batch.Clear()
			currentBatchBytes = 0
		}
	}

	// final flush
	if batch.Count() > 0 {
		if err := ig.db.DB.Write(ig.db.WO, batch); err != nil {
			return fmt.Errorf("error during final edges flush: %w", err)
		}
	}

	return scanner.Err()
}
