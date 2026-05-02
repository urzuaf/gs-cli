package pipeline

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/linxGnu/grocksdb"

	"gs-cli/internal/cache"
	"gs-cli/internal/metastore"
	"gs-cli/internal/parser"
	"gs-cli/internal/rocks"
	"gs-cli/internal/storage"
)

type IndexStrategy int

const (
	StrategyUnrolled IndexStrategy = iota
	StrategyRolledNaive
	StrategyRolledMerge
)

type BenchIngestor struct {
	db     *rocks.Store
	meta   *metastore.MetaStore
	cache  *cache.NodeCache
	dbPath string
}

func NewBenchIngestor(db *rocks.Store, meta *metastore.MetaStore, cache *cache.NodeCache, dbPath string) *BenchIngestor {
	return &BenchIngestor{
		db:     db,
		meta:   meta,
		cache:  cache,
		dbPath: dbPath,
	}
}

func (bi *BenchIngestor) Ingest(nodeFile, edgeFile string, strategy IndexStrategy) error {
	start := time.Now()

	strategyName := "Unrolled"
	if strategy == StrategyRolledNaive {
		strategyName = "Rolled-Naive (RMW)"
	} else if strategy == StrategyRolledMerge {
		strategyName = "Rolled-Merge"
	}

	log.Printf("Starting ingestion (strategy=%s)...\n", strategyName)

	if nodeFile != "" {
		if err := bi.ingestNodes(nodeFile, strategy); err != nil {
			return err
		}
	}

	if edgeFile != "" {
		if err := bi.ingestEdges(edgeFile, strategy); err != nil {
			return err
		}
	}

	elapsed := time.Since(start)

	log.Println("Waiting for compactions to get accurate size...")
	bi.db.DB.CompactRangeCF(bi.db.CFNodes, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFEdges, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFIdxNodeProp, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFIdxEdgeProp, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFIdxEdgeSrc, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFIdxEdgeDst, grocksdb.Range{})
	bi.db.DB.CompactRangeCF(bi.db.CFIdxLabel, grocksdb.Range{})

	size, err := bi.getDirSize(bi.dbPath)
	if err != nil {
		log.Printf("Warning: could not calculate DB size: %v\n", err)
	}

	fmt.Printf("\n--- Benchmark Results (%s) ---\n", strategyName)
	fmt.Printf("Total Time: %v\n", elapsed)
	fmt.Printf("DB Size:    %.2f MB\n", float64(size)/(1024*1024))
	fmt.Printf("-------------------------------------\n\n")

	return nil
}

func (bi *BenchIngestor) ingestNodes(filePath string, strategy IndexStrategy) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var currentHeader []string
	splitBuffer := make([]string, 0, 50)
	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	naiveBuffer := make(map[string][]byte)

	count := 0
	currentBatchBytes := 0

	flushNaive := func() error {
		for k, v := range naiveBuffer {
			idxKey := []byte(k)
			existing, _ := bi.db.DB.GetCF(bi.db.RO, bi.db.CFIdxNodeProp, idxKey)
			var finalVal []byte
			if existing.Exists() {
				finalVal = append(existing.Data(), v...)
			} else {
				finalVal = v
			}
			existing.Free()
			batch.PutCF(bi.db.CFIdxNodeProp, idxKey, finalVal)
		}
		clear(naiveBuffer)
		return bi.db.DB.Write(bi.db.WO, batch)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			continue
		}
		if currentHeader == nil {
			continue
		}

		rec := parser.ParseLine(line, currentHeader, splitBuffer)
		if rec.ID == "" || rec.Label == "" {
			continue
		}

		bi.cache.Put(rec.ID, rec.Label)

		nodeKey := storage.EncodeNodeKey(rec.ID)
		nodeVal := storage.EncodeNodeValue(rec.Label, rec.Props)
		batch.PutCF(bi.db.CFNodes, nodeKey, nodeVal)
		currentBatchBytes += len(nodeKey) + len(nodeVal)

		for k, v := range rec.Props {
			if v == "" {
				continue
			}

			idxKey := storage.IdxKey(k, storage.Norm(v))

			switch strategy {
			case StrategyUnrolled:
				idxKeyFull := storage.IdxKey(k, storage.Norm(v), rec.ID)
				batch.PutCF(bi.db.CFIdxNodeProp, idxKeyFull, []byte{})
				currentBatchBytes += len(idxKeyFull)

			case StrategyRolledMerge:
				// Blind merge
				batch.MergeCF(bi.db.CFIdxNodeProp, idxKey, []byte(rec.ID+","))
				currentBatchBytes += len(idxKey) + len(rec.ID) + 1

			case StrategyRolledNaive:
				// Blind append in local memory buffer
				keyStr := string(idxKey)
				naiveBuffer[keyStr] = append(naiveBuffer[keyStr], []byte(rec.ID+",")...)
				currentBatchBytes += len(idxKey) + len(rec.ID) + 1
			}
		}

		count++
		if count%LoggerBatch == 0 {
			log.Printf("Ingested %d nodes...\n", count)
		}

		if currentBatchBytes >= MaxBatchSizeBytes {
			if strategy == StrategyRolledNaive {
				if err := flushNaive(); err != nil {
					return err
				}
			} else {
				if err := bi.db.DB.Write(bi.db.WO, batch); err != nil {
					return err
				}
			}
			batch.Clear()
			currentBatchBytes = 0
		}
	}

	if strategy == StrategyRolledNaive {
		return flushNaive()
	} else if batch.Count() > 0 {
		return bi.db.DB.Write(bi.db.WO, batch)
	}
	return scanner.Err()
}

