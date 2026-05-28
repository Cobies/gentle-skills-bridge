package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gentle-skills-bridge/bridge"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ASCII Art de la rosa inspirado en Gentle-AI con soporte de colores ANSI
const asciiRose = `   
         .---.
        /     \
    (( |  * *  | ))
     \\ \   o   / //
      \\ '---' //
       \\  |  //
        \\ | //
         '-|-'
           |
           |
`

type tuiModel struct {
	choices     []string
	cursor      int
	selected    string
	configPath  string
	cfg         *bridge.Config
	activePath  string
	state       string // "menu", "add", "version", "success"
	textInput   textinput.Model
	errMessage  string
	infoMessage string
}

func initialModel(configPath string, cfg *bridge.Config, activePath string) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "C:/Ruta/A/Tus/Skills"
	ti.Focus()
	ti.CharLimit = 250
	ti.Width = 50

	return tuiModel{
		choices:    []string{"Sincronizar Skills (sync)", "Monitorear en tiempo real (watch)", "Agregar carpeta origen (add)", "Ver versión (version)", "Salir"},
		cursor:     0,
		configPath: configPath,
		cfg:        cfg,
		activePath: activePath,
		state:      "menu",
		textInput:  ti,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.selected = "exit"
			return m, tea.Quit
		}

		switch m.state {
		case "menu":
			switch msg.String() {
			case "q":
				m.selected = "exit"
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			case "enter":
				switch m.cursor {
				case 0: // Sync
					m.selected = "sync"
					return m, tea.Quit
				case 1: // Watch
					m.selected = "watch"
					return m, tea.Quit
				case 2: // Add Source
					m.state = "add"
					m.textInput.Reset()
					m.textInput.Focus()
					m.errMessage = ""
				case 3: // Version
					m.state = "version"
				case 4: // Exit
					m.selected = "exit"
					return m, tea.Quit
				}
			}

		case "add":
			switch msg.Type {
			case tea.KeyEsc:
				m.state = "menu"
				m.infoMessage = ""
				return m, nil
			case tea.KeyEnter:
				val := strings.TrimSpace(m.textInput.Value())
				if val == "" {
					m.errMessage = "La ruta no puede estar vacía."
					return m, nil
				}

				absPath, err := filepath.Abs(val)
				if err != nil {
					m.errMessage = fmt.Sprintf("Ruta inválida: %v", err)
					return m, nil
				}

				info, err := os.Stat(absPath)
				if err != nil {
					m.errMessage = fmt.Sprintf("La ruta no existe: %s", absPath)
					return m, nil
				}
				if !info.IsDir() {
					m.errMessage = "La ruta especificada no es un directorio."
					return m, nil
				}

				// Check duplicate
				absClean := filepath.Clean(absPath)
				alreadyExists := false
				for _, src := range m.cfg.Sources {
					if filepath.Clean(src) == absClean {
						alreadyExists = true
						break
					}
				}

				if alreadyExists {
					m.infoMessage = fmt.Sprintf("¡La carpeta ya estaba registrada!\n-> %s", absClean)
					m.state = "success"
					return m, nil
				}

				m.cfg.Sources = append(m.cfg.Sources, absClean)
				if err := saveConfig(m.activePath, m.cfg); err != nil {
					m.errMessage = fmt.Sprintf("Error al guardar config: %v", err)
					return m, nil
				}

				m.infoMessage = fmt.Sprintf("¡Carpeta agregada con éxito!\n-> %s", absClean)
				m.state = "success"
				return m, nil
			}

			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case "version", "success":
			switch msg.String() {
			case "enter", "esc", "q":
				m.state = "menu"
				m.infoMessage = ""
				m.errMessage = ""
			}
		}
	}

	return m, nil
}

