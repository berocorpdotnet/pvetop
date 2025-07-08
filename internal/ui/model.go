package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/berocorpdotnet/pvetop/internal/api"
	"github.com/berocorpdotnet/pvetop/internal/models"
	"github.com/berocorpdotnet/pvetop/internal/theme"
)


type sortColumn int

const (
	sortByVMID sortColumn = iota
	sortByName
	sortByStatus
	sortByCPU
	sortByMem
	sortByDisk
	sortByDiskIO
	sortByNetIO
)

type viewMode int

const (
	viewGuests viewMode = iota
	viewNodes
)

type column int

const (
	colID column = iota
	colName
	colType
	colStatus
	colCPU
	colMem
	colMemGiB
	colDiskIO
	colNetIO
	colNode
)

type nodeColumn int

const (
	nodeColName nodeColumn = iota
	nodeColStatus
	nodeColCPU
	nodeColMem
	nodeColMemGiB
	nodeColDiskIO
	nodeColNetIO
	nodeColVMs
	nodeColCTs
)

type Model struct {
	client       *api.Client
	guests       []models.Guest
	nodes        []models.Node
	prevGuests   []models.Guest
	prevGuestMap map[int]models.Guest
	sortBy       sortColumn
	sortReverse  bool
	showAll      bool
	viewMode     viewMode
	isCluster    bool
	err          error
	width        int
	height       int
	keys         keyMap
	lastUpdate   time.Time
	lastFetch    time.Time
	scrollOffset int
	selectedRow  int
}

type keyMap struct {
	Quit       key.Binding
	Help       key.Binding
	SortVMID   key.Binding
	SortCPU    key.Binding
	SortMem    key.Binding
	SortDiskIO key.Binding
	SortNetIO  key.Binding
	Reverse    key.Binding
	ToggleAll  key.Binding
	ToggleView key.Binding
	Up         key.Binding
	Down       key.Binding
}

func NewModel(client *api.Client) Model {
	return Model{
		client:       client,
		sortBy:       sortByCPU,
		sortReverse:  true, 
		showAll:      false, 
		viewMode:     viewGuests,
		selectedRow:  -1, 
		scrollOffset: 0,
		keys: keyMap{
			Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			Help:       key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
			SortVMID:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "sort by VMID")),
			SortCPU:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "sort by CPU")),
			SortMem:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "sort by memory")),
			SortDiskIO: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "sort by disk I/O")),
			SortNetIO:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "sort by net I/O")),
			Reverse:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reverse sort")),
			ToggleAll:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle all/active")),
			ToggleView: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "toggle nodes/guests")),
			Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "scroll up")),
			Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "scroll down")),
		},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		tea.EnterAltScreen,
		tea.ClearScreen,
	)
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) getVisibleColumns() map[column]bool {
	visible := make(map[column]bool)
	
	visible[colName] = true
	visible[colNode] = true
	
	totalWidth := 20 + 8 + 2 
	
	columns := []struct {
		col   column
		width int
	}{
		{colID, 7},        
		{colType, 7},      
		{colStatus, 9},    
		{colCPU, 7},       
		{colMem, 7},       
		{colMemGiB, 16},   
		{colNetIO, 14},    
		{colDiskIO, 14},   
	}
	
	for _, col := range columns {
		totalWidth += col.width
	}
	
	sacrificeOrder := []column{colDiskIO, colNetIO, colMemGiB, colID, colStatus, colType, colMem, colCPU}
	
	for _, col := range sacrificeOrder {
		if totalWidth <= m.width {
			break 
		}
		visible[col] = false
		for _, c := range columns {
			if c.col == col {
				totalWidth -= c.width
				break
			}
		}
	}
	
	for _, colInfo := range columns {
		if _, exists := visible[colInfo.col]; !exists {
			visible[colInfo.col] = true
		}
	}
	
	return visible
}

