# Análisis de Estrategia de Indexación: Unrolled vs. Packed

Este documento detalla la comparativa técnica entre la estrategia actual de indexación de propiedades (**Unrolled**) y la alternativa propuesta de lista única por propiedad (**Packed**), basada en pruebas de rendimiento realizadas.

## Definición de las Estrategias

### Estrategia Actual: **Unrolled Index**
Cada par propiedad-valor para un nodo se guarda como una clave individual en RocksDB.
*   **Clave:** `idx|prop|propiedad|valor|node_id`
*   **Valor:** `(vacío)`
*   **Mecanismo:** Recuperación mediante iteradores de prefijo (Prefix Scan).

### Estrategia Propuesta: **Packed Index**
Todos los IDs de los nodos que comparten una propiedad se guardan en una única lista (slice de bytes).
*   **Clave:** `idx|prop|propiedad|valor`
*   **Valor:** `[id1, id2, id3, ..., idN]`
*   **Mecanismo:** Recuperación mediante un único `Get` y deserialización en memoria.

---

## Resultados de Benchmarking (Apple M1)

Los siguientes datos reflejan el tiempo de inserción por lotes (batches de 20,000 nodos) y el tiempo de recuperación total.

| Nodos (Total) | Inserción Unrolled (ms) | Inserción Packed (ms) | Recuperación Unrolled (ms) | Recuperación Packed (ms) |
| :--- | :--- | :--- | :--- | :--- |
| **100k** | 59.1 ms | 1.5 ms | 25.9 ms | 0.14 ms |
| **1M** | 307.4 ms | 186.5 ms | 218.6 ms | 1.88 ms |
| **5M** | 1,328.4 ms | 4,725.9 ms | 1,286.9 ms | 7.29 ms |
| **10M** | **2,611.6 ms** | **42,034.5 ms** | 2,661.7 ms | 51.79 ms |

---

## ¿Por qué la estrategia Packed es una "Trampa de Rendimiento"?

### A. Complejidad Algorítmica ($O(N)$ vs $O(N^2)$)
*   **Unrolled:** Escala de forma **lineal**. Si duplicas los nodos, el tiempo de inserción se duplica. Es predecible.
*   **Packed:** Escala de forma **cuadrática**. Debido a que cada nuevo batch de datos obliga a leer la lista anterior, copiarla y re-escribirla por completo, el costo crece exponencialmente. A los 10M de nodos, la inserción es **16 veces más lenta** que la actual.

### B. Amplificación de Escritura (Write Amplification)
Para añadir un ID de 8 bytes a una propiedad compartida por 10 millones de nodos:
*   **En Unrolled:** Escribes ~50 bytes al disco.
*   **En Packed:** Tienes que leer ~80 MB del disco y **volver a escribir 80 MB**.
Esto genera un desgaste innecesario en el almacenamiento y satura el ancho de banda de la memoria y el bus de datos.

### C. Streaming vs. Bloqueo de Memoria
*   **Unrolled (Streaming):** El uso de iteradores permite procesar los resultados a medida que se encuentran. Puedes empezar a devolver datos al usuario en microsegundos.
*   **Packed (Atomic Load):** Obliga a cargar los 80 MB completos en RAM antes de poder procesar siquiera el primer resultado. Esto puede provocar latencias perceptibles y picos de memoria en sistemas con alta concurrencia.

---

## 4. Ventajas de RocksDB con la Estrategia Unrolled

RocksDB está diseñado específicamente para millones de registros pequeños:
1.  **Prefix Compression:** RocksDB comprime las claves ordenadas en disco, eliminando la redundancia de los prefijos repetidos. El ahorro de espacio de la estrategia Packed es marginal comparado con el riesgo de rendimiento.
2.  **Bloom Filters:** Permiten saber si una propiedad existe sin siquiera tocar el disco, algo que funciona igual de bien en ambas estrategias.
3.  **LSM-Tree Optimization:** La estructura interna de RocksDB gestiona mucho mejor las escrituras de claves pequeñas y frecuentes que la sobrescritura de valores gigantes.

---
