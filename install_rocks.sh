#!/bin/bash
sudo apt-get remove -y librocksdb-dev

sudo apt-get update
sudo apt-get install -y build-essential cmake git libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev

git clone --branch v10.10.1 https://github.com/facebook/rocksdb.git
cd rocksdb
mkdir build && cd build
cmake -DCMAKE_BUILD_TYPE=Release -DWITH_SNAPPY=1 -DWITH_LZ4=1 -DWITH_ZLIB=1 -DWITH_ZSTD=1 -DWITH_BZ2=1 -DWITH_GFLAGS=1 ..
make -j$(nproc)
sudo make install
sudo ldconfig