func (bi *BenchIngestor) ingestEdges(filePath string, strategy IndexStrategy) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	var currentHeader []string
	splitBuffer := make([]string, 0, 50)
	batch := grocksdb.NewWriteBatch()
	defer batch.Destroy()

	count := 0
	currentBatchBytes := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			currentHeader = parser.ParseHeader(line)
			continue
		}
		if currentHeader == nil {
			continue
		}

		rec := parser.ParseLine(line, currentHeader, splitBuffer)
		if rec.Label == "" || rec.Src == "" || rec.Dst == "" {
			continue
		}

		edgeID := rec.ID
		if edgeID == "" {
			edgeID = storage.MakeEdgeID(rec.Src, rec.Label, rec.Dst)
		}

		edgeKey := storage.EncodeEdgeKey(edgeID)
		edgeVal := storage.EncodeEdgeValue(rec.Label, rec.Src, rec.Dst, rec.Props)
		batch.PutCF(bi.db.CFEdges, edgeKey, edgeVal)
		currentBatchBytes += len(edgeKey) + len(edgeVal)

		// Label Index (Always unrolled for now)
		idxLabelKey := storage.IdxKey(rec.Label, edgeID)
		batch.PutCF(bi.db.CFIdxLabel, idxLabelKey, edgeVal)

		// Src/Dst Indexes
		if strategy == StrategyUnrolled {
			batch.PutCF(bi.db.CFIdxEdgeSrc, storage.IdxKey(rec.Src, edgeID), []byte{})
			batch.PutCF(bi.db.CFIdxEdgeDst, storage.IdxKey(rec.Dst, edgeID), []byte{})
		} else if strategy == StrategyRolledMerge {
			batch.MergeCF(bi.db.CFIdxEdgeSrc, storage.IdxKey(rec.Src), []byte(edgeID+","))
			batch.MergeCF(bi.db.CFIdxEdgeDst, storage.IdxKey(rec.Dst), []byte(edgeID+","))
		} else {
			// Naive RMW for Src/Dst
			if batch.Count() > 0 {
				bi.db.DB.Write(bi.db.WO, batch)
				batch.Clear()
			}
			bi.rmwAppend(bi.db.CFIdxEdgeSrc, storage.IdxKey(rec.Src), edgeID)
			bi.rmwAppend(bi.db.CFIdxEdgeDst, storage.IdxKey(rec.Dst), edgeID)
		}

		// Property Index
		for k, v := range rec.Props {
			if v == "" {
				continue
			}
			idxKey := storage.IdxKey(k, storage.Norm(v))
			if strategy == StrategyUnrolled {
				batch.PutCF(bi.db.CFIdxEdgeProp, storage.IdxKey(k, storage.Norm(v), edgeID), []byte{})
			} else if strategy == StrategyRolledMerge {
				batch.MergeCF(bi.db.CFIdxEdgeProp, idxKey, []byte(edgeID+","))
			} else {
				if batch.Count() > 0 {
					bi.db.DB.Write(bi.db.WO, batch)
					batch.Clear()
				}
				bi.rmwAppend(bi.db.CFIdxEdgeProp, idxKey, edgeID)
			}
		}

		count++
		if count%LoggerBatch == 0 {
			log.Printf("Ingested %d edges...\n", count)
		}

		if currentBatchBytes >= MaxBatchSizeBytes {
			if err := bi.db.DB.Write(bi.db.WO, batch); err != nil {
				return err
			}
			batch.Clear()
			currentBatchBytes = 0
		}
	}

	if batch.Count() > 0 {
		return bi.db.DB.Write(bi.db.WO, batch)
	}
	return scanner.Err()
}

