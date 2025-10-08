package main

import (
	"fmt"
	"sync"
	"time"
)

// Spinner provides a simple terminal loading animation during long operations
type Spinner struct {
	chars []string
	delay time.Duration
	end   chan struct{}
	wg    sync.WaitGroup
}

func NewSpinner() *Spinner {
	return &Spinner{
		chars: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		delay: 100 * time.Millisecond,
	}
}

func (s *Spinner) Start(message string) {
	s.end = make(chan struct{})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		i := 0
		for {
			select {
			case <-s.end:
				fmt.Printf("\r✅ %s\n", message)
				return
			default:
				fmt.Printf("\r%s %s", s.chars[i%len(s.chars)], message)
				i++
				time.Sleep(s.delay)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.end)
	s.wg.Wait()
}
