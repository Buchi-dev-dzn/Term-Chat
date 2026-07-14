package tui

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termchat/internal/discovery"
	"termchat/internal/protocol"
)

type viewMode int

const (
	viewSetup viewMode = iota
	viewChat
)

type setupStep int

const (
	stepMode setupStep = iota
	stepNick
	stepJoinLoading
	stepJoinList
	stepJoinManualAddr
	stepJoinManualRoom
	stepJoinManualProtection
	stepJoinPasscode
	stepHostRoom
	stepHostProtection
	stepHostPasscode
	stepConnecting
)

type browseDoneMsg struct {
	records []discovery.DiscoveryRecord
	err     error
}

type connectDoneMsg struct {
	client     *protocol.Client
	advertiser *discovery.Advertiser
	events     <-chan protocol.Event
	localInfo  string
	err        error
}

type chatEventMsg struct {
	event protocol.Event
}

type chatClosedMsg struct{}

type sendResultMsg struct {
	body   string
	sentAt time.Time
	err    error
}

type tuiModel struct {
	appCtx     context.Context
	cancel     context.CancelFunc
	mode       viewMode
	step       setupStep
	width      int
	height     int
	err        string
	status     string
	state      SetupState
	rooms      []discovery.DiscoveryRecord
	cursor     int
	input      textinput.Model
	logs       []string
	people     []string
	peopleSet  map[string]struct{}
	client     *protocol.Client
	advertiser *discovery.Advertiser
	events     <-chan protocol.Event
	localInfo  string
}

var (
	appBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1)
	titleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true)
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	panelStyle  = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
)

func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	model := newTUIModel(appCtx, cancel)
	program := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen(), tea.WithContext(appCtx))
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(tuiModel)
	if !ok {
		return nil
	}

	if m.client != nil {
		_ = m.client.Close()
	}
	if m.advertiser != nil {
		m.advertiser.Close()
	}
	if m.err != "" && m.mode != viewChat {
		return fmt.Errorf("%s", m.err)
	}
	return nil
}

func newTUIModel(ctx context.Context, cancel context.CancelFunc) tuiModel {
	input := textinput.New()
	input.CharLimit = 256
	input.Width = 48

	return tuiModel{
		appCtx:    ctx,
		cancel:    cancel,
		mode:      viewSetup,
		step:      stepMode,
		status:    "Choose host or join.",
		input:     input,
		peopleSet: map[string]struct{}{},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}
	}

	if m.mode == viewChat {
		return m.updateChat(msg)
	}
	return m.updateSetup(msg)
}

func (m tuiModel) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case browseDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.step = stepJoinManualAddr
			m.status = "Discovery failed. Enter server IP:port manually."
			m.prepareInput("Server IP:port", "192.168.0.10:9000", false)
			return m, textinput.Blink
		}
		sortRecords(msg.records)
		m.rooms = msg.records
		m.cursor = 0
		if len(msg.records) == 0 {
			m.step = stepJoinManualAddr
			m.status = "No LAN rooms found. Enter server IP:port manually."
			m.prepareInput("Server IP:port", "192.168.0.10:9000", false)
			return m, textinput.Blink
		}
		m.step = stepJoinList
		m.status = "Select a room on this LAN."
		return m, nil
	case connectDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.step = previousConnectStep(m.state)
			m.status = "Connection failed. Fix the values and try again."
			if m.step == stepJoinPasscode || m.step == stepHostPasscode {
				m.prepareInput("Passcode", "", true)
				return m, textinput.Blink
			}
			return m, nil
		}
		m.client = msg.client
		m.advertiser = msg.advertiser
		m.events = msg.events
		m.localInfo = msg.localInfo
		m.mode = viewChat
		m.err = ""
		m.status = "Connected."
		m.logs = []string{fmt.Sprintf("Connected to %s.", m.state.Room)}
		m.ensurePerson(m.state.Nick)
		m.prepareInput("Message", "Type a message", false)
		return m, tea.Batch(textinput.Blink, waitEventCmd(m.events))
	case tea.KeyMsg:
		switch m.step {
		case stepMode:
			return m.updateModeStep(msg)
		case stepNick, stepJoinManualAddr, stepJoinManualRoom, stepJoinPasscode, stepHostRoom, stepHostPasscode:
			return m.updateTextStep(msg)
		case stepJoinLoading, stepConnecting:
			return m, nil
		case stepJoinList:
			return m.updateJoinList(msg)
		case stepJoinManualProtection, stepHostProtection:
			return m.updateProtectionStep(msg)
		}
	}

	var cmd tea.Cmd
	if usesInput(m.step) {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

func (m tuiModel) updateModeStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = 0
	case "down", "j":
		m.cursor = 1
	case "enter":
		if m.cursor == 0 {
			m.state.Mode = ModeHost
			m.step = stepNick
			m.status = "Enter your nickname."
			m.prepareInput("Nickname", "[name]", false)
		} else {
			m.state.Mode = ModeJoin
			m.step = stepNick
			m.status = "Enter your nickname."
			m.prepareInput("Nickname", "[name]", false)
		}
		return m, textinput.Blink
	}
	return m, nil
}

