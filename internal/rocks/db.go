package rocks

import (
	"runtime"

	"github.com/linxGnu/grocksdb"
)

const (
	CFDefault = "default"
	CFNodes   = "cf_nodes"
	CFEdges   = "cf_edges"
	CFIndex   = "cf_index"
)

type Store struct {
	DB *grocksdb.DB

	CFNodes *grocksdb.ColumnFamilyHandle
	CFEdges *grocksdb.ColumnFamilyHandle
	CFIndex *grocksdb.ColumnFamilyHandle

	WO *grocksdb.WriteOptions
	RO *grocksdb.ReadOptions
}

func Open(dbPath string) (*Store, error) {
	bbto := grocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetCacheIndexAndFilterBlocks(true) //

	// cf options
	cfOpts := grocksdb.NewDefaultOptions()
	cfOpts.SetBlockBasedTableFactory(bbto)
	cfOpts.SetCompression(grocksdb.LZ4Compression)           //
	cfOpts.SetBottommostCompression(grocksdb.LZ4Compression) //
	cfOpts.SetWriteBufferSize(128 * 1024 * 1024)             // 128MB per memtable
	cfOpts.SetMaxWriteBufferNumber(4)                        //
	cfOpts.SetMinWriteBufferNumberToMerge(2)                 //

	// general options
	dbOpts := grocksdb.NewDefaultOptions()
	dbOpts.SetCreateIfMissing(true)               //
	dbOpts.SetCreateIfMissingColumnFamilies(true) //
	dbOpts.SetMaxBackgroundJobs(runtime.NumCPU()) //

	cfNames := []string{CFDefault, CFNodes, CFEdges, CFIndex}
	cfOptions := []*grocksdb.Options{cfOpts, cfOpts, cfOpts, cfOpts}

	// open the db
	db, handles, err := grocksdb.OpenDbColumnFamilies(dbOpts, dbPath, cfNames, cfOptions)
	if err != nil {
		return nil, err
	}

	wo := grocksdb.NewDefaultWriteOptions()
	wo.DisableWAL(true) //
	wo.SetSync(false)   //

	ro := grocksdb.NewDefaultReadOptions()

	return &Store{
		DB:      db,
		CFNodes: handles[1],
		CFEdges: handles[2],
		CFIndex: handles[3],
		WO:      wo,
		RO:      ro,
	}, nil
}

// safely release the memory
func (s *Store) Close() {
	if s.WO != nil {
		s.WO.Destroy()
	}
	if s.RO != nil {
		s.RO.Destroy()
	}
	if s.CFNodes != nil {
		s.CFNodes.Destroy()
	}
	if s.CFEdges != nil {
		s.CFEdges.Destroy()
	}
	if s.CFIndex != nil {
		s.CFIndex.Destroy()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}
