package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gentle-skills-bridge/bridge"
	"github.com/fsnotify/fsnotify"
)

const version = "1.0.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gentle-skills-bridge", flag.ContinueOnError)
	fs.SetOutput(stderr)

	configPath := fs.String("config", "config.json", "Ruta al archivo de configuración config.json (por defecto local o global)")
	dryRun := fs.Bool("dry-run", false, "Simula la sincronización sin escribir cambios físicos")

	fs.Usage = func() {
		fmt.Fprintln(stdout, "gentle-skills-bridge v"+version+" — Sincronizador de Skills para Gentle-AI y Engram")
		fmt.Fprintln(stdout, "Inspirado y dedicado a la comunidad de @gentlemanprogramming")
		fmt.Fprintln(stdout, "Uso:")
		fmt.Fprintln(stdout, "  gentle-skills-bridge <comando> [opciones]")
		fmt.Fprintln(stdout, "\nComandos:")
		fmt.Fprintln(stdout, "  sync     Realiza una sincronización única de skills")
		fmt.Fprintln(stdout, "  watch    Monitorea directorios en tiempo real y sincroniza cambios")
		fmt.Fprintln(stdout, "  add      Agrega la carpeta actual (o la especificada) como origen de skills")
		fmt.Fprintln(stdout, "  version  Muestra la versión instalada")
		fmt.Fprintln(stdout, "\nOpciones:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	parsedArgs := fs.Args()
	if len(parsedArgs) < 1 {
		fs.Usage()
		return 1
	}

	cmd := parsedArgs[0]

	// Determine dry-run status anywhere in args (workaround for flag placement after subcommands)
	hasDryRun := false
	for _, arg := range args {
		if arg == "--dry-run" || arg == "-dry-run" {
			hasDryRun = true
			break
		}
	}

	switch cmd {
	case "version":
		fmt.Fprintf(stdout, "gentle-skills-bridge v%s\n", version)
		return 0
	case "sync":
		cfg, _, err := loadConfig(*configPath, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "[error] Falló la carga de configuración: %v\n", err)
			return 1
		}
		cfg.DryRun = *dryRun || hasDryRun
		return runSync(cfg, stdout, stderr)
	case "watch":
		cfg, _, err := loadConfig(*configPath, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "[error] Falló la carga de configuración: %v\n", err)
			return 1
		}
		if *dryRun {
			fmt.Fprintln(stderr, "[error] El modo dry-run no está soportado en watch")
			return 1
		}
		return runWatch(cfg, stdout, stderr)
	case "add":
		targetPath := "."
		if len(parsedArgs) > 1 {
			targetPath = parsedArgs[1]
		}

		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			fmt.Fprintf(stderr, "[error] No se pudo resolver la ruta absoluta: %v\n", err)
			return 1
		}

		// Verify directory exists
		info, err := os.Stat(absPath)
		if err != nil {
			fmt.Fprintf(stderr, "[error] La ruta especificada no existe: %s\n", absPath)
			return 1
		}
		if !info.IsDir() {
			fmt.Fprintf(stderr, "[error] La ruta especificada no es un directorio: %s\n", absPath)
			return 1
		}

		cfg, activePath, err := loadConfig(*configPath, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "[error] Falló la carga de configuración: %v\n", err)
			return 1
		}

		// Normalize paths before checking duplication
		absClean := filepath.Clean(absPath)
		alreadyExists := false
		for _, src := range cfg.Sources {
			if filepath.Clean(src) == absClean {
				alreadyExists = true
				break
			}
		}

		if alreadyExists {
			fmt.Fprintf(stdout, "[info] La carpeta ya está registrada como origen de skills: %s\n", absClean)
			return 0
		}

		cfg.Sources = append(cfg.Sources, absClean)
		if err := saveConfig(activePath, cfg); err != nil {
			fmt.Fprintf(stderr, "[error] No se pudo guardar la configuración: %v\n", err)
			return 1
		}

		fmt.Fprintf(stdout, "[info] Carpeta agregada con éxito a los orígenes en %s:\n  -> %s\n", activePath, absClean)
		return 0

	default:
		fmt.Fprintf(stderr, "[error] Comando desconocido: %s\n\n", cmd)
		fs.Usage()
		return 1
	}
}