func (m tuiModel) updateTextStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.back()
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if msg.String() != "enter" {
		return m, cmd
	}

	value := strings.TrimSpace(m.input.Value())
	switch m.step {
	case stepNick:
		if value == "" {
			m.err = "Nickname is required."
			return m, nil
		}
		m.state.Nick = value
		m.err = ""
		if m.state.Mode == ModeHost {
			m.step = stepHostRoom
			m.status = "Set the public room name."
			m.prepareInput("Public room name", "lobby", false)
			return m, textinput.Blink
		}
		m.step = stepJoinLoading
		m.status = "Scanning the LAN for rooms..."
		return m, browseCmd(m.appCtx)
	case stepHostRoom:
		if value == "" {
			m.err = "Room name is required."
			return m, nil
		}
		m.state.Room = value
		m.err = ""
		m.step = stepHostProtection
		m.cursor = 0
		m.status = "Choose room protection."
		return m, nil
	case stepHostPasscode:
		if value == "" {
			m.err = "Passcode is required for a protected room."
			return m, nil
		}
		m.state.Passcode = value
		m.err = ""
		m.step = stepConnecting
		m.status = "Starting host..."
		return m, connectCmd(m.appCtx, m.state)
	case stepJoinManualAddr:
		if value == "" {
			m.err = "Server IP:port is required."
			return m, nil
		}
		m.state.ManualAddr = value
		m.err = ""
		m.step = stepJoinManualRoom
		m.status = "Enter the room name."
		m.prepareInput("Room name", "lobby", false)
		return m, textinput.Blink
	case stepJoinManualRoom:
		if value == "" {
			m.err = "Room name is required."
			return m, nil
		}
		m.state.Room = value
		m.err = ""
		m.step = stepJoinManualProtection
		m.cursor = 0
		m.status = "Choose whether the room is protected."
		return m, nil
	case stepJoinPasscode:
		if value == "" && m.state.Protected {
			m.err = "Passcode is required for a protected room."
			return m, nil
		}
		m.state.Passcode = value
		m.err = ""
		m.step = stepConnecting
		m.status = "Connecting..."
		return m, connectCmd(m.appCtx, m.state)
	}

	return m, nil
}

func (m tuiModel) updateJoinList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	max := len(m.rooms)
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < max {
			m.cursor++
		}
	case "esc":
		m.step = stepNick
		m.status = "Enter your nickname."
		m.prepareInput("Nickname", m.state.Nick, false)
		return m, textinput.Blink
	case "enter":
		m.err = ""
		if m.cursor == max {
			m.state.SelectedPeer = nil
			m.state.ManualAddr = ""
			m.step = stepJoinManualAddr
			m.status = "Enter server IP:port."
			m.prepareInput("Server IP:port", "192.168.0.10:9000", false)
			return m, textinput.Blink
		}
		selected := m.rooms[m.cursor]
		m.state.SelectedPeer = &selected
		m.state.Room = selected.Room
		m.state.Protected = selected.Protected
		m.state.ManualAddr = ""
		if m.state.Protected {
			m.step = stepJoinPasscode
			m.status = "Enter the room passcode."
			m.prepareInput("Passcode", "", true)
			return m, textinput.Blink
		}
		m.state.Passcode = ""
		m.step = stepConnecting
		m.status = "Connecting..."
		return m, connectCmd(m.appCtx, m.state)
	}
	return m, nil
}

func (m tuiModel) updateProtectionStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = 0
	case "down", "j":
		m.cursor = 1
	case "esc":
		m.back()
		return m, textinput.Blink
	case "enter":
		protected := m.cursor == 1
		m.state.Protected = protected
		m.err = ""
		if m.step == stepHostProtection {
			if protected {
				m.step = stepHostPasscode
				m.status = "Enter the room passcode."
				m.prepareInput("Passcode", "", true)
				return m, textinput.Blink
			}
			m.state.Passcode = ""
			m.step = stepConnecting
			m.status = "Starting host..."
			return m, connectCmd(m.appCtx, m.state)
		}
		if protected {
			m.step = stepJoinPasscode
			m.status = "Enter the room passcode."
			m.prepareInput("Passcode", "", true)
			return m, textinput.Blink
		}
		m.state.Passcode = ""
		m.step = stepConnecting
		m.status = "Connecting..."
		return m, connectCmd(m.appCtx, m.state)
	}
	return m, nil
}