func (m Model) formatHeaders(visible map[column]bool) string {
	var parts []string
	
	columnOrder := []column{colID, colName, colType, colStatus, colCPU, colMem, colMemGiB, colDiskIO, colNetIO, colNode}
	
	for _, col := range columnOrder {
		if !visible[col] {
			continue
		}
		switch col {
		case colID:
			parts = append(parts, fmt.Sprintf("%-6s", "ID"))
		case colName:
			parts = append(parts, fmt.Sprintf("%-20s", "NAME"))
		case colType:
			parts = append(parts, fmt.Sprintf("%-6s", "TYPE"))
		case colStatus:
			parts = append(parts, fmt.Sprintf("%-8s", "STATUS"))
		case colCPU:
			parts = append(parts, fmt.Sprintf("%6s", "CPU%"))
		case colMem:
			parts = append(parts, fmt.Sprintf("%6s", "MEM%"))
		case colMemGiB:
			parts = append(parts, fmt.Sprintf("%15s", "MEM(GiB)"))
		case colDiskIO:
			parts = append(parts, fmt.Sprintf("%13s", "DISK(KiB/s)"))
		case colNetIO:
			parts = append(parts, fmt.Sprintf("%13s", "NET(KiB/s)"))
		case colNode:
			parts = append(parts, fmt.Sprintf("%-8s", "NODE"))
		}
	}
	
	return strings.Join(parts, " ")
}

func (m Model) formatGuestRow(guest models.Guest, visible map[column]bool) string {
	var parts []string
	
	columnOrder := []column{colID, colName, colType, colStatus, colCPU, colMem, colMemGiB, colDiskIO, colNetIO, colNode}
	
	for _, col := range columnOrder {
		if !visible[col] {
			continue
		}
		switch col {
		case colID:
			parts = append(parts, fmt.Sprintf("%-6d", guest.VMID))
		case colName:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%-20s", truncate(guest.Name, 20))))
			} else {
				nameStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Text)
				parts = append(parts, nameStyle.Render(fmt.Sprintf("%-20s", truncate(guest.Name, 20))))
			}
		case colType:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%-6s", guest.Type)))
			} else {
				typeColor := theme.Catppuccin.Blue
				if guest.Type == "lxc" {
					typeColor = theme.Catppuccin.Peach
				}
				typeStyle := lipgloss.NewStyle().Foreground(typeColor)
				parts = append(parts, typeStyle.Render(fmt.Sprintf("%-6s", guest.Type)))
			}
		case colStatus:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%-8s", guest.Status)))
			} else {
				statusStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Green).Bold(true)
				parts = append(parts, statusStyle.Render(fmt.Sprintf("%-8s", guest.Status)))
			}
		case colCPU:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%6s", "—")))
			} else {
				cpuPercent := guest.CPU * 100
				cpuColor := theme.Catppuccin.Green
				if cpuPercent > 80 {
					cpuColor = theme.Catppuccin.Red
				} else if cpuPercent > 50 {
					cpuColor = theme.Catppuccin.Yellow
				}
				cpuStyle := lipgloss.NewStyle().Foreground(cpuColor)
				parts = append(parts, cpuStyle.Render(fmt.Sprintf("%6.1f", cpuPercent)))
			}
		case colMem:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%6s", "—")))
			} else {
				memPercent := float64(guest.Mem) / float64(guest.MaxMem) * 100
				memColor := theme.Catppuccin.Green
				if memPercent > 80 {
					memColor = theme.Catppuccin.Red
				} else if memPercent > 50 {
					memColor = theme.Catppuccin.Yellow
				}
				memStyle := lipgloss.NewStyle().Foreground(memColor)
				parts = append(parts, memStyle.Render(fmt.Sprintf("%6.1f", memPercent)))
			}
		case colMemGiB:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%15s", "—")))
			} else {
				memUsedGiB := float64(guest.Mem) / (1024 * 1024 * 1024)
				memMaxGiB := float64(guest.MaxMem) / (1024 * 1024 * 1024)
				memGiBText := fmt.Sprintf("%5.1f / %-5.1f", memUsedGiB, memMaxGiB)
				parts = append(parts, fmt.Sprintf("%15s", memGiBText))
			}
		case colDiskIO:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%13s", "—")))
			} else {
				diskRate := m.getDiskRate(guest)
				parts = append(parts, fmt.Sprintf("%13s", diskRate))
			}
		case colNetIO:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%13s", "—")))
			} else {
				netRate := m.getNetRate(guest)
				parts = append(parts, fmt.Sprintf("%13s", netRate))
			}
		case colNode:
			if guest.Status != "running" {
				greyStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Overlay0)
				parts = append(parts, greyStyle.Render(fmt.Sprintf("%-8s", guest.Node)))
			} else {
				parts = append(parts, fmt.Sprintf("%-8s", guest.Node))
			}
		}
	}
	
	return strings.Join(parts, " ")
}

