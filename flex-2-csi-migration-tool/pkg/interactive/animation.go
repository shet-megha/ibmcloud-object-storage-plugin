package interactive

import (
	"fmt"
	"time"
)

func (a *App) showLoadingAnimation(done chan bool) {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	statusMessages := []string{
		"Scanning namespaces...",
		"Discovering Persistent Volumes...",
		"Finding Flex PVs...",
		"Mapping PVs to PVCs...",
		"Discovering secrets...",
		"Finding workloads (Deployments, StatefulSets, DaemonSets)...",
		"Checking pod status...",
		"Analyzing resource dependencies...",
		"Building resource graph...",
		"Scanning for services...",
	}

	i := 0
	msgIndex := 0
	msgCounter := 0

	for {
		select {
		case <-done:
			return
		default:
			if msgCounter%20 == 0 {
				msgIndex = (msgIndex + 1) % len(statusMessages)
			}
			msgCounter++

			fmt.Printf("\r   %s %s                    ", spinners[i%len(spinners)], statusMessages[msgIndex])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (a *App) showProgressBar(percentage int) {
	barLength := 50
	filled := (percentage * barLength) / 100

	fmt.Print("\n[")
	for i := 0; i < barLength; i++ {
		if i < filled {
			fmt.Print("█")
		} else {
			fmt.Print("░")
		}
	}
	fmt.Printf("] %d%%\n", percentage)
}

func (a *App) pause() {
	fmt.Print("\nPress Enter to continue...")
	a.reader.ReadString('\n')
}

// Made with Bob
