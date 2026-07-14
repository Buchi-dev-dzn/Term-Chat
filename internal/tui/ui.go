package tui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type lineUI struct {
	reader *bufio.Reader
	out    io.Writer
}

func newLineUI(in io.Reader, out io.Writer) *lineUI {
	return &lineUI{
		reader: bufio.NewReader(in),
		out:    out,
	}
}

func (ui *lineUI) ask(label string) (string, error) {
	fmt.Fprintf(ui.out, "%s: ", label)
	text, err := ui.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func (ui *lineUI) askChoice(prompt string, valid []string) (string, error) {
	for {
		fmt.Fprint(ui.out, prompt)
		value, err := ui.ask("> ")
		if err != nil {
			return "", err
		}
		for _, item := range valid {
			if value == item {
				return value, nil
			}
		}
		fmt.Fprintln(ui.out, "Invalid selection.")
	}
}