func (m tuiModel) updateChat(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatEventMsg:
		m.applyEvent(msg.event)
		return m, waitEventCmd(m.events)
	case chatClosedMsg:
		m.status = "Connection closed."
		return m, tea.Quit
	case sendResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.err = ""
			m.logs = append(m.logs, fmt.Sprintf("[%s] %s: %s", msg.sentAt.Local().Format("15:04"), m.state.Nick, msg.body))
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.cancel()
			return m, tea.Quit
		case "enter":
			body := strings.TrimSpace(m.input.Value())
			if body == "" {
				return m, nil
			}
			if body == "/quit" {
				m.cancel()
				return m, tea.Quit
			}
			m.input.SetValue("")
			return m, tea.Batch(textinput.Blink, sendMsgCmd(m.client, body))
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	if m.mode == viewChat {
		return m.chatView()
	}
	return m.setupView()
}

func (m tuiModel) setupView() string {
	frameWidth := contentWidth(m.width, 42)
	leftWidth := clamp(frameWidth/3, 20, 28)
	rightWidth := max(18, frameWidth-leftWidth-2)

	left := panelStyle.Width(leftWidth).Render(strings.Join([]string{
		titleStyle.Render("termchat"),
		subtleStyle.Render("Single-command LAN chat"),
		"",
		m.stepSummary(),
		"",
		subtleStyle.Render("Controls"),
		"j/k or arrows: move",
		"Enter: confirm",
		"Esc: back",
		"Ctrl+C: quit",
	}, "\n"))

	right := panelStyle.Width(rightWidth).Render(strings.Join([]string{
		titleStyle.Render(m.stepTitle()),
		subtleStyle.Render(m.status),
		"",
		m.stepContent(),
		errorBlock(m.err),
	}, "\n"))

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	return appBorder.Width(frameWidth).Render(body)
}

func (m tuiModel) chatView() string {
	frameWidth := contentWidth(m.width, 54)
	header := panelStyle.Width(frameWidth).Render(strings.Join([]string{
		titleStyle.Render("Chat"),
		subtleStyle.Render(fmt.Sprintf("room=%s   local=%s   nick=%s", m.state.Room, m.localInfo, m.state.Nick)),
		subtleStyle.Render("Enter: send   Esc or /quit: leave"),
		errorBlock(m.err),
	}, "\n"))

	logHeight := clamp(m.height-14, 6, 18)
	if frameWidth < 72 {
		logPanel := panelStyle.Width(frameWidth).Height(logHeight).Render(strings.Join(m.tailLogs(logHeight-2), "\n"))
		peoplePanel := panelStyle.Width(frameWidth).Render(strings.Join(append([]string{activeStyle.Render("Participants")}, m.people...), "\n"))
		inputPanel := panelStyle.Width(frameWidth).Render(fmt.Sprintf("%s\n%s", activeStyle.Render("Message"), m.input.View()))
		body := lipgloss.JoinVertical(lipgloss.Left, logPanel, peoplePanel, inputPanel)
		return appBorder.Width(frameWidth).Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
	}

	leftWidth := frameWidth - 20
	rightWidth := 18
	logPanel := panelStyle.Width(leftWidth).Height(logHeight).Render(strings.Join(m.tailLogs(logHeight-2), "\n"))
	inputPanel := panelStyle.Width(leftWidth).Render(fmt.Sprintf("%s\n%s", activeStyle.Render("Message"), m.input.View()))
	peoplePanel := panelStyle.Width(rightWidth).Height(logHeight + 3).Render(strings.Join(append([]string{activeStyle.Render("Participants")}, m.people...), "\n"))
	body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.JoinVertical(lipgloss.Left, logPanel, inputPanel), " ", peoplePanel)
	return appBorder.Width(frameWidth).Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m tuiModel) stepSummary() string {
	mode := "-"
	if m.state.Mode != "" {
		mode = string(m.state.Mode)
	}
	room := "-"
	if m.state.Room != "" {
		room = m.state.Room
	}
	security := "open"
	if m.state.Protected {
		security = "protected"
	}
	addr := "-"
	if target := m.state.TargetAddr(); target != "" {
		addr = target
	}
	return strings.Join([]string{
		activeStyle.Render("Current"),
		fmt.Sprintf("mode: %s", mode),
		fmt.Sprintf("nick: %s", fallback(m.state.Nick, "-")),
		fmt.Sprintf("room: %s", room),
		fmt.Sprintf("security: %s", security),
		fmt.Sprintf("target: %s", addr),
	}, "\n")
}

