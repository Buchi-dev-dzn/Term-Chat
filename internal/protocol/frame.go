package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

func WriteFrame(w io.Writer, frame ControlFrame) error {
	if err := json.NewEncoder(w).Encode(frame); err != nil {
		return fmt.Errorf("write frame: %w", err)
	}
	return nil
}

func ReadFrame(r *bufio.Reader) (ControlFrame, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return ControlFrame{}, err
	}
	var frame ControlFrame
	if err := json.Unmarshal(line, &frame); err != nil {
		return ControlFrame{}, err
	}
	return frame, nil
}
