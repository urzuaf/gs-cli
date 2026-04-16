package rocks

import (
	"runtime"

	"github.com/linxGnu/grocksdb"
)

const (
	CFDefault     = "default"
	CFNodes       = "cf_nodes"
	CFEdges       = "cf_edges"
	CFIdxNodeProp = "cf_idx_node_prop"
	CFIdxEdgeProp = "cf_idx_edge_prop"
	CFIdxEdgeSrc  = "cf_idx_edge_src"
	CFIdxEdgeDst  = "cf_idx_edge_dst"
	CFIdxLabel    = "cf_idx_label"
)

type Store struct {
	DB *grocksdb.DB

	CFNodes       *grocksdb.ColumnFamilyHandle
	CFEdges       *grocksdb.ColumnFamilyHandle
	CFIdxNodeProp *grocksdb.ColumnFamilyHandle
	CFIdxEdgeProp *grocksdb.ColumnFamilyHandle
	CFIdxEdgeSrc  *grocksdb.ColumnFamilyHandle
	CFIdxEdgeDst  *grocksdb.ColumnFamilyHandle
	CFIdxLabel    *grocksdb.ColumnFamilyHandle

	WO *grocksdb.WriteOptions
	RO *grocksdb.ReadOptions
}

func Open(dbPath string, readOnly bool) (*Store, error) {
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
	dbOpts.SetCreateIfMissing(!readOnly)               //
	dbOpts.SetCreateIfMissingColumnFamilies(!readOnly) //
	dbOpts.SetMaxBackgroundJobs(runtime.NumCPU())      //

	cfNames := []string{CFDefault, CFNodes, CFEdges, CFIdxNodeProp, CFIdxEdgeProp, CFIdxEdgeSrc, CFIdxEdgeDst, CFIdxLabel}
	cfOptions := make([]*grocksdb.Options, len(cfNames))
	for i := range cfOptions {
		cfOptions[i] = cfOpts
	}

	var db *grocksdb.DB
	var handles []*grocksdb.ColumnFamilyHandle
	var err error

	if readOnly {
		db, handles, err = grocksdb.OpenDbForReadOnlyColumnFamilies(dbOpts, dbPath, cfNames, cfOptions, false)
	} else {
		db, handles, err = grocksdb.OpenDbColumnFamilies(dbOpts, dbPath, cfNames, cfOptions)
	}

	if err != nil {
		return nil, err
	}

	wo := grocksdb.NewDefaultWriteOptions()
	wo.DisableWAL(true) //
	wo.SetSync(false)   //

	ro := grocksdb.NewDefaultReadOptions()

	return &Store{
		DB:            db,
		CFNodes:       handles[1],
		CFEdges:       handles[2],
		CFIdxNodeProp: handles[3],
		CFIdxEdgeProp: handles[4],
		CFIdxEdgeSrc:  handles[5],
		CFIdxEdgeDst:  handles[6],
		CFIdxLabel:    handles[7],
		WO:            wo,
		RO:            ro,
	}, nil
}

// GetProperty returns a property from the DB or a specific CF
func (s *Store) GetProperty(propName string, cf *grocksdb.ColumnFamilyHandle) string {
	if cf != nil {
		return s.DB.GetPropertyCF(propName, cf)
	}
	return s.DB.GetProperty(propName)
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
	if s.CFIdxNodeProp != nil {
		s.CFIdxNodeProp.Destroy()
	}
	if s.CFIdxEdgeProp != nil {
		s.CFIdxEdgeProp.Destroy()
	}
	if s.CFIdxEdgeSrc != nil {
		s.CFIdxEdgeSrc.Destroy()
	}
	if s.CFIdxEdgeDst != nil {
		s.CFIdxEdgeDst.Destroy()
	}
	if s.CFIdxLabel != nil {
		s.CFIdxLabel.Destroy()
	}
	if s.DB != nil {
		s.DB.Close()
	}
}
