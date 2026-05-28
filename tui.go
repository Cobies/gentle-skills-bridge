package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gentle-skills-bridge/bridge"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ASCII Art de COBIES en estilo block / slant
const asciiCobies = `
  ██████╗  ██████╗ ██████╗ ██╗███████╗███████╗
 ██╔════╝ ██╔═══██╗██╔══██╗██║██╔════╝██╔════╝
 ██║      ██║   ██║██████╔╝██║█████╗  ███████╗
 ██║      ██║   ██║██╔══██╗██║██╔══╝  ╚════██║
 ╚██████╗ ╚██████╔╝██████╔╝██║███████╗███████║
  ╚═════╝  ╚═════╝ ╚══════╝╚═╝╚══════╝╚══════╝
`

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// visualLen calcula el largo visual de una cadena en columnas de pantalla,
// eliminando códigos de color ANSI y contando runas Unicode individuales.
func visualLen(s string) int {
	cleanStr := ansiRegex.ReplaceAllString(s, "")
	return utf8.RuneCountInString(cleanStr)
}

// padLine rellena la cadena con espacios hasta el ancho especificado, considerando su largo visual real en runas.
func padLine(content string, width int) string {
	visLen := visualLen(content)
	if visLen >= width {
		return content
	}
	return content + strings.Repeat(" ", width-visLen)
}

type tuiModel struct {
	choices      []string
	cursor       int
	removeCursor int
	selected     string
	configPath   string
	cfg          *bridge.Config
	activePath   string
	state        string // "menu", "add", "remove", "version", "success"
	textInput    textinput.Model
	errMessage   string
	infoMessage  string
}

