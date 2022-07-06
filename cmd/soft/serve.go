package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/charmbracelet/keygen"
	"github.com/charmbracelet/soft-serve/config"
	"github.com/charmbracelet/soft-serve/server"
	"github.com/spf13/cobra"
)

var (
	hookTmpl  *template.Template
	initHooks bool
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the server",
		Long:  "Start the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.DefaultConfig()
			// Internal API keypair
			_, err := keygen.NewWithWrite(
				strings.TrimSuffix(cfg.InternalKeyPath, "_ed25519"),
				nil,
				keygen.Ed25519,
			)
			if err != nil {
				return err
			}
			// Create git server hooks
			if initHooks {
				ex, err := os.Executable()
				if err != nil {
					return err
				}
				repos, err := os.ReadDir(cfg.RepoPath)
				if err != nil {
					return err
				}
				for _, repo := range repos {
					for _, hook := range []string{"pre-receive", "update", "post-receive"} {
						var data bytes.Buffer
						var args string
						hp := fmt.Sprintf("%s/%s/hooks/%s", cfg.RepoPath, repo.Name(), hook)
						if hook == "update" {
							args = "$1 $2 $3"
						}
						err = hookTmpl.Execute(&data, hookScript{
							Executable: ex,
							Hook:       hook,
							Args:       args,
						})
						if err != nil {
							return err
						}
						err = os.WriteFile(hp, data.Bytes(), 0755) //nolint:gosec
						if err != nil {
							return err
						}
					}
				}
			}
			s := server.NewServer(cfg)

			done := make(chan os.Signal, 1)
			signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

			log.Printf("Starting SSH server on %s:%d", cfg.BindAddr, cfg.Port)
			go func() {
				if err := s.Start(); err != nil {
					log.Fatalln(err)
				}
			}()

			<-done

			log.Printf("Stopping SSH server on %s:%d", cfg.BindAddr, cfg.Port)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer func() { cancel() }()
			return s.Shutdown(ctx)
		},
	}
)

type hookScript struct {
	Executable string
	Hook       string
	Args       string
}

func init() {
	hookTmpl = template.New("hook")
	hookTmpl, _ = hookTmpl.Parse(`#!/usr/bin/env bash
# AUTO GENERATED BY SOFT SERVE, DO NOT MODIFY
{{ .Executable }} internal hook {{ .Hook }} {{ .Args }}
`)
	serveCmd.Flags().BoolVarP(&initHooks, "init-hooks", "i", false, "Initialize git hooks")
}
