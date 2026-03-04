package main

import (
	"context"
	"flag"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"
)

func main() {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := run(ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func getTerminalDimentions() (terminalDimentions, error) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return terminalDimentions{}, fmt.Errorf("unable to get terminal dimentions: %w", err)
	}

	return terminalDimentions{
		width:  width,
		height: height,
	}, nil
}

func getOutputFilename(inputFilename string) string {
	dir, filename := filepath.Split(inputFilename)
	return filepath.Join(dir, strings.TrimSuffix(filename, filepath.Ext(filename))+"_out.png")
}

func run(ctx context.Context) error {
	flag.Parse()

	inputFilename := flag.Arg(0)
	inputFilename, err := filepath.Abs(inputFilename)
	if err != nil {
		return fmt.Errorf("unable to get abs input filepath: %w", err)
	}

	src, err := readImage(inputFilename)
	if err != nil {
		return fmt.Errorf("unable to read input image: %w", err)
	}

	terminalDims, err := getTerminalDimentions()
	if err != nil {
		return fmt.Errorf("unable to get terminal dimentions: %w", err)
	}

	res, err := processImage(src, terminalDims)
	if err != nil {
		return fmt.Errorf("unable to process image: %w", err)
	}

	sb := strings.Builder{}
	for _, row := range res {
		for _, char := range row {
			sb.WriteString(char.char)
		}
		sb.WriteString("\n")
	}
	fmt.Print(sb.String())

	dimsCh := detectTerminalDimentionsChange(ctx)
	dimsCh = timeBufferedChan(dimsCh, time.Millisecond*300)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case dims, ok := <-dimsCh:
				if !ok {
					return
				}

				res, err := processImage(src, dims)
				if err != nil {
					return
				}

				sb := strings.Builder{}
				sb.WriteString("\033[2J\033[H")

				for _, row := range res {
					for _, char := range row {
						sb.WriteString(char.char)
					}
					sb.WriteString("\n")
				}

				fmt.Print(sb.String())
			}
		}
	}()

	<-ctx.Done()

	return nil
}

type terminalDimentions struct {
	width  int
	height int
}

func detectTerminalDimentionsChange(ctx context.Context) <-chan terminalDimentions {
	res := make(chan terminalDimentions, 1)

	nc := make(chan os.Signal, 1000)
	signal.Notify(nc, syscall.SIGWINCH)

	go func() {
		defer close(res)
		defer close(nc)

		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-nc:
				if !ok {
					return
				}
				if s != syscall.SIGWINCH {
					continue
				}

				dims, err := getTerminalDimentions()
				if err != nil {
					continue
				}

				select {
				case <-ctx.Done():
					return
				case res <- dims:
				default:
					<-res
					res <- dims
				}
			}
		}
	}()

	return res
}

func timeBufferedChan[T any](in <-chan T, buf time.Duration) <-chan T {
	out := make(chan T, cap(in))

	var currentVal T
	m := &sync.Mutex{}
	isWaiting := false

	scheduleNextValue := func() {
		<-time.After(buf)

		m.Lock()
		defer m.Unlock()

		if !isWaiting {
			return
		}

		out <- currentVal
		isWaiting = false
	}

	nextValue := func(val T) {
		m.Lock()
		defer m.Unlock()

		currentVal = val

		if !isWaiting {
			isWaiting = true
			go scheduleNextValue()
		}
	}

	go func() {
		defer close(out)
		for val := range in {
			nextValue(val)
		}
	}()

	return out
}
