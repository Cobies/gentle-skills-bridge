package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// visualLen calcula el largo visual de una cadena ignorando los códigos ANSI de color.
func visualLen(s string) int {
	return len(ansiRegex.ReplaceAllString(s, ""))
}

// padLine rellena la cadena con espacios hasta el ancho especificado, considerando su largo visual real.
func padLine(content string, width int) string {
	visLen := visualLen(content)
	if visLen >= width {
		return content
	}
	return content + strings.Repeat(" ", width-visLen)
}

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
	ti.Width = 45

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

	// Colores ANSI estilo Cyberpunk/Terminal retro
	greenBright := "\x1b[38;5;82m" // Verde brillante fluorescente
	greenDim := "\x1b[38;5;29m"    // Verde apagado para detalles
	cyanBright := "\x1b[38;5;51m"  // Cian brillante
	whiteBold := "\x1b[1;37m"      // Blanco negrita
	reset := "\x1b[0m"

	boxWidth := 74 // Ancho interior exacto de la caja

	// Dibujar borde superior estilizado: ╔[ ID: GENTLE-BRIDGE ]════════════════════════════════════[ STATUS: ONLINE ]╗
	s.WriteString(greenBright + "╔[ ID: GENTLE-BRIDGE ]" + strings.Repeat("═", 31) + "[ STATUS: ACTIVE ]╗" + reset + "\n")

	// Línea en blanco
	s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

	// Dibujar logo COBIES en verde/cian brillante centrado
	cobiesLines := strings.Split(asciiCobies, "\n")
	for _, line := range cobiesLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Centrar la línea sumando espacios
		centeredLine := strings.Repeat(" ", 14) + cyanBright + line + reset
		s.WriteString(greenBright + "║ " + reset + padLine(centeredLine, boxWidth) + greenBright + " ║" + reset + "\n")
	}

	s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

	// Separador intermedio
	s.WriteString(greenBright + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")

	switch m.state {
	case "menu":
		s.WriteString(greenBright + "║ " + reset + padLine(whiteBold+"Seleccioná una opción para continuar:"+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

		for i, choice := range m.choices {
			if m.cursor == i {
				line := "  " + cyanBright + "❯ " + whiteBold + choice + reset
				s.WriteString(greenBright + "║ " + reset + padLine(line, boxWidth) + greenBright + " ║" + reset + "\n")
			} else {
				line := "    " + choice
				s.WriteString(greenBright + "║ " + reset + padLine(line, boxWidth) + greenBright + " ║" + reset + "\n")
			}
		}

		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

		if m.infoMessage != "" {
			s.WriteString(greenBright + "║ " + reset + padLine(greenBright+m.infoMessage+reset, boxWidth) + greenBright + " ║" + reset + "\n")
			s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		}

		s.WriteString(greenBright + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := greenDim + "j/k o ↑/↓: navegar • enter: seleccionar • q: salir" + reset
		s.WriteString(greenBright + "║ " + reset + padLine("  "+helpText, boxWidth) + greenBright + " ║" + reset + "\n")

	case "add":
		s.WriteString(greenBright + "║ " + reset + padLine(whiteBold+"AGREGAR CARPETA ORIGEN (Obsidian/Skills)"+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine(" Ingresá la ruta absoluta al directorio de notas:", boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

		tiStr := "   " + m.textInput.View()
		s.WriteString(greenBright + "║ " + reset + padLine(tiStr, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

		if m.errMessage != "" {
			s.WriteString(greenBright + "║ " + reset + padLine("   \x1b[31;1m[Error] "+m.errMessage+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		} else {
			s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		}

		s.WriteString(greenBright + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := greenDim + "enter: confirmar • esc: volver al menú" + reset
		s.WriteString(greenBright + "║ " + reset + padLine("  "+helpText, boxWidth) + greenBright + " ║" + reset + "\n")

	case "success":
		s.WriteString(greenBright + "║ " + reset + padLine(whiteBold+"OPERACIÓN COMPLETADA CON ÉXITO"+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")

		lines := strings.Split(m.infoMessage, "\n")
		for _, line := range lines {
			s.WriteString(greenBright + "║ " + reset + padLine("  "+greenBright+line+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		}

		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := greenDim + "Presioná Enter o Esc para volver al menú" + reset
		s.WriteString(greenBright + "║ " + reset + padLine("  "+helpText, boxWidth) + greenBright + " ║" + reset + "\n")

	case "version":
		s.WriteString(greenBright + "║ " + reset + padLine(whiteBold+"INFORMACIÓN DE VERSIÓN"+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("  Versión instalada: "+whiteBold+"v1.0.0"+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("  Configuración activa: "+greenBright+m.activePath+reset, boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "║ " + reset + padLine("", boxWidth) + greenBright + " ║" + reset + "\n")
		s.WriteString(greenBright + "╠" + strings.Repeat("═", boxWidth+2) + "╣" + reset + "\n")
		helpText := greenDim + "Presioná Enter o Esc para volver al menú" + reset
		s.WriteString(greenBright + "║ " + reset + padLine("  "+helpText, boxWidth) + greenBright + " ║" + reset + "\n")
	}

	// Dibujar borde inferior estilizado: ╚[ ENDEVS CONTROL PLANE ]═════════════════════════════════[ v1.0 - 2026 ]╝
	s.WriteString(greenBright + "╚[ ENDEVS CONTROL PLANE ]" + strings.Repeat("═", 34) + "[ v1.0 - 2026 ]╝" + reset + "\n")

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
		fmt.Fprintln(stdout, "\x1b[32;1m[+] ¡Chau! Gracias por usar gentle-skills-bridge.\x1b[0m")
		return 0
	}

	return 0
}
