# GS-CLI

`gs-cli` is an interactive command-line interface written in Go to manage, inspect, and benchmark PathDB.

---

## Setup Guide

This section describes how to install dependencies, compile the dependencies, run the server, and launch the CLI.

### Step 1: Install Development Tools and Compilers

You will need **Java (JDK 18 or 21)**, **Maven**, **Go (1.23+)**, and a **C++ Compiler**.

#### On macOS:
1. **Install Xcode Command Line Tools**:
   ```bash
   xcode-select --install
   ```
2. **Install Homebrew** (if not installed):
   Follow instructions at [brew.sh](https://brew.sh).
3. **Install Compilers & Build Tools**:
   ```bash
   brew install go openjdk maven rocksdb
   ```
4. **Configure Java Environment**:
   Ensure Java is accessible by adding the following to your shell profile (e.g., `~/.zshrc` or `~/.bash_profile`):
   ```bash
   export JAVA_HOME="/opt/homebrew/opt/openjdk"
   export PATH="$JAVA_HOME/bin:$PATH"
   ```

#### On Ubuntu / Debian:
1. **Update package lists & install build essentials**:
   ```bash
   sudo apt-get update
   sudo apt-get install -y build-essential gcc g++ make git
   ```
2. **Install Java 21, Maven and Go**:
   ```bash
   sudo apt-get install -y openjdk-21-jdk maven golang-go
   ```
3. **Verify installations**:
   ```bash
   java -version
   mvn -version
   go version
   ```

---

### Step 2: Install System Dependencies for RocksDB

`gs-cli` relies on CGO bindings to talk to RocksDB directly when running in Local Mode. This requires C++ headers and libraries to be installed on your OS.

*   **macOS (Homebrew)**:
    Already installed in Step 1 via `brew install rocksdb`.
*   **Ubuntu / Debian**:
    Install the development packages for RocksDB and its compression libraries:
    ```bash
    sudo apt-get install -y librocksdb-dev libsnappy-dev zlib1g-dev libbz2-dev libgflags-dev liblz4-dev libzstd-dev
    ```

---

### Step 3: Clone, Build and Install `graph-storage`

`graph-storage` is a Java-based dependency required by the PathDB server. It must be compiled and installed into your local Maven repository.

1. **Clone the repository**:
   ```bash
   git clone https://github.com/dbgutalca/graph-storage.git
   cd graph-storage
   ```
2. **Compile and install**:
   ```bash
   mvn clean install
   ```
3. **Return to your workspace root**:
   ```bash
   cd ..
   ```

---

### Step 4: Clone, Build and Run `pathdb-server`

1. **Clone the repository**:
   ```bash
   git clone https://github.com/dbgutalca/pathdb-server.git
   cd pathdb-server
   ```
2. **Switch to the `storage` branch**:
   ```bash
   git checkout storage
   ```
3. **Run the server**:
   ```bash
   mvn spring-boot:run
   ```
   *The server will start up and listen on port `8080` by default.*
4. **Open a new terminal window** to run the CLI while keeping the server alive.

---

### Step 5: Clone, Compile and Run `gs-cli`

1. **Clone the repository**:
   ```bash
   git clone https://github.com/urzuaf/gs-cli.git
   cd gs-cli
   ```
2. **Compile the CLI binary**:
   *   **On macOS**:
       ```bash
       CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib" go build -o gs-cli ./cmd/gs-cli
       ```
   *   **On Linux (Ubuntu/Debian)**:
       ```bash
       go build -o gs-cli ./cmd/gs-cli
       ```
3. **Run the interactive CLI**:
   *   **On macOS**:
       ```bash
       CGO_CFLAGS="-I/opt/homebrew/include" CGO_LDFLAGS="-L/opt/homebrew/lib" ./gs-cli
       ```
   *   **On Linux (Ubuntu/Debian)**:
       ```bash
       ./gs-cli
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