func (m Model) getVisibleNodeColumns() map[nodeColumn]bool {
	visible := make(map[nodeColumn]bool)
	
	visible[nodeColName] = true
	
	totalWidth := 12 + 1 
	
	columns := []struct {
		col   nodeColumn
		width int
	}{
		{nodeColStatus, 9},    
		{nodeColCPU, 7},       
		{nodeColMem, 7},       
		{nodeColMemGiB, 16},   
		{nodeColDiskIO, 14},   
		{nodeColNetIO, 14},    
		{nodeColVMs, 7},       
		{nodeColCTs, 7},       
	}
	
	for _, col := range columns {
		totalWidth += col.width
	}
	
	sacrificeOrder := []nodeColumn{nodeColDiskIO, nodeColNetIO, nodeColMemGiB, nodeColVMs, nodeColCTs, nodeColStatus, nodeColMem, nodeColCPU}
	
	for _, col := range sacrificeOrder {
		if totalWidth <= m.width {
			break 
		}
		visible[col] = false
		for _, c := range columns {
			if c.col == col {
				totalWidth -= c.width
				break
			}
		}
	}
	
	for _, colInfo := range columns {
		if _, exists := visible[colInfo.col]; !exists {
			visible[colInfo.col] = true
		}
	}
	
	return visible
}

func (m Model) formatNodeHeaders(visible map[nodeColumn]bool) string {
	var parts []string
	
	columnOrder := []nodeColumn{nodeColName, nodeColStatus, nodeColCPU, nodeColMem, nodeColMemGiB, nodeColDiskIO, nodeColNetIO, nodeColVMs, nodeColCTs}
	
	for _, col := range columnOrder {
		if !visible[col] {
			continue
		}
		switch col {
		case nodeColName:
			parts = append(parts, fmt.Sprintf("%-12s", "NODE"))
		case nodeColStatus:
			parts = append(parts, fmt.Sprintf("%-8s", "STATUS"))
		case nodeColCPU:
			parts = append(parts, fmt.Sprintf("%6s", "CPU%"))
		case nodeColMem:
			parts = append(parts, fmt.Sprintf("%6s", "MEM%"))
		case nodeColMemGiB:
			parts = append(parts, fmt.Sprintf("%15s", "MEM(GiB)"))
		case nodeColDiskIO:
			parts = append(parts, fmt.Sprintf("%13s", "DISK(KiB/s)"))
		case nodeColNetIO:
			parts = append(parts, fmt.Sprintf("%13s", "NET(KiB/s)"))
		case nodeColVMs:
			parts = append(parts, fmt.Sprintf("%6s", "#VMs"))
		case nodeColCTs:
			parts = append(parts, fmt.Sprintf("%6s", "#CTs"))
		}
	}
	
	return strings.Join(parts, " ")
}