func initialModel(configPath string, cfg *bridge.Config, activePath string) tuiModel {
	ti := textinput.New()
	ti.Placeholder = ". (Dejá vacío para agregar la carpeta actual)"
	ti.Focus()
	ti.CharLimit = 250
	ti.Width = 45

	return tuiModel{
		choices:      []string{"Sincronizar Skills (sync)", "Agregar carpeta origen (add)", "Quitar carpeta origen (remove)", "Ver versión (version)", "Salir"},
		cursor:       0,
		removeCursor: 0,
		configPath:   configPath,
		cfg:          cfg,
		activePath:   activePath,
		state:        "menu",
		textInput:    ti,
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
				case 1: // Add Source
					m.state = "add"
					m.textInput.Reset()
					m.textInput.Focus()
					m.errMessage = ""
				case 2: // Remove Source
					m.state = "remove"
					m.removeCursor = 0
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
					val = "." // Default to current directory if empty
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

		case "remove":
			switch msg.String() {
			case "esc", "q":
				m.state = "menu"
				m.infoMessage = ""
				return m, nil
			case "up", "k":
				if m.removeCursor > 0 {
					m.removeCursor--
				}
			case "down", "j":
				if m.removeCursor < len(m.cfg.Sources)-1 {
					m.removeCursor++
				}
			case "enter":
				if len(m.cfg.Sources) == 0 {
					m.state = "menu"
					return m, nil
				}

				removedPath := m.cfg.Sources[m.removeCursor]
				m.cfg.Sources = append(m.cfg.Sources[:m.removeCursor], m.cfg.Sources[m.removeCursor+1:]...)

				if err := saveConfig(m.activePath, m.cfg); err != nil {
					m.errMessage = fmt.Sprintf("Error al guardar config: %v", err)
					return m, nil
				}

				m.infoMessage = fmt.Sprintf("¡Carpeta removida de los orígenes!\n-> %s", removedPath)
				m.state = "success"
				return m, nil
			}

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

	// Colores ANSI sobrios (tonalidad normal de terminal estándar)
	borderCol := "\x1b[90m"       // Gris oscuro para contornos (tonalidad normal)
	textDim := "\x1b[37m"         // Blanco estándar
	cyanBright := "\x1b[36;1m"     // Cian brillante para logo y selección activa
	whiteBold := "\x1b[1;37m"     // Blanco negrita para títulos
	redBright := "\x1b[31;1m"     // Rojo brillante para errores
	greenBright := "\x1b[32;1m"   // Verde para éxito
	reset := "\x1b[0m"

	boxWidth := 74 // Ancho interior exacto de la caja

	// Dibujar borde superior estilizado: ╔[ ID: GENTLE-BRIDGE ]═════════════════════════════════════[ STATUS: ACTIVE ]╗
	s.WriteString(borderCol + "╔[ ID: GENTLE-BRIDGE ]" + strings.Repeat("═", 37) + "[ STATUS: ACTIVE ]╗" + reset + "\n")

	// Línea en blanco
	s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

	// Dibujar logo COBIES en cian brillante centrado
	cobiesLines := strings.Split(asciiCobies, "\n")
	for _, line := range cobiesLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Centrar la línea sumando espacios
		centeredLine := strings.Repeat(" ", 14) + cyanBright + line + reset
		s.WriteString(borderCol + "║ " + reset + padLine(centeredLine, boxWidth) + borderCol + " ║" + reset + "\n")
	}

	s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

	// Separador intermedio
	s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")

	switch m.state {
	case "menu":
		s.WriteString(borderCol + "║ " + reset + padLine(whiteBold+"Seleccioná una opción para continuar:"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		for i, choice := range m.choices {
			if m.cursor == i {
				line := "  " + cyanBright + "❯ " + whiteBold + choice + reset
				s.WriteString(borderCol + "║ " + reset + padLine(line, boxWidth) + borderCol + " ║" + reset + "\n")
			} else {
				line := "    " + textDim + choice + reset
				s.WriteString(borderCol + "║ " + reset + padLine(line, boxWidth) + borderCol + " ║" + reset + "\n")
			}
		}

		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		if m.infoMessage != "" {
			s.WriteString(borderCol + "║ " + reset + padLine(greenBright+m.infoMessage+reset, boxWidth) + borderCol + " ║" + reset + "\n")
			s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		}

		s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := borderCol + "j/k o ↑/↓: navegar • enter: seleccionar • q: salir" + reset
		s.WriteString(borderCol + "║ " + reset + padLine("  "+helpText, boxWidth) + borderCol + " ║" + reset + "\n")

	case "add":
		s.WriteString(borderCol + "║ " + reset + padLine(whiteBold+"AGREGAR CARPETA ORIGEN (Obsidian/Skills)"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine(" Ingresá la ruta absoluta al directorio de notas:", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		tiStr := "   " + m.textInput.View()
		s.WriteString(borderCol + "║ " + reset + padLine(tiStr, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		if m.errMessage != "" {
			s.WriteString(borderCol + "║ " + reset + padLine("   "+redBright+"[Error] "+m.errMessage+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		} else {
			s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		}

		s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := borderCol + "enter: confirmar • esc: volver al menú" + reset
		s.WriteString(borderCol + "║ " + reset + padLine("  "+helpText, boxWidth) + borderCol + " ║" + reset + "\n")

	case "remove":
		s.WriteString(borderCol + "║ " + reset + padLine(whiteBold+"QUITAR CARPETA ORIGEN"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		if len(m.cfg.Sources) == 0 {
			s.WriteString(borderCol + "║ " + reset + padLine(" No hay carpetas origen registradas actualmente.", boxWidth) + borderCol + " ║" + reset + "\n")
			s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		} else {
			s.WriteString(borderCol + "║ " + reset + padLine(" Seleccioná la carpeta que querés quitar y presioná Enter:", boxWidth) + borderCol + " ║" + reset + "\n")
			s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

			for i, src := range m.cfg.Sources {
				if m.removeCursor == i {
					line := "  " + redBright + "✗ " + whiteBold + src + reset
					s.WriteString(borderCol + "║ " + reset + padLine(line, boxWidth) + borderCol + " ║" + reset + "\n")
				} else {
					line := "    " + textDim + src + reset
					s.WriteString(borderCol + "║ " + reset + padLine(line, boxWidth) + borderCol + " ║" + reset + "\n")
				}
			}
		}

		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := borderCol + "j/k o ↑/↓: navegar • enter: quitar carpeta • esc: volver" + reset
		s.WriteString(borderCol + "║ " + reset + padLine("  "+helpText, boxWidth) + borderCol + " ║" + reset + "\n")

	case "success":
		s.WriteString(borderCol + "║ " + reset + padLine(whiteBold+"OPERACIÓN COMPLETADA CON ÉXITO"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")

		lines := strings.Split(m.infoMessage, "\n")
		for _, line := range lines {
			s.WriteString(borderCol + "║ " + reset + padLine("  "+greenBright+line+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		}

		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := borderCol + "Presioná Enter o Esc para volver al menú" + reset
		s.WriteString(borderCol + "║ " + reset + padLine("  "+helpText, boxWidth) + borderCol + " ║" + reset + "\n")

	case "version":
		s.WriteString(borderCol + "║ " + reset + padLine(whiteBold+"INFORMACIÓN DE VERSIÓN"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("  Versión instalada: "+whiteBold+"v1.0.0"+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("  Configuración activa: "+greenBright+m.activePath+reset, boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "║ " + reset + padLine("", boxWidth) + borderCol + " ║" + reset + "\n")
		s.WriteString(borderCol + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := borderCol + "Presioná Enter o Esc para volver al menú" + reset
		s.WriteString(borderCol + "║ " + reset + padLine("  "+helpText, boxWidth) + borderCol + " ║" + reset + "\n")
	}

	// Dibujar borde inferior estilizado: ╚══════════════════════════════════════════════════════════[ v1.0 - 2026 ]╝
	s.WriteString(borderCol + "╚" + strings.Repeat("═", 62) + "[ v1.0 - 2026 ]╝" + reset + "\n")

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
	case "exit":
		fmt.Fprintln(stdout, "\x1b[36;1m[+] ¡Chau! Gracias por usar gentle-skills-bridge.\x1b[0m")
		return 0
	}

	return 0
}