func (m tuiModel) View() string {
	var s strings.Builder

	// Colores ANSI
	red := "\x1b[31;1m"
	green := "\x1b[32;1m"
	yellow := "\x1b[33;1m"
	cyan := "\x1b[36;1m"
	bold := "\x1b[1m"
	reset := "\x1b[0m"

	// Dibujar borde superior de caja
	s.WriteString("┌────────────────────────────────────────────────────────┐\n")

	// Dibujar rosa coloreada en rojo y tallo verde
	roseLines := strings.Split(asciiRose, "\n")
	for _, line := range roseLines {
		if line == "" {
			continue
		}
		// Colorear flor de rojo y hojas/tallo de verde
		coloredLine := line
		if strings.Contains(line, "*") || strings.Contains(line, "o") || strings.Contains(line, "/     \\") || strings.Contains(line, ".---.") {
			coloredLine = red + line + reset
		} else if strings.Contains(line, "|") || strings.Contains(line, "\\") || strings.Contains(line, "/") {
			coloredLine = green + line + reset
		}
		s.WriteString(fmt.Sprintf("│ %-54s │\n", coloredLine))
	}

	s.WriteString("│                                                        │\n")
	s.WriteString(fmt.Sprintf("│ %s%-54s%s │\n", bold, "  Gentle-Skills-Bridge v1.0.0 — Dashboard de Sincronización", reset))
	s.WriteString(fmt.Sprintf("│ %-54s │\n", "  Inspirado y dedicado a la comunidad de @gentlemanprogramming"))
	s.WriteString("├────────────────────────────────────────────────────────┤\n")

	switch m.state {
	case "menu":
		s.WriteString("│ Seleccioná una opción para continuar:                  │\n")
		s.WriteString("│                                                        │\n")
		for i, choice := range m.choices {
			cursorStr := "  "
			if m.cursor == i {
				cursorStr = cyan + "❯ " + reset
				s.WriteString(fmt.Sprintf("│ %s%-62s │\n", cursorStr, bold+choice+reset))
			} else {
				s.WriteString(fmt.Sprintf("│ %s%-54s │\n", cursorStr, choice))
			}
		}
		s.WriteString("│                                                        │\n")
		if m.infoMessage != "" {
			s.WriteString(fmt.Sprintf("│ %s%-62s │\n", green, m.infoMessage+reset))
			s.WriteString("│                                                        │\n")
		}
		s.WriteString("├────────────────────────────────────────────────────────┤\n")
		s.WriteString(fmt.Sprintf("│ %-54s │\n", "j/k o ↑/↓: navegar • enter: seleccionar • q/esc: salir"))

	case "add":
		s.WriteString("│ AGREGAR CARPETA ORIGEN (Obsidian/Skills)               │\n")
		s.WriteString("│                                                        │\n")
		s.WriteString("│ Ingresá la ruta absoluta al directorio de notas:        │\n")
		s.WriteString("│                                                        │\n")
		// Render text input inside box
		tiStr := m.textInput.View()
		s.WriteString(fmt.Sprintf("│ %-62s │\n", tiStr))
		s.WriteString("│                                                        │\n")
		if m.errMessage != "" {
			s.WriteString(fmt.Sprintf("│ %s%-62s │\n", red, "[Error] "+m.errMessage+reset))
		} else {
			s.WriteString("│                                                        │\n")
		}
		s.WriteString("├────────────────────────────────────────────────────────┤\n")
		s.WriteString(fmt.Sprintf("│ %-54s │\n", "enter: confirmar • esc: volver al menú"))

	case "success":
		s.WriteString("│ OPERACIÓN COMPLETADA CON ÉXITO                         │\n")
		s.WriteString("│                                                        │\n")
		lines := strings.Split(m.infoMessage, "\n")
		for _, line := range lines {
			s.WriteString(fmt.Sprintf("│ %s%-62s │\n", green, line+reset))
		}
		s.WriteString("│                                                        │\n")
		s.WriteString("├────────────────────────────────────────────────────────┤\n")
		s.WriteString(fmt.Sprintf("│ %-54s │\n", "Presioná Enter o Esc para volver al menú"))

	case "version":
		s.WriteString("│ INFORMACIÓN DE VERSIÓN                                 │\n")
		s.WriteString("│                                                        │\n")
		s.WriteString(fmt.Sprintf("│ Versión instalada: %s%-35s%s │\n", bold, "v1.0.0", reset))
		s.WriteString(fmt.Sprintf("│ Configuración activa: %s%-31s%s │\n", yellow, m.activePath, reset))
		s.WriteString("│                                                        │\n")
		s.WriteString("├────────────────────────────────────────────────────────┤\n")
		s.WriteString(fmt.Sprintf("│ %-54s │\n", "Presioná Enter o Esc para volver al menú"))
	}

	s.WriteString("└────────────────────────────────────────────────────────┘\n")
	return s.String()
}

func runInteractiveMenu(stdout, stderr io.Writer, configPath string, dryRun bool) int {
	cfg, activePath, err := loadConfig(configPath, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "[error] Falló la carga de configuración: %v\n", err)
		return 1
	}

	p := tea.NewProgram(initialModel(configPath, cfg, activePath), tea.WithOutput(stdout), tea.WithInput(os.Stdin))
	m, err := p.Run()
	if err != nil {
		fmt.Fprintf(stderr, "[error] Error en el menú interactivo: %v\n", err)
		return 1
	}

	finalModel := m.(tuiModel)

	switch finalModel.selected {
	case "sync":
		cfg.DryRun = dryRun
		return runSync(cfg, stdout, stderr)
	case "watch":
		if dryRun {
			fmt.Fprintln(stderr, "[error] El modo dry-run no está soportado en watch")
			return 1
		}
		return runWatch(cfg, stdout, stderr)
	case "exit":
		fmt.Fprintln(stdout, "¡Chau! Gracias por usar gentle-skills-bridge.")
		return 0
	}

	return 0
}