func (m Model) formatNodeRow(node models.Node, visible map[nodeColumn]bool) string {
	var parts []string
	
	cpuPercent := node.CPU * 100
	memPercent := float64(node.Mem) / float64(node.MaxMem) * 100
	
	vmCount, ctCount := m.countGuestsOnNode(node.Node)
	
	diskRate := m.getNodeDiskRate(node.Node)
	netRate := m.getNodeNetRate(node.Node)
	
	memUsedGiB := float64(node.Mem) / (1024 * 1024 * 1024)
	memMaxGiB := float64(node.MaxMem) / (1024 * 1024 * 1024)
	memGiBText := fmt.Sprintf("%5.1f / %-5.1f", memUsedGiB, memMaxGiB)
	
	columnOrder := []nodeColumn{nodeColName, nodeColStatus, nodeColCPU, nodeColMem, nodeColMemGiB, nodeColDiskIO, nodeColNetIO, nodeColVMs, nodeColCTs}
	
	for _, col := range columnOrder {
		if !visible[col] {
			continue
		}
		switch col {
		case nodeColName:
			parts = append(parts, fmt.Sprintf("%-12s", node.Node))
		case nodeColStatus:
			statusColor := theme.Catppuccin.Red
			if node.Status == "online" {
				statusColor = theme.Catppuccin.Green
			}
			statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
			parts = append(parts, statusStyle.Render(fmt.Sprintf("%-8s", node.Status)))
		case nodeColCPU:
			cpuColor := theme.Catppuccin.Green
			if cpuPercent > 80 {
				cpuColor = theme.Catppuccin.Red
			} else if cpuPercent > 50 {
				cpuColor = theme.Catppuccin.Yellow
			}
			cpuStyle := lipgloss.NewStyle().Foreground(cpuColor)
			parts = append(parts, cpuStyle.Render(fmt.Sprintf("%6.1f", cpuPercent)))
		case nodeColMem:
			memColor := theme.Catppuccin.Green
			if memPercent > 80 {
				memColor = theme.Catppuccin.Red
			} else if memPercent > 50 {
				memColor = theme.Catppuccin.Yellow
			}
			memStyle := lipgloss.NewStyle().Foreground(memColor)
			parts = append(parts, memStyle.Render(fmt.Sprintf("%6.1f", memPercent)))
		case nodeColMemGiB:
			parts = append(parts, fmt.Sprintf("%15s", memGiBText))
		case nodeColDiskIO:
			parts = append(parts, fmt.Sprintf("%13s", diskRate))
		case nodeColNetIO:
			parts = append(parts, fmt.Sprintf("%13s", netRate))
		case nodeColVMs:
			parts = append(parts, fmt.Sprintf("%6d", vmCount))
		case nodeColCTs:
			parts = append(parts, fmt.Sprintf("%6d", ctCount))
		}
	}
	
	return strings.Join(parts, " ")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen 

	case tickMsg:
		return m, tea.Batch(
			tick(),
			m.fetchData(),
		)

	case dataMsg:
		prevGuestMap := make(map[int]models.Guest, len(m.guests))
		for _, guest := range m.guests {
			prevGuestMap[guest.VMID] = guest
		}
		m.prevGuestMap = prevGuestMap
		m.prevGuests = m.guests
		m.guests = msg.guests
		m.nodes = msg.nodes
		m.isCluster = len(msg.nodes) > 1
		now := time.Now()
		if !m.lastUpdate.IsZero() {
			m.lastFetch = m.lastUpdate
		}
		m.lastUpdate = now
		if m.lastFetch.IsZero() {
			m.lastFetch = now
		}
		m.sortGuests()

	case errMsg:
		m.err = msg.err

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}

		case key.Matches(msg, m.keys.Down):
			contentHeight := m.height - 5 
			if contentHeight < 1 {
				contentHeight = 1
			}
			displayGuests := m.getDisplayGuests()
			maxScroll := len(displayGuests) - contentHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset < maxScroll {
				m.scrollOffset++
			}

		case key.Matches(msg, m.keys.SortVMID):
			m.sortBy = sortByVMID
			m.sortGuests()
			m.scrollOffset = 0 

		case key.Matches(msg, m.keys.SortCPU):
			m.sortBy = sortByCPU
			m.sortGuests()
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.SortMem):
			m.sortBy = sortByMem
			m.sortGuests()
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.SortDiskIO):
			m.sortBy = sortByDiskIO
			m.sortGuests()
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.SortNetIO):
			m.sortBy = sortByNetIO
			m.sortGuests()
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.Reverse):
			m.sortReverse = !m.sortReverse
			m.sortGuests()
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.ToggleAll):
			m.showAll = !m.showAll
			m.scrollOffset = 0 

		case key.Matches(msg, m.keys.ToggleView):
			if len(m.nodes) > 0 { 
				if m.viewMode == viewGuests {
					m.viewMode = viewNodes
				} else {
					m.viewMode = viewGuests
				}
				m.scrollOffset = 0 
			}
		}
	}

	return m, nil
}

