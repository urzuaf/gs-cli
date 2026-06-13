#!/bin/bash

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

echo "Iniciando secuencia de Benchmarks (Cold Run)..."
echo "=========================================================="

# 1. Benchmark sf01
echo ">>> [1/4] Ejecutando Scale Factor: sf01"
./gs-cli/gs-cli -e "\connect 8080; \c sf01; \benchmark gs-cli/consultas165 --cold --restart-cmd \"$SCRIPT_DIR/restart_server.sh\" --name disk_sf01"
echo ">>> [1/4] Finalizado. Resultados en raw_results_disk_sf01.txt y final_results_disk_sf01.txt"
echo "----------------------------------------------------------"

# 2. Benchmark sf03
echo ">>> [2/4] Ejecutando Scale Factor: sf03"
./gs-cli/gs-cli -e "\connect 8080; \c sf03; \benchmark gs-cli/consultas165 --cold --restart-cmd \"$SCRIPT_DIR/restart_server.sh\" --name disk_sf03"
echo ">>> [2/4] Finalizado. Resultados en raw_results_disk_sf03.txt y final_results_disk_sf03.txt"
echo "----------------------------------------------------------"

# 3. Benchmark sf1
echo ">>> [3/4] Ejecutando Scale Factor: sf1"
./gs-cli/gs-cli -e "\connect 8080; \c sf1; \benchmark gs-cli/consultas165 --cold --restart-cmd \"$SCRIPT_DIR/restart_server.sh\" --name disk_sf1"
echo ">>> [3/4] Finalizado. Resultados en raw_results_disk_sf1.txt y final_results_disk_sf1.txt"
echo "----------------------------------------------------------"

# 4. Benchmark sf3
echo ">>> [4/4] Ejecutando Scale Factor: sf3"
./gs-cli/gs-cli -e "\connect 8080; \c sf3; \benchmark gs-cli/consultas165 --cold --restart-cmd \"$SCRIPT_DIR/restart_server.sh\" --name disk_sf3"
echo ">>> [4/4] Finalizado. Resultados en raw_results_disk_sf3.txt y final_results_disk_sf3.txt"
echo "=========================================================="
echo "¡Secuencia completa de benchmarks finalizada con éxito!"
