package tui

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Spinner is a goroutine-based terminal spinner that rewrites its line in
// place. Call Stop() to clear it; Success()/Fail() to clear and print a
// final status line.
type Spinner struct {
	msg   atomic.Value // string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner begins rendering a spinner with the given initial message.
func StartSpinner(initialMsg string) *Spinner {
	s := &Spinner{
		start: time.Now(),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	s.msg.Store(initialMsg)
	s.render(0)
	go s.loop()
	return s
}

func (s *Spinner) render(frameIdx int) {
	msg, _ := s.msg.Load().(string)
	elapsed := time.Since(s.start).Round(time.Second)
	fmt.Printf("\r\033[K%s %s %s",
		Cyan(spinnerFrames[frameIdx%len(spinnerFrames)]),
		msg,
		Dim(fmt.Sprintf("(%s)", elapsed)),
	)
}

// Update swaps the message shown next to the animated frame.
func (s *Spinner) Update(msg string) {
	s.msg.Store(msg)
}

func (s *Spinner) loop() {
	defer close(s.done)
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	i := 1
	for {
		select {
		case <-s.stop:
			fmt.Print("\r\033[K")
			return
		case <-t.C:
			s.render(i)
			i++
		}
	}
}

// Stop clears the spinner line without printing anything else.
func (s *Spinner) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	<-s.done
}

// Success stops the spinner and prints a Success line in its place.
func (s *Spinner) Success(msg string) {
	s.Stop()
	Success(msg)
}

// Fail stops the spinner and prints an Err line in its place.
func (s *Spinner) Fail(msg string) {
	s.Stop()
	Err(msg)
}