func (m Model) getDisplayGuests() []models.Guest {
	displayGuests := m.guests
	if !m.showAll {
		var activeGuests []models.Guest
		for _, g := range m.guests {
			if g.Status == "running" {
				activeGuests = append(activeGuests, g)
			}
		}
		displayGuests = activeGuests
	}
	return displayGuests
}

func (m *Model) sortGuests() {
	sort.Slice(m.guests, func(i, j int) bool {
		if m.guests[i].Status == "running" && m.guests[j].Status != "running" {
			return true
		}
		if m.guests[i].Status != "running" && m.guests[j].Status == "running" {
			return false
		}
		
		if m.guests[i].Status != "running" && m.guests[j].Status != "running" {
			return m.guests[i].VMID < m.guests[j].VMID
		}
		
		var less bool
		switch m.sortBy {
		case sortByVMID:
			less = m.guests[i].VMID < m.guests[j].VMID
		case sortByName:
			less = m.guests[i].Name < m.guests[j].Name
		case sortByStatus:
			less = m.guests[i].Status < m.guests[j].Status
		case sortByCPU:
			less = m.guests[i].CPU < m.guests[j].CPU
		case sortByMem:
			less = float64(m.guests[i].Mem)/float64(m.guests[i].MaxMem) < float64(m.guests[j].Mem)/float64(m.guests[j].MaxMem)
		case sortByDisk:
			less = m.guests[i].Disk < m.guests[j].Disk
		case sortByDiskIO:
			iRate := m.getDiskRateNumeric(m.guests[i])
			jRate := m.getDiskRateNumeric(m.guests[j])
			less = iRate < jRate
		case sortByNetIO:
			iRate := m.getNetRateNumeric(m.guests[i])
			jRate := m.getNetRateNumeric(m.guests[j])
			less = iRate < jRate
		}
		
		if m.sortReverse {
			return !less
		}
		return less
	})
}

type dataMsg struct {
	guests []models.Guest
	nodes  []models.Node
}

type errMsg struct {
	err error
}

func (m Model) fetchData() tea.Cmd {
	return func() tea.Msg {
		nodes, err := m.client.GetNodes()
		if err != nil {
			return errMsg{err: err}
		}
		
		guests, err := m.client.GetAllGuests()
		if err != nil {
			return errMsg{err: err}
		}
		
		return dataMsg{
			guests: guests,
			nodes:  nodes,
		}
	}
}

const (
	minTerminalWidth  = 35
	minTerminalHeight = 10
	widthLarge        = 80
	widthMedium       = 60
	widthSmall        = 50
	widthTiny         = 40
)

func (m Model) View() string {
	
	if m.width < minTerminalWidth || m.height < minTerminalHeight {
		message := fmt.Sprintf("Terminal too small\nMinimum: %dx%d\nCurrent: %dx%d", 
			minTerminalWidth, minTerminalHeight, m.width, m.height)
		
		messageStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(theme.Catppuccin.Text).
			Background(theme.Catppuccin.Base)
			
		return messageStyle.Render(message)
	}

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(theme.Catppuccin.Red)
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress 'q' to quit.", m.err))
	}

	if m.viewMode == viewNodes && len(m.nodes) > 0 {
		return m.viewNodes()
	}
	return m.viewGuests()
}