func (bi *BenchIngestor) rmwAppend(cf *grocksdb.ColumnFamilyHandle, key []byte, id string) {
	existing, _ := bi.db.DB.GetCF(bi.db.RO, cf, key)
	var newVal []byte
	if existing.Exists() {
		newVal = append(existing.Data(), []byte(id+",")...)
	} else {
		newVal = []byte(id + ",")
	}
	existing.Free()
	bi.db.DB.PutCF(bi.db.WO, cf, key, newVal)
}

func (bi *BenchIngestor) QueryNodes(prop, val string, strategy IndexStrategy) error {
	start := time.Now()
	count := 0

	prop = storage.Norm(prop)
	val = storage.Norm(val)

	if strategy == StrategyUnrolled {
		prefix := storage.IdxKey(prop, val)
		prefix = append(prefix, 0)

		it := bi.db.DB.NewIteratorCF(bi.db.RO, bi.db.CFIdxNodeProp)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
	} else {
		// Key: prop|val
		key := storage.IdxKey(prop, val)
		slice, err := bi.db.DB.GetCF(bi.db.RO, bi.db.CFIdxNodeProp, key)
		if err != nil {
			return err
		}
		defer slice.Free()

		if slice.Exists() {
			data := string(slice.Data())
			ids := strings.Split(strings.TrimSuffix(data, ","), ",")
			count = len(ids)
		}
	}

	elapsed := time.Since(start)
	strategyName := bi.getStrategyName(strategy)

	fmt.Printf("\n--- Query Results (%s) ---\n", strategyName)
	fmt.Printf("Property: %s = %s\n", prop, val)
	fmt.Printf("IDs found: %d\n", count)
	fmt.Printf("Time:      %v\n", elapsed)
	fmt.Printf("---------------------------\n\n")

	return nil
}

func (bi *BenchIngestor) QueryEdgesBySrc(srcID string, strategy IndexStrategy) error {
	start := time.Now()
	count := 0

	if strategy == StrategyUnrolled {
		prefix := storage.IdxKey(srcID)
		prefix = append(prefix, 0)

		it := bi.db.DB.NewIteratorCF(bi.db.RO, bi.db.CFIdxEdgeSrc)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
	} else {
		key := storage.IdxKey(srcID)
		slice, err := bi.db.DB.GetCF(bi.db.RO, bi.db.CFIdxEdgeSrc, key)
		if err != nil {
			return err
		}
		defer slice.Free()

		if slice.Exists() {
			data := string(slice.Data())
			ids := strings.Split(strings.TrimSuffix(data, ","), ",")
			count = len(ids)
		}
	}

	elapsed := time.Since(start)
	strategyName := bi.getStrategyName(strategy)

	fmt.Printf("\n--- Edge Query Results (%s) ---\n", strategyName)
	fmt.Printf("Source ID: %s\n", srcID)
	fmt.Printf("Edges found: %d\n", count)
	fmt.Printf("Time:        %v\n", elapsed)
	fmt.Printf("-------------------------------\n\n")

	return nil
}

func (bi *BenchIngestor) getStrategyName(strategy IndexStrategy) string {
	switch strategy {
	case StrategyUnrolled:
		return "Unrolled"
	case StrategyRolledNaive:
		return "Rolled-Naive"
	case StrategyRolledMerge:
		return "Rolled-Merge"
	default:
		return "Unknown"
	}
}

func (bi *BenchIngestor) getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