// loadConfig loads configuration from path, falling back to global config if default path not found.
func loadConfig(path string, stdout io.Writer) (*bridge.Config, string, error) {
	// If path is custom and doesn't exist, return error
	if path != "config.json" {
		if _, err := os.Stat(path); err != nil {
			return nil, "", fmt.Errorf("archivo de configuración no encontrado: %w", err)
		}
		cfg, err := readConfigFile(path)
		return cfg, path, err
	}

	// 1. Check local config.json
	if _, err := os.Stat(path); err == nil {
		cfg, err := readConfigFile(path)
		return cfg, path, err
	}

	// 2. Check global config.json in user home: ~/.gentle-skills-bridge/config.json
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("no se pudo obtener la carpeta home del usuario: %w", err)
	}

	globalFolder := filepath.Join(home, ".gentle-skills-bridge")
	globalPath := filepath.Join(globalFolder, "config.json")

	if _, err := os.Stat(globalPath); err == nil {
		cfg, err := readConfigFile(globalPath)
		return cfg, globalPath, err
	}

	// 3. Create default global config if it doesn't exist
	fmt.Fprintf(stdout, "[info] No se encontró config.json local ni global. Creando configuración global por defecto en:\n  -> %s\n\n", globalPath)
	if err := os.MkdirAll(globalFolder, 0755); err != nil {
		return nil, "", fmt.Errorf("no se pudo crear la carpeta global de configuración: %w", err)
	}

	defaultCfg := &bridge.Config{
		Sources:            []string{},
		Targets:            []string{},
		AutoDiscoverAgents: true,
		SyncToEngram:       true,
		EngramProject:      "global",
		WatchIntervalMS:    1000,
	}

	if err := saveConfig(globalPath, defaultCfg); err != nil {
		return nil, "", fmt.Errorf("no se pudo inicializar el archivo de configuración global: %w", err)
	}

	return defaultCfg, globalPath, nil
}

func readConfigFile(path string) (*bridge.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg bridge.Config
	dec := json.NewDecoder(file)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	// Basic validation as expected by TestRunInvalidConfig
	if len(cfg.Sources) == 0 && !cfg.AutoDiscoverAgents {
		return nil, fmt.Errorf("configuración inválida: debe haber al menos un origen (sources) o auto_discover_agents activo")
	}

	return &cfg, nil
}

func saveConfig(path string, cfg *bridge.Config) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func runSync(cfg *bridge.Config, stdout, stderr io.Writer) int {
	if cfg.DryRun {
		fmt.Fprintln(stdout, "[dry-run] Simulating synchronization without writing files")
	}
	fmt.Fprintln(stdout, "[info] Iniciando sincronización única...")
	startTime := time.Now()
	res, err := bridge.SyncFiles(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "[error] Sincronización fallida: %v\n", err)
		return 1
	}

	duration := time.Since(startTime)
	fmt.Fprintf(stdout, "\n[info] Sincronización completada en %v\n", duration)
	fmt.Fprintf(stdout, "  Procesados: %d\n", res.TotalProcessed)
	fmt.Fprintf(stdout, "  Sincronizados: %d\n", res.TotalSynced)
	fmt.Fprintf(stdout, "  Fallidos: %d\n", res.FailedCount)

	if len(res.Errors) > 0 {
		fmt.Fprintln(stdout, "\nErrores encontrados:")
		for _, errStr := range res.Errors {
			fmt.Fprintf(stdout, "  - %s\n", errStr)
		}
	}
	return 0
}

func runWatch(cfg *bridge.Config, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "[info] Iniciando modo monitoreo (watch)...")

	// Run initial sync
	runSync(cfg, stdout, stderr)

	// Setup fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(stderr, "[error] No se pudo crear el monitor de archivos: %v\n", err)
		return 1
	}
	defer watcher.Close()

	// Register source directories
	for _, src := range cfg.Sources {
		cleanSrc := filepath.Clean(src)
		if _, err := os.Stat(cleanSrc); os.IsNotExist(err) {
			fmt.Fprintf(stdout, "[warning] La carpeta de origen no existe, saltando: %s\n", cleanSrc)
			continue
		}

		// Watch directory and all subdirectories
		err = filepath.Walk(cleanSrc, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// Skip hidden dirs
				if filepath.Base(path) != filepath.Base(cleanSrc) && filepath.Base(path)[0] == '.' {
					return filepath.SkipDir
				}
				err = watcher.Add(path)
				if err != nil {
					fmt.Fprintf(stdout, "[warning] No se pudo monitorear la carpeta %s: %v\n", path, err)
				} else {
					fmt.Fprintf(stdout, "[watch] Monitoreando carpeta: %s\n", path)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "[error] Falló el registro de directorios en %s: %v\n", cleanSrc, err)
		}
	}

	// Debounce timer channel
	var debounceTimer *time.Timer
	debounceDuration := time.Duration(cfg.WatchIntervalMS) * time.Millisecond
	eventChan := make(chan bool)

	// Event handling loop
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// We only care about modifications, creations and deletions of markdown files
				if filepath.Ext(event.Name) == ".md" {
					if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
						fmt.Fprintf(stdout, "[watch] Cambio detectado: %s (%s)\n", event.Name, event.Op)
						eventChan <- true
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(stdout, "[watch-error] %v\n", err)
			}
		}
	}()

	// Debounce worker loop
	fmt.Fprintln(stdout, "\n[watch] Esperando cambios en tiempo real... (Presiona Ctrl+C para salir)")
	for {
		select {
		case <-eventChan:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				fmt.Fprintln(stdout, "\n[watch] Procesando cambios...")
				res, err := bridge.SyncFiles(cfg)
				if err != nil {
					fmt.Fprintf(stdout, "[watch-error] Falló la sincronización: %v\n", err)
				} else if res.FailedCount > 0 {
					fmt.Fprintf(stdout, "[watch] Completado con %d errores\n", res.FailedCount)
				} else {
					fmt.Fprintln(stdout, "[watch] Sincronización completada con éxito.")
				}
			})
		}
	}
}
