package style

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

var frames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧"}

// Spinner displays an animated spinner with a message on a TTY.
// On non-TTY writers it prints the message once and does nothing else.
type Spinner struct {
	w     io.Writer
	msg   string
	done  chan struct{}
	wg    sync.WaitGroup
	isTTY bool
}

// StartSpinner begins displaying an animated spinner with the given message.
// Call the returned Stop function when the operation completes.
func StartSpinner(w io.Writer, msg string) *Spinner {
	s := &Spinner{
		w:    w,
		msg:  msg,
		done: make(chan struct{}),
	}

	// Check if writer is a TTY (only animate if so).
	if f, ok := w.(*os.File); ok {
		s.isTTY = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}

	if !s.isTTY {
		fmt.Fprintf(w, "%s\n", msg)
		return s
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		i := 0
		for {
			select {
			case <-s.done:
				// Clear the spinner line.
				fmt.Fprintf(s.w, "\r\033[K")
				return
			default:
				frame := Dim.Render(frames[i%len(frames)])
				fmt.Fprintf(s.w, "\r%s %s", frame, s.msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return s
}

// Stop stops the spinner animation and clears the line.
func (s *Spinner) Stop() {
	if !s.isTTY {
		return
	}
	close(s.done)
	s.wg.Wait()
}
