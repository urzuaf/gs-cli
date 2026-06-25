# GS-CLI

`gs-cli` is an interactive command-line interface written in Go to manage and query PathDB.

## Prerequisites

To compile and run `gs-cli`, you need the following dependencies installed on your system:

1. **Go Compiler**: Version 1.23 or higher.
2. **RocksDB C++ Library**: Required for the Go `grocksdb` CGO bindings.
   *   **On macOS (Homebrew)**:
       ```bash
       brew install rocksdb
       ```
   *   **On Ubuntu/Debian**:
       ```bash
       sudo apt-get install librocksdb-dev
       ```

---

## Running the CLI
To run the interactive shell, execute:
```bash
CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib" go run ./cmd/gs-cli
```

---

## Command Reference

### Global Commands
Available in any session state to navigate or switch modes:

*   **`\connect <path>` or `\connect <port>`**: Switch session mode.
    *   *Local Mode*: Pass a directory path (e.g., `\connect ./databases`) to manage local databases on disk.
    *   *Remote Mode*: Pass a port number (e.g., `\connect 8080` or `localhost:8080`) to connect to a running PathDB server.
*   **`\l`**: List available databases.
    *   In *Local Mode*, lists directories in the configured root folder.
    *   In *Remote Mode*, queries the active databases registered on the server.
*   **`\c <dbname>`**: Select the active database.
    *   In *Local Mode*, targets the database for ingestion or stats.
    *   In *Remote Mode*, requests the server to mount the database in RAM or Disk.
*   **`\q`**: Quit the interactive shell session.

---

### Local Mode Commands
Only available in Local Mode (`\connect <path>`):

*   **`\create <dbname>`**: Create a new empty database directory on disk.
*   **`\drop <dbname>`**: Delete the selected database directory from disk.
*   **`\validate -n <nodes.pgdf> -e <edges.pgdf>`**: Validate PGDF syntax and referential integrity before ingestion.
*   **`\i -n <nodes.pgdf> -e <edges.pgdf> [--yolo]`**: Ingest PGDF files directly into RocksDB.
    *   Validates files first by default.
    *   Add `--yolo` or `-yolo` to skip integrity validation for faster ingestion.
*   **`\s`**: Open the local RocksDB store and display physical stats (LSM levels, memtables, index/filter memory).
*   **`\d`**: Read local metadata and display the node/edge counts, labels, properties, and connection schemas.

---

### Remote Mode Commands
Only available in Remote Mode (`\connect <port>`):

*   **`[Query Text]`**: Any plain text query (not starting with `\`, e.g., `MATCH...`) is sent to the server for execution. Displays execution time, Peak RAM, and total paths.
*   **`\results`**: Toggle detailed path output visibility (ON/OFF). Disabled by default to prevent terminal flooding.
*   **`\d`**: Fetch and display database schema and metadata from the active remote database.
*   **`\benchmark <filepath> [options]`**: Read queries from a file and execute them sequentially. Generates `raw_results_<name>.txt` and `final_results_<name>.txt`.
    *   `--fast`: Run only 1 iteration per query (defaults to 5).
    *   `--cold`: Restart the server before each query to measure performance without cache.
    *   `--restart-cmd "<cmd>"`: Shell command to restart the server (required for `--cold`).
    *   `--name <name>`: Prefix for output files.