func (m Model) viewNodes() string {
	var s string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Catppuccin.Text).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	
	onlineNodes := 0
	for _, node := range m.nodes {
		if node.Status == "online" {
			onlineNodes++
		}
	}
	
	var headerText string
	if m.width >= widthLarge {
		nodeViewTitle := "cluster nodes"
		if len(m.nodes) == 1 {
			nodeViewTitle = "node"
		}
		headerText = fmt.Sprintf(" pvetop - %s (%d/%d online) - refresh: 2s ", 
			nodeViewTitle, onlineNodes, len(m.nodes))
	} else if m.width >= widthSmall {
		headerText = fmt.Sprintf(" pvetop nodes (%d/%d online) ", onlineNodes, len(m.nodes))
	} else {
		headerText = fmt.Sprintf(" pvetop (%d/%d) ", onlineNodes, len(m.nodes))
	}
	
	s += headerStyle.Render(headerText)
	s += "\n\n"

	visibleNodeCols := m.getVisibleNodeColumns()
	
	colHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Catppuccin.Subtext1).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	headers := m.formatNodeHeaders(visibleNodeCols)
	s += colHeaderStyle.Render(headers) + "\n"

	contentHeight := m.height - 5 
	if contentHeight < 1 {
		contentHeight = 1
	}
	
	for _, node := range m.nodes {
		rowStyle := lipgloss.NewStyle().Width(m.width)
		
		row := m.formatNodeRow(node, visibleNodeCols)
		s += rowStyle.Render(row) + "\n"
	}

	usedHeight := 4 + len(m.nodes) 
	paddingLines := m.height - usedHeight - 1 
	if paddingLines < 0 {
		paddingLines = 0
	}
	for i := 0; i < paddingLines; i++ {
		s += "\n"
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Subtext1).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	
	var helpText string
	if m.width >= widthLarge {
		helpText = "q:quit | n:switch-to-guests | c:sort-cpu | m:sort-mem | r:reverse"
	} else if m.width >= widthMedium {
		helpText = "q:quit | n:guests | c/m:sort | r:reverse"
	} else if m.width >= widthTiny {
		helpText = "q:quit | n:guests | c/m:sort"
	} else {
		helpText = "q:quit"
	}
	
	s += "\n" + helpStyle.Render(helpText)

	return s
}

func (m Model) viewGuests() string {
	var s string

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Catppuccin.Text).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	
	displayGuests := m.guests
	if !m.showAll {
		var activeGuests []models.Guest
		for _, g := range m.guests {
			if g.Status == "running" {
				activeGuests = append(activeGuests, g)
			}
		}
		displayGuests = activeGuests
	}
	
	totalGuests := len(m.guests)
	activeGuests := 0
	for _, g := range m.guests {
		if g.Status == "running" {
			activeGuests++
		}
	}
	
	var headerText string
	if m.width >= widthLarge {
		headerText = fmt.Sprintf(" pvetop - connected to %s (%d/%d running guests) - refresh: 2s ", 
			"proxmox", activeGuests, totalGuests)
	} else if m.width >= widthSmall {
		headerText = fmt.Sprintf(" pvetop (%d/%d running) ", activeGuests, totalGuests)
	} else {
		headerText = fmt.Sprintf(" pvetop (%d/%d) ", activeGuests, totalGuests)
	}
	
	s += headerStyle.Render(headerText)
	s += "\n\n"

	visibleCols := m.getVisibleColumns()
	
	colHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Catppuccin.Subtext1).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	headers := m.formatHeaders(visibleCols)
	s += colHeaderStyle.Render(headers) + "\n"

	contentHeight := m.height - 5 
	if contentHeight < 1 {
		contentHeight = 1
	}
	
	maxScroll := len(displayGuests) - contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	
	startIdx := m.scrollOffset
	endIdx := startIdx + contentHeight
	if endIdx > len(displayGuests) {
		endIdx = len(displayGuests)
	}
	
	visibleGuests := displayGuests[startIdx:endIdx]

	for _, guest := range visibleGuests {
		
		rowStyle := lipgloss.NewStyle().Width(m.width)
		
		row := m.formatGuestRow(guest, visibleCols)
		s += rowStyle.Render(row) + "\n"
	}

	usedHeight := 4 + len(visibleGuests) 
	paddingLines := m.height - usedHeight - 1 
	if paddingLines < 0 {
		paddingLines = 0
	}
	for i := 0; i < paddingLines; i++ {
		s += "\n"
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Catppuccin.Subtext1).
		Background(theme.Catppuccin.Surface1).
		Width(m.width)
	
	var helpText string
	if m.width >= widthLarge {
		helpText = "q:quit | ↑↓/jk:scroll | c/m/d/i:sort | r:reverse | a:toggle-all"
		if len(m.nodes) > 0 {
			helpText += " | n:nodes"
		}
	} else if m.width >= widthMedium {
		helpText = "q:quit | ↑↓:scroll | c/m:sort | r:reverse | a:all"
		if len(m.nodes) > 0 {
			helpText += " | n:nodes"
		}
	} else if m.width >= widthTiny {
		helpText = "q:quit | ↑↓:scroll | c/m:sort"
	} else {
		helpText = "q:quit"
	}
	
	s += "\n" + helpStyle.Render(helpText)

	return s
}