func (m tuiModel) stepTitle() string {
	switch m.step {
	case stepMode:
		return "Choose Mode"
	case stepNick:
		return "Nickname"
	case stepJoinLoading:
		return "Scanning LAN"
	case stepJoinList:
		return "Join a Room"
	case stepJoinManualAddr:
		return "Manual Address"
	case stepJoinManualRoom:
		return "Manual Room Name"
	case stepJoinManualProtection:
		return "Room Protection"
	case stepJoinPasscode:
		return "Room Passcode"
	case stepHostRoom:
		return "Public Room Name"
	case stepHostProtection:
		return "Room Protection"
	case stepHostPasscode:
		return "Host Passcode"
	case stepConnecting:
		return "Connecting"
	default:
		return "Setup"
	}
}

func (m tuiModel) stepContent() string {
	switch m.step {
	case stepMode:
		return m.selectBlock([]string{"Host a room", "Join a room"}, m.cursor)
	case stepNick, stepJoinManualAddr, stepJoinManualRoom, stepJoinPasscode, stepHostRoom, stepHostPasscode:
		return m.input.View()
	case stepJoinLoading:
		return "Scanning for `_termchat._tcp` rooms on this LAN..."
	case stepJoinList:
		items := make([]string, 0, len(m.rooms)+1)
		for _, room := range m.rooms {
			kind := "open"
			if room.Protected {
				kind = "protected"
			}
			items = append(items, fmt.Sprintf("%s [%s]  %s:%d", room.Room, kind, room.Addr, room.Port))
		}
		items = append(items, "Manual IP:port")
		return m.selectBlock(items, m.cursor)
	case stepJoinManualProtection, stepHostProtection:
		return m.selectBlock([]string{"Open room", "Password-protected room"}, m.cursor)
	case stepConnecting:
		return "Establishing connection..."
	default:
		return ""
	}
}

func (m tuiModel) selectBlock(items []string, cursor int) string {
	lines := make([]string, 0, len(items))
	for i, item := range items {
		prefix := "  "
		if i == cursor {
			prefix = activeStyle.Render("> ")
			lines = append(lines, prefix+activeStyle.Render(item))
			continue
		}
		lines = append(lines, prefix+item)
	}
	return strings.Join(lines, "\n")
}

func (m *tuiModel) prepareInput(prompt, placeholder string, password bool) {
	m.input.Prompt = prompt + ": "
	m.input.Placeholder = placeholder
	m.input.SetValue("")
	m.input.Focus()
	m.input.Width = clamp(m.width-20, 16, 56)
	if password {
		m.input.EchoMode = textinput.EchoPassword
		m.input.EchoCharacter = '•'
	} else {
		m.input.EchoMode = textinput.EchoNormal
	}
}

func (m *tuiModel) applyEvent(event protocol.Event) {
	switch event.Kind {
	case protocol.EventMessage:
		if event.Sender == m.state.Nick {
			return
		}
		m.logs = append(m.logs, fmt.Sprintf("[%s] %s: %s", event.SentAt.Local().Format("15:04"), event.Sender, event.Body))
		m.ensurePerson(event.Sender)
	case protocol.EventParticipantJoined:
		m.logs = append(m.logs, fmt.Sprintf("* %s joined", event.Sender))
		m.ensurePerson(event.Sender)
	case protocol.EventParticipantLeft:
		m.logs = append(m.logs, fmt.Sprintf("* %s left", event.Sender))
		delete(m.peopleSet, event.Sender)
		m.rebuildPeople()
	case protocol.EventError:
		m.logs = append(m.logs, fmt.Sprintf("! %s", strings.Trim(event.Body, `"`)))
	}
}

func (m *tuiModel) ensurePerson(name string) {
	if name == "" {
		return
	}
	if _, ok := m.peopleSet[name]; ok {
		return
	}
	m.peopleSet[name] = struct{}{}
	m.rebuildPeople()
}

func (m *tuiModel) rebuildPeople() {
	m.people = m.people[:0]
	for name := range m.peopleSet {
		m.people = append(m.people, name)
	}
	if len(m.people) == 0 {
		m.people = []string{"-"}
	}
}

