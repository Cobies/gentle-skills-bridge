# Gentle Skills Bridge

Una utilidad multiplataforma escrita en Go diseñada para conectar tus notas y guías de desarrollo (como vaults de **Obsidian** u otras carpetas personalizadas) directamente con las carpetas de skills de tus agentes de desarrollo (**Gentle-AI**, **Antigravity**, **OpenCode**, **Claude Code**, etc.) e indexarlas en tiempo real en la memoria persistente de **Engram**.

Este proyecto se inspira y está dedicado con mucho respeto a **[@gentlemanprogramming](https://github.com/Gentleman-Programming)** y su ecosistema de ingeniería de agentes.

## ¿Qué problema resuelve?

1. **Gestión Descentralizada de Skills**: Te permite escribir y organizar tus skills y mejores prácticas de código usando tu editor favorito (como Obsidian) sin tener que copiar archivos manualmente en los directorios internos de cada agente de IA.
2. **Cumplimiento de Estructura**: Si tus notas de Obsidian no contienen el bloque de metadatos YAML de Gentle-AI (`name` y `description` triggers), la herramienta los genera automáticamente basándose en el título de la nota para que el agente pueda activarlas de forma contextual.
3. **Enlaces Multiplataforma Inteligentes**: Utiliza enlaces simbólicos de sistema (`symlinks`) para que cualquier cambio que hagas en Obsidian se refleje al instante. En sistemas como Windows donde los enlaces simbólicos requieren privilegios de Administrador, realiza un **copiado automático en segundo plano** al instante, garantizando un funcionamiento sin fricciones.
4. **Búsqueda Semántica en Engram**: Sincroniza el contenido completo de tus notas con la base de datos local de Engram para que cualquier agente pueda consultar tus guías de forma semántica con `mem_search`, incluso si no están cargadas de forma local en la sesión actual.
5. **Detección Automática de Agentes**: Lee el archivo `~/.gentle-ai/state.json` del sistema para autodetectar los agentes activos y registrar las carpetas de skills de todos ellos de manera unificada.

---

## Requisitos

- **Go** (v1.24 o superior recomendado, verificado en go1.26.3)
- **Engram** (Opcional, para la sincronización de memoria)

---

## Configuración y CLI Interactivo

La herramienta implementa una búsqueda de configuración en cascada:
1. Revisa si hay un `config.json` local en el directorio actual.
2. Si no lo hay, busca (y crea si falta) la configuración global en:  
   `~/.gentle-skills-bridge/config.json`

### Registro Rápido de Carpetas (`add`)
Para registrar la carpeta actual (o una ruta específica) como origen de notas de skills, solo tenés que abrir tu terminal en esa carpeta y ejecutar:

```bash
gentle-skills-bridge add
```

O podés especificar la ruta absoluta directamente:
```bash
gentle-skills-bridge add C:\Users\Cobies-PC\Obsidian\Vault\Skills
```
La herramienta se encarga de resolver la ruta, cargar tu `config.json` activo, agregarla al listado de `sources` (evitando duplicados) y guardarlo de forma interactiva.

---

## Estructura de `config.json`

El archivo de configuración tiene el siguiente formato:

```json
{
  "sources": [
    "C:/Users/Cobies-PC/Obsidian/Vault/Skills"
  ],
  "targets": [],
  "auto_discover_agents": true,
  "sync_to_engram": true,
  "engram_project": "global",
  "engram_url": "http://127.0.0.1:7437",
  "watch_interval_ms": 1000
}
```

---

## Comandos Disponibles

### 1. Registrar Origen (`add`)
Agrega una carpeta a las fuentes de skills:
```bash
go run main.go add [ruta]
```

### 2. Sincronización Única (`sync`)
Escanea todos tus orígenes, formatea las notas y las despliega en las carpetas de skills de tus agentes instalados:
```bash
go run main.go sync
```
*(Podés pasar el flag `--dry-run` para simular la sincronización sin escribir ningún archivo en disco)*.

### 3. Monitoreo en Tiempo Real (`watch`)
Monitorea en segundo plano todas tus carpetas origen. Cada vez que crees, modifiques o elimines una nota Markdown en tu Obsidian, se sincronizará automáticamente con todos tus agentes y se actualizará su memoria en Engram al instante:
```bash
go run main.go watch
```

---

---

## Instalación con Scoop (Recomendada)

Podés instalar la herramienta directamente en Windows utilizando **Scoop** sin necesidad de agregar ningún bucket previo, ejecutando el siguiente comando en tu terminal (PowerShell o CMD):

```bash
scoop install https://raw.githubusercontent.com/Cobies/gentle-skills-bridge/main/gentle-skills-bridge.json
```

Scoop descargará el ejecutable compilado, creará un shim automático y lo agregará al `PATH` de tu sistema para que esté disponible globalmente.

### Actualizar la herramienta
Para actualizar a la versión más reciente en el futuro:
```bash
scoop update gentle-skills-bridge
```

---

## Ejecución desde la Consola

Una vez instalado (ya sea mediante Scoop o compitiendo manualmente), podés abrir cualquier terminal y ejecutar la interfaz interactiva simplemente escribiendo:

```bash
gentle-skills-bridge
```

O podés usar comandos directos:
- **`gentle-skills-bridge sync`**: Sincroniza tus notas una única vez.
- **`gentle-skills-bridge watch`**: Deja el programa corriendo en segundo plano monitoreando cambios en tiempo real.
- **`gentle-skills-bridge add [ruta]`**: Agrega un directorio origen de notas.

### Requisitos para el funcionamiento:
1. **Configuración inicial**: La herramienta buscará un archivo `config.json` local. Si no existe, creará uno global automáticamente en tu directorio de usuario: `~/.gentle-skills-bridge/config.json`.
2. **Permisos de Administrador (para enlaces simbólicos)**: En Windows, la creación de `symlinks` (enlaces simbólicos) requiere permisos elevados. **Sin embargo, no te preocupes**: si no se ejecuta como Administrador, la herramienta detectará la falta de privilegios y **cambiará automáticamente a modo copia en segundo plano**, sincronizando tus archivos de forma segura.

---

## Compilación Manual (Desarrollo)

Si preferís compilar la herramienta vos mismo:

```bash
go build -ldflags="-s -w" -o gentle-skills-bridge.exe main.go
```

## Licencia

Este proyecto está bajo la Licencia MIT.
