package pipeline

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"github.com/google/uuid"

	"github.com/linxGnu/grocksdb"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/parser"
	"gs-cli/internal/rocks"
	"gs-cli/internal/storage"
)

const MaxBatchSizeBytes = 16 * 1024 * 1024 // 16 MB
const TargetMegaBlobSizeBytes = 16 * 1024 * 1024 // 16 MB target for FatBatches
const LoggerBatch = 100000

type Ingestor struct {
	db        *rocks.Store
	meta      *metastore.MetaStore
	cache     *cache.NodeCache
	Verbosity int
}

func NewIngestor(db *rocks.Store, meta *metastore.MetaStore, cache *cache.NodeCache, verbosity int) *Ingestor {
	return &Ingestor{
		db:        db,
		meta:      meta,
		cache:     cache,
		Verbosity: verbosity,
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
	var count int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			if ig.Verbosity >= 3 {
				log.Printf("DEBUG: New header detected: %v\n", currentHeader)
			}
			continue
		}

		if currentHeader == nil {
			continue // ignore data without header
		}

		rec := parser.ParseLine(line, currentHeader, splitBuffer)
		if rec.ID == "" || rec.Label == "" {
			continue
		}

		if ig.Verbosity >= 1 {
			count++
			if count%LoggerBatch == 0 {
				log.Printf("INFO: Ingested %d nodes...\n", count)
			}
		}

		// serialize
		nodeKey := storage.EncodeNodeKey(rec.ID)
		nodeVal := storage.EncodeNodeValue(rec.Label, rec.Props)

		// save in cache
		ig.cache.Put(rec.ID, nodeVal)

		if ig.Verbosity >= 3 {
			log.Printf("DEBUG: Node record: ID=%s, Label=%s, Props=%v\n", rec.ID, rec.Label, rec.Props)
			log.Printf("DEBUG: Binary Key:   %s\n", hex.EncodeToString(nodeKey))
			log.Printf("DEBUG: Binary Value: %s\n", hex.EncodeToString(nodeVal))
		}

		// add batch to rocks
		batch.PutCF(ig.db.CFNodes, nodeKey, nodeVal)
		currentBatchBytes += len(nodeKey) + len(nodeVal)

		// props secondary index
		for k, v := range rec.Props {
			if v == "" {
				continue
			}
			idxKey := storage.IdxKey(k, storage.Norm(v), rec.ID)
			batch.PutCF(ig.db.CFIdxNodeProp, idxKey, []byte{})
			currentBatchBytes += len(idxKey)
			if ig.Verbosity >= 3 {
				log.Printf("DEBUG:   Prop Index Key [%s=%s]: %s\n", k, v, hex.EncodeToString(idxKey))
			}
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
	var count int

	// Fat Indexing structures
	labelToPendingMegaBlobs := make(map[string][][]byte)
	labelToPendingSize := make(map[string]int)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			if ig.Verbosity >= 3 {
				log.Printf("DEBUG: New header detected: %v\n", currentHeader)
			}
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

		// log progress (Level 1+)
		if ig.Verbosity >= 1 {
			count++
			if count%LoggerBatch == 0 {
				log.Printf("INFO: Ingested %d edges...\n", count)
			}
		}

		// generate if no ID
		edgeID := rec.ID
		if edgeID == "" {
			edgeID = storage.MakeEdgeID(rec.Src, rec.Label, rec.Dst)
		}

		// serialize
		edgeKey := storage.EncodeEdgeKey(edgeID)
		edgeVal := storage.EncodeEdgeValue(edgeID, rec.Label, rec.Src, rec.Dst, rec.Props)

		if ig.Verbosity >= 3 {
			log.Printf("DEBUG: Edge record: ID=%s, Label=%s, Src=%s, Dst=%s, Props=%v\n", edgeID, rec.Label, rec.Src, rec.Dst, rec.Props)
			log.Printf("DEBUG: Binary Key:   %s\n", hex.EncodeToString(edgeKey))
			log.Printf("DEBUG: Binary Value: %s\n", hex.EncodeToString(edgeVal))
		}

		batch.PutCF(ig.db.CFEdges, edgeKey, edgeVal)
		currentBatchBytes += len(edgeKey) + len(edgeVal)

		// fat indexing: group blobs in batches by target byte size per label
		srcB, _ := ig.cache.Get(rec.Src)
		dstB, _ := ig.cache.Get(rec.Dst)
		megaBlob := storage.EncodeMegaBlob(edgeVal, srcB, dstB)

		pending := labelToPendingMegaBlobs[rec.Label]
		currentSize, ok := labelToPendingSize[rec.Label]
		if !ok {
			currentSize = 4 // 4 bytes for count in FatBatch
		}

		pending = append(pending, megaBlob)
		currentSize += 4 + len(megaBlob) // 4 bytes for length prefix per megaBlob

		if currentSize >= TargetMegaBlobSizeBytes {
			fatBatchKey := []byte(rec.Label + ":" + uuid.New().String())
			batch.PutCF(ig.db.CFIdxLabel, fatBatchKey, storage.EncodeFatBatch(pending))
			pending = nil
			currentSize = 4
		}
		labelToPendingMegaBlobs[rec.Label] = pending
		labelToPendingSize[rec.Label] = currentSize

		// 2. Src index
		idxSrcKey := storage.IdxKey(rec.Src, edgeID)
		batch.PutCF(ig.db.CFIdxEdgeSrc, idxSrcKey, []byte{})
		currentBatchBytes += len(idxSrcKey)

		// 3. Dst index
		idxDstKey := storage.IdxKey(rec.Dst, edgeID)
		batch.PutCF(ig.db.CFIdxEdgeDst, idxDstKey, []byte{})
		currentBatchBytes += len(idxDstKey)

		// 4. Property index
		for k, v := range rec.Props {
			if v == "" {
				continue
			}
			idxKey := storage.IdxKey(k, storage.Norm(v), edgeID)
			batch.PutCF(ig.db.CFIdxEdgeProp, idxKey, []byte{})
			currentBatchBytes += len(idxKey)
			if ig.Verbosity >= 3 {
				log.Printf("DEBUG:   Prop Index Key [%s=%s]: %s\n", k, v, hex.EncodeToString(idxKey))
			}
		}

		// update metadata (note: src/dst labels are not available here easily without decoding cache)
		propKeys := make([]string, 0, len(rec.Props))
		for k := range rec.Props {
			propKeys = append(propKeys, k)
		}
		ig.meta.IncEdge(rec.Label, "", "", propKeys) // Labels are left empty as per current MetaStore use

		// Flush batch
		if currentBatchBytes >= MaxBatchSizeBytes {
			if err := ig.db.DB.Write(ig.db.WO, batch); err != nil {
				return fmt.Errorf("error writing edges: %w", err)
			}
			batch.Clear()
			currentBatchBytes = 0
		}
	}

	// Flush remaining Fat Batches
	for label, pending := range labelToPendingMegaBlobs {
		if len(pending) > 0 {
			fatBatchKey := []byte(label + ":" + uuid.New().String())
			batch.PutCF(ig.db.CFIdxLabel, fatBatchKey, storage.EncodeFatBatch(pending))
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
