package main

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) == 1 {
		slog.Error("no arguments provided")

		os.Exit(1)
	}

	ctx := waitGracefulShutdown()

	group, _ := errgroup.WithContext(ctx)
	group.SetLimit(runtime.NumCPU())

	keywords := []string{"return", "continue", "break"}

	var files []string
	if len(os.Args) == 2 {
		files = os.Args[1:]
	}

	if files[0] == "./..." {
		pwd, err := os.Getwd()
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}

		files, err = ListGoFiles(pwd)
		if err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}

	for _, fileName := range files {
	tryAgain:
		select {
		case <-ctx.Done():

			return
		default:
			if ok := group.TryGo(func() error {
				replacementCount := 0
				defer func() {
					if replacementCount > 0 {
						slog.Info("replaced", slog.Int("count", replacementCount), slog.String("in", fileName))
					}
				}()

				tmp, err := os.ReadFile(fileName)
				if err != nil {
					return err
				}

				file := strings.Split(string(tmp), "\n")

			AhShitAndHereWeGoAgain:
				for lineNum, line := range file {
					if strings.Contains(line, "//") {
						continue
					}

					for _, keyword := range keywords {
						if strings.Contains(line, keyword) && strings.TrimSpace(file[lineNum-1]) != "" {
							if len(file[lineNum+1]) == 0 || len(file[lineNum-1]) == 0 ||
								strings.Contains(file[lineNum+1], "//") {
								continue
							}

							prevSymb := file[lineNum-1][len(file[lineNum-1])-1]
							nextSymb := file[lineNum+1][len(file[lineNum+1])-1]

							if prevSymb == '{' && (nextSymb == '}' || nextSymb == ',') ||
								strings.Contains(file[lineNum-1], "func") ||
								strings.Contains(file[lineNum-1], "case") ||
								strings.Contains(file[lineNum-1], "default") {
								break // skip check on error only
							}

							file = append(file[:lineNum], append([]string{""}, file[lineNum:]...)...)

							replacementCount++
							goto AhShitAndHereWeGoAgain
						}
					}
				}

				if isDemo := os.Getenv("DEMO"); isDemo != "" {
					return nil
				}

				return os.WriteFile(fileName, []byte(strings.Join(file, "\n")), 0644)
			}); !ok {
				time.Sleep(time.Millisecond)
				goto tryAgain
			}
		}

	}

	if err := group.Wait(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func waitGracefulShutdown() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		slog.Info("received %s signal, terminating...", <-sig)

		cancel()
	}()

	return ctx
}

func ListGoFiles(rootDir string) ([]string, error) {
	var goFiles []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	return goFiles, err
}
