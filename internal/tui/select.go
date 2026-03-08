package tui

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/term"
)

// Select displays an interactive picklist and returns the selected index.
// Returns -1 if the user cancels (Ctrl+C / q / Esc).
func Select(prompt string, items []string) int {
	if len(items) == 0 {
		return -1
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return -1
	}
	defer term.Restore(fd, oldState)

	cursor := 0
	render(prompt, items, cursor)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return -1
		}

		switch {
		case n == 1 && (buf[0] == 3 || buf[0] == 'q' || buf[0] == 27): // Ctrl+C, q, Esc
			clearList(len(items) + 2)
			return -1
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'): // Enter
			clearList(len(items) + 2)
			return cursor
		case n == 1 && (buf[0] == 'k' || buf[0] == 'K'): // vim up
			if cursor > 0 {
				cursor--
			}
		case n == 1 && (buf[0] == 'j' || buf[0] == 'J'): // vim down
			if cursor < len(items)-1 {
				cursor++
			}
		case n == 3 && buf[0] == 27 && buf[1] == '[': // arrow keys
			switch buf[2] {
			case 'A': // up
				if cursor > 0 {
					cursor--
				}
			case 'B': // down
				if cursor < len(items)-1 {
					cursor++
				}
			}
		}

		clearList(len(items) + 2)
		render(prompt, items, cursor)
	}
}

func render(prompt string, items []string, cursor int) {
	fmt.Printf("%s\r\n", prompt)
	for i, item := range items {
		if i == cursor {
			fmt.Printf("  \033[36m❯ %s\033[0m\r\n", item)
		} else {
			fmt.Printf("    %s\r\n", item)
		}
	}
	fmt.Print("\033[2m  ↑/↓ to move, Enter to select, Esc to cancel\033[0m")
}

func clearList(lines int) {
	for i := 0; i < lines; i++ {
		fmt.Print("\033[2K") // clear line
		if i < lines-1 {
			fmt.Print("\033[A") // move up
		}
	}
	fmt.Print("\r")
}

// InputInt prompts the user to type a number. Shows a default value that is
// accepted on bare Enter. Returns the default if the user presses Enter without
// typing, or -1 on cancel (Ctrl+C / Esc).
func InputInt(prompt string, defaultVal int) int {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return defaultVal
	}
	defer term.Restore(fd, oldState)

	input := ""
	renderInput(prompt, input, defaultVal)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return -1
		}

		if n == 1 {
			switch {
			case buf[0] == 3 || buf[0] == 27: // Ctrl+C, Esc
				fmt.Print("\r\033[2K")
				return -1
			case buf[0] == '\r' || buf[0] == '\n': // Enter
				fmt.Print("\r\033[2K")
				if input == "" {
					return defaultVal
				}
				v, err := strconv.Atoi(input)
				if err != nil || v < 1 {
					return defaultVal
				}
				return v
			case buf[0] == 127 || buf[0] == 8: // Backspace
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			case buf[0] >= '0' && buf[0] <= '9':
				input += string(buf[0])
			}
		}

		fmt.Print("\r\033[2K")
		renderInput(prompt, input, defaultVal)
	}
}

func renderInput(prompt, input string, defaultVal int) {
	if input == "" {
		fmt.Printf("%s \033[2m(%d)\033[0m ", prompt, defaultVal)
	} else {
		fmt.Printf("%s %s", prompt, input)
	}
}