func (m Model) getDiskRate(guest models.Guest) string {
	rate := m.getDiskRateNumeric(guest)
	return formatBytesPerSec(rate)
}

func (m Model) getDiskRateNumeric(guest models.Guest) int64 {
	if prev, ok := m.prevGuestMap[guest.VMID]; ok {
		timeDiff := m.lastUpdate.Sub(m.lastFetch).Seconds()
		if timeDiff > 0 {
			readDiff := guest.DiskRead - prev.DiskRead
			writeDiff := guest.DiskWrite - prev.DiskWrite
			if readDiff < 0 { readDiff = 0 }
			if writeDiff < 0 { writeDiff = 0 }
			totalDiff := readDiff + writeDiff
			return int64(float64(totalDiff) / timeDiff)
		}
	}
	return 0
}

func (m Model) getNetRate(guest models.Guest) string {
	rate := m.getNetRateNumeric(guest)
	return formatBytesPerSec(rate)
}

func (m Model) getNetRateNumeric(guest models.Guest) int64 {
	if prev, ok := m.prevGuestMap[guest.VMID]; ok {
		timeDiff := m.lastUpdate.Sub(m.lastFetch).Seconds()
		if timeDiff > 0 {
			inDiff := guest.NetIn - prev.NetIn
			outDiff := guest.NetOut - prev.NetOut
			if inDiff < 0 { inDiff = 0 }
			if outDiff < 0 { outDiff = 0 }
			totalDiff := inDiff + outDiff
			return int64(float64(totalDiff) / timeDiff)
		}
	}
	return 0
}

func (m Model) calculateHostCPUPercent(guest models.Guest) float64 {
	for _, node := range m.nodes {
		if node.Node == guest.Node {
			return (guest.CPU * float64(guest.CPUs)) / float64(node.MaxCPU) * 100
		}
	}
	return 0
}

func (m Model) calculateHostMemPercent(guest models.Guest) float64 {
	for _, node := range m.nodes {
		if node.Node == guest.Node {
			return float64(guest.Mem) / float64(node.MaxMem) * 100
		}
	}
	return 0
}

func (m Model) countGuestsOnNode(nodeName string) (vmCount, ctCount int) {
	for _, guest := range m.guests {
		if guest.Node == nodeName {
			if guest.Type == "qemu" {
				vmCount++
			} else if guest.Type == "lxc" {
				ctCount++
			}
		}
	}
	return vmCount, ctCount
}

func (m Model) getNodeDiskRate(nodeName string) string {
	var totalRate int64
	for _, guest := range m.guests {
		if guest.Node == nodeName && guest.Status == "running" {
			totalRate += m.getDiskRateNumeric(guest)
		}
	}
	return formatBytesPerSec(totalRate)
}

func (m Model) getNodeNetRate(nodeName string) string {
	var totalRate int64
	for _, guest := range m.guests {
		if guest.Node == nodeName && guest.Status == "running" {
			totalRate += m.getNetRateNumeric(guest)
		}
	}
	return formatBytesPerSec(totalRate)
}


func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatBytesPerSec(b int64) string {
	kibps := float64(b) / 1024
	return fmt.Sprintf("%8.1f", kibps)
}

func formatBytesShort(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, minutes)
	}
	return fmt.Sprintf("%02dh %02dm", hours, minutes)
}