func (m tuiModel) tailLogs(height int) []string {
	if height < 1 {
		height = 1
	}
	if len(m.logs) == 0 {
		return []string{subtleStyle.Render("No messages yet.")}
	}
	if len(m.logs) <= height {
		return m.logs
	}
	return m.logs[len(m.logs)-height:]
}

func (m *tuiModel) back() {
	m.err = ""
	switch m.step {
	case stepNick:
		m.step = stepMode
		m.status = "Choose host or join."
	case stepHostRoom:
		m.step = stepNick
		m.status = "Enter your nickname."
		m.prepareInput("Nickname", m.state.Nick, false)
	case stepHostProtection:
		m.step = stepHostRoom
		m.status = "Set the public room name."
		m.prepareInput("Public room name", m.state.Room, false)
	case stepHostPasscode:
		m.step = stepHostProtection
		m.status = "Choose room protection."
		m.cursor = boolCursor(m.state.Protected)
	case stepJoinManualAddr:
		if len(m.rooms) > 0 {
			m.step = stepJoinList
			m.status = "Select a room on this LAN."
		} else {
			m.step = stepNick
			m.status = "Enter your nickname."
			m.prepareInput("Nickname", m.state.Nick, false)
		}
	case stepJoinManualRoom:
		m.step = stepJoinManualAddr
		m.status = "Enter server IP:port."
		m.prepareInput("Server IP:port", m.state.ManualAddr, false)
	case stepJoinManualProtection:
		m.step = stepJoinManualRoom
		m.status = "Enter the room name."
		m.prepareInput("Room name", m.state.Room, false)
	case stepJoinPasscode:
		if m.state.SelectedPeer != nil {
			m.step = stepJoinList
			m.status = "Select a room on this LAN."
		} else {
			m.step = stepJoinManualProtection
			m.status = "Choose whether the room is protected."
			m.cursor = boolCursor(m.state.Protected)
		}
	}
}

func browseCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		records, err := discovery.Browse(ctx, 2*time.Second)
		return browseDoneMsg{records: records, err: err}
	}
}

func connectCmd(ctx context.Context, state SetupState) tea.Cmd {
	return func() tea.Msg {
		addr := state.TargetAddr()
		localInfo := addr
		var advertiser *discovery.Advertiser
		if state.Mode == ModeHost {
			room := hostRoomConfig(state)
			_, port, err := protocol.StartHost(ctx, []protocol.RoomConfig{room})
			if err != nil {
				return connectDoneMsg{err: err}
			}
			advertiser, err = discovery.Advertise(room, port)
			if err != nil {
				return connectDoneMsg{err: err}
			}
			localIP := localIPv4()
			localInfo = net.JoinHostPort(localIP, fmt.Sprintf("%d", port))
			addr = net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
		}
		client, err := protocol.Dial(ctx, addr, state.Room, state.Nick, state.Passcode)
		if err != nil {
			if advertiser != nil {
				advertiser.Close()
			}
			return connectDoneMsg{err: err}
		}
		return connectDoneMsg{
			client:     client,
			advertiser: advertiser,
			events:     client.Receive(ctx),
			localInfo:  localInfo,
		}
	}
}

func waitEventCmd(events <-chan protocol.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return chatClosedMsg{}
		}
		return chatEventMsg{event: event}
	}
}

func sendMsgCmd(client *protocol.Client, body string) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		return sendResultMsg{body: body, sentAt: now, err: client.Send(body)}
	}
}

func usesInput(step setupStep) bool {
	switch step {
	case stepNick, stepJoinManualAddr, stepJoinManualRoom, stepJoinPasscode, stepHostRoom, stepHostPasscode:
		return true
	default:
		return false
	}
}

func previousConnectStep(state SetupState) setupStep {
	if state.Mode == ModeHost {
		if state.Protected {
			return stepHostPasscode
		}
		return stepHostProtection
	}
	if state.Protected {
		return stepJoinPasscode
	}
	if state.SelectedPeer != nil {
		return stepJoinList
	}
	return stepJoinManualProtection
}

func boolCursor(v bool) int {
	if v {
		return 1
	}
	return 0
}

func errorBlock(err string) string {
	if err == "" {
		return ""
	}
	return "\n" + errorStyle.Render(err)
}

func fallback(v, alt string) string {
	if strings.TrimSpace(v) == "" {
		return alt
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func contentWidth(termWidth, fallback int) int {
	if termWidth <= 0 {
		return fallback
	}
	return max(20, termWidth-6)
}